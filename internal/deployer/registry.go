package deployer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/internal/api"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
)

// FetchManifestHash fetches the manifests tar.gz from the API and returns its SHA-256 hash.
func FetchManifestHash(ctx context.Context, slug string) (string, error) {
	_, hash, err := fetchManifestsTarGz(ctx, slug)
	return hash, err
}

// DeployChallengeFromRegistry fetches challenge manifests from the API and applies them.
// Returns the content hash of the tar.gz for change detection.
func DeployChallengeFromRegistry(ctx context.Context, clientset *kubernetes.Clientset, dynamicClient dynamic.Interface, slug string) (string, error) {
	logger.Info("Fetching manifests for '%s'...", slug)

	data, hash, err := fetchManifestsTarGz(ctx, slug)
	if err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp("", "kubeasy-dev-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractTarGz(data, tmpDir); err != nil {
		return "", fmt.Errorf("failed to extract manifests: %w", err)
	}

	groups, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		return "", fmt.Errorf("failed to discover API resources: %w", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groups)

	if err := applyManifestDirs(ctx, tmpDir, slug, mapper, dynamicClient); err != nil {
		return "", err
	}

	logger.Info("Waiting for challenge resources to be ready...")
	if err := WaitForChallengeReady(ctx, clientset, slug); err != nil {
		return "", fmt.Errorf("challenge resources failed to become ready: %w", err)
	}

	return hash, nil
}

func fetchManifestsTarGz(ctx context.Context, slug string) ([]byte, string, error) {
	client, err := api.NewPublicClient()
	if err != nil {
		return nil, "", err
	}

	resp, err := client.GetChallengeManifestsWithResponse(ctx, slug)
	if err != nil {
		return nil, "", fmt.Errorf("failed to reach API: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, "", fmt.Errorf("API returned HTTP %d for challenge %q", resp.StatusCode(), slug)
	}

	data := resp.Body
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:]), nil
}

func extractTarGz(data []byte, destDir string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}

		target := filepath.Join(destDir, filepath.Clean("/"+header.Name))
		if !strings.HasPrefix(target, cleanDest) {
			return fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		if _, err := io.Copy(f, tr); err != nil { //nolint:gosec
			f.Close()
			return fmt.Errorf("failed to write file: %w", err)
		}
		f.Close()
	}

	return nil
}
