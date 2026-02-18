package deployer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// DeployLocalChallenge applies manifests from a local challenge directory to the cluster.
// Unlike DeployChallenge, it reads from the local filesystem instead of pulling from OCI.
func DeployLocalChallenge(ctx context.Context, clientset *kubernetes.Clientset, dynamicClient dynamic.Interface, challengeDir string, namespace string) error {
	logger.Info("Deploying local challenge from '%s'...", challengeDir)

	// Find and apply YAML files from manifests/ and policies/
	dirs := []string{"manifests", "policies"}
	for _, dir := range dirs {
		dirPath := filepath.Join(challengeDir, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			logger.Debug("Directory '%s' not found in challenge directory, skipping", dir)
			continue
		}

		var files []string
		err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".yaml") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to walk %s: %w", dir, err)
		}

		for _, f := range files {
			logger.Debug("Applying manifest: %s", f)
			data, err := os.ReadFile(f)
			if err != nil {
				return fmt.Errorf("failed to read manifest %s: %w", f, err)
			}
			if err := kube.ApplyManifest(ctx, data, namespace, clientset, dynamicClient); err != nil {
				return fmt.Errorf("failed to apply manifest %s: %w", filepath.Base(f), err)
			}
		}
	}

	// Wait for Deployments and StatefulSets to be ready
	logger.Info("Waiting for challenge resources to be ready...")
	if err := WaitForChallengeReady(ctx, clientset, namespace); err != nil {
		return fmt.Errorf("challenge resources failed to become ready: %w", err)
	}

	logger.Info("Local challenge deployed successfully.")
	return nil
}
