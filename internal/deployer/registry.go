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
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
)

// FetchManifestHash fetches the manifests tar.gz and returns its SHA-256 hash.
// Used for change detection in watch mode without re-applying.
func FetchManifestHash(ctx context.Context, registryURL, slug string) (string, error) {
	_, hash, err := fetchManifestsTarGz(ctx, registryURL, slug)
	return hash, err
}

// DeployChallengeFromRegistry fetches challenge manifests from the registry and applies them.
// Returns the content hash of the tar.gz for change detection in watch mode.
func DeployChallengeFromRegistry(ctx context.Context, clientset *kubernetes.Clientset, dynamicClient dynamic.Interface, registryURL, slug string) (string, error) {
	logger.Info("Fetching manifests for '%s' from %s...", slug, registryURL)

	data, hash, err := fetchManifestsTarGz(ctx, registryURL, slug)
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

func fetchManifestsTarGz(ctx context.Context, registryURL, slug string) ([]byte, string, error) {
	url := fmt.Sprintf("%s/challenges/%s/manifests", registryURL, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to reach registry at %s: %w", registryURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("registry returned HTTP %d for challenge %q", resp.StatusCode, slug)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

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
