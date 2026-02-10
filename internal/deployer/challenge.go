package deployer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"

	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// DeployChallenge pulls the challenge OCI artifact and applies manifests to the cluster.
func DeployChallenge(ctx context.Context, clientset *kubernetes.Clientset, dynamicClient dynamic.Interface, slug string) error {
	logger.Info("Deploying challenge '%s' from OCI registry...", slug)

	// Create temporary directory for extracted artifacts
	tmpDir, err := os.MkdirTemp("", "kubeasy-challenge-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Pull OCI artifact
	ref := fmt.Sprintf("%s/%s:latest", ChallengesOCIRegistry, slug)
	logger.Debug("Pulling OCI artifact: %s", ref)

	if err := pullOCIArtifact(ctx, ref, tmpDir); err != nil {
		return fmt.Errorf("failed to pull challenge artifact from %s: %w", ref, err)
	}

	// Find and apply YAML files from manifests/ and policies/
	dirs := []string{"manifests", "policies"}
	for _, dir := range dirs {
		dirPath := filepath.Join(tmpDir, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			logger.Debug("Directory '%s' not found in challenge artifact, skipping", dir)
			continue
		}

		files, err := filepath.Glob(filepath.Join(dirPath, "**", "*.yaml"))
		if err != nil {
			return fmt.Errorf("failed to glob %s/*.yaml: %w", dir, err)
		}
		// Also match files directly in the directory (Glob ** doesn't match current dir)
		directFiles, err := filepath.Glob(filepath.Join(dirPath, "*.yaml"))
		if err != nil {
			return fmt.Errorf("failed to glob %s/*.yaml: %w", dir, err)
		}
		files = appendUnique(files, directFiles)

		for _, f := range files {
			logger.Debug("Applying manifest: %s", f)
			data, err := os.ReadFile(f)
			if err != nil {
				return fmt.Errorf("failed to read manifest %s: %w", f, err)
			}
			if err := kube.ApplyManifest(ctx, data, slug, clientset, dynamicClient); err != nil {
				return fmt.Errorf("failed to apply manifest %s: %w", filepath.Base(f), err)
			}
		}
	}

	// Wait for Deployments and StatefulSets to be ready
	logger.Info("Waiting for challenge resources to be ready...")
	if err := waitForChallengeReady(ctx, clientset, slug); err != nil {
		return fmt.Errorf("challenge resources failed to become ready: %w", err)
	}

	logger.Info("Challenge '%s' deployed successfully.", slug)
	return nil
}

// pullOCIArtifact pulls an OCI artifact from the registry to the target directory.
func pullOCIArtifact(ctx context.Context, ref string, targetDir string) error {
	repo, err := remote.NewRepository(ref)
	if err != nil {
		return fmt.Errorf("failed to create OCI repository reference: %w", err)
	}

	store, err := file.New(targetDir)
	if err != nil {
		return fmt.Errorf("failed to create file store: %w", err)
	}
	defer store.Close()

	_, err = oras.Copy(ctx, repo, repo.Reference.Reference, store, "", oras.DefaultCopyOptions)
	if err != nil {
		return fmt.Errorf("failed to pull OCI artifact: %w", err)
	}

	return nil
}

// waitForChallengeReady waits for all Deployments and StatefulSets in the namespace to be ready.
func waitForChallengeReady(ctx context.Context, clientset *kubernetes.Clientset, namespace string) error {
	// List Deployments
	deployments, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	if len(deployments.Items) > 0 {
		names := make([]string, 0, len(deployments.Items))
		for _, d := range deployments.Items {
			names = append(names, d.Name)
		}
		if err := kube.WaitForDeploymentsReady(ctx, clientset, namespace, names); err != nil {
			return err
		}
	}

	// List StatefulSets
	statefulsets, err := clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list statefulsets: %w", err)
	}

	if len(statefulsets.Items) > 0 {
		names := make([]string, 0, len(statefulsets.Items))
		for _, s := range statefulsets.Items {
			names = append(names, s.Name)
		}
		if err := kube.WaitForStatefulSetsReady(ctx, clientset, namespace, names); err != nil {
			return err
		}
	}

	return nil
}

// appendUnique appends items from src to dst, skipping duplicates.
func appendUnique(dst, src []string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, s := range dst {
		seen[s] = struct{}{}
	}
	for _, s := range src {
		if _, ok := seen[s]; !ok {
			dst = append(dst, s)
		}
	}
	return dst
}
