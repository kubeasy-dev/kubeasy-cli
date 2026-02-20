package deployer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/fs"
)

// BuildAndLoadImage builds a Docker image from the given directory and loads it
// into the Kind cluster using the Kind library's nodeutils for image loading.
func BuildAndLoadImage(ctx context.Context, imageDir string, imageTag string, clusterName string) error {
	// Verify Dockerfile exists
	dockerfile := filepath.Join(imageDir, "Dockerfile")
	if _, err := os.Stat(dockerfile); err != nil {
		return fmt.Errorf("dockerfile not found in %s", imageDir)
	}

	// 1. Build Docker image
	logger.Info("Building Docker image '%s' from %s...", imageTag, imageDir)
	buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", imageTag, imageDir)
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		logger.Debug("Docker build output: %s", string(buildOutput))
		return fmt.Errorf("docker build failed: %w", err)
	}
	logger.Info("Docker image '%s' built successfully", imageTag)

	// 2. Get Kind nodes using the Kind library
	provider := cluster.NewProvider()
	nodeList, err := provider.ListInternalNodes(clusterName)
	if err != nil {
		return fmt.Errorf("failed to list Kind nodes: %w", err)
	}
	if len(nodeList) == 0 {
		return fmt.Errorf("no Kind nodes found for cluster '%s'", clusterName)
	}

	// 3. Save Docker image to a temporary tar file
	dir, err := fs.TempDir("", "kubeasy-image-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(dir)

	imageTarPath := filepath.Join(dir, "image.tar")
	logger.Info("Saving Docker image to %s...", imageTarPath)
	saveCmd := exec.CommandContext(ctx, "docker", "save", "-o", imageTarPath, imageTag)
	if err := saveCmd.Run(); err != nil {
		return fmt.Errorf("docker save failed: %w", err)
	}

	// 4. Load image into each Kind node
	for _, node := range nodeList {
		logger.Info("Loading image '%s' into Kind node '%s'...", imageTag, node.String())
		f, err := os.Open(imageTarPath)
		if err != nil {
			return fmt.Errorf("failed to open image tar: %w", err)
		}
		loadErr := nodeutils.LoadImageArchive(node, f)
		f.Close()
		if loadErr != nil {
			return fmt.Errorf("failed to load image into node %s: %w", node.String(), loadErr)
		}
	}

	logger.Info("Image '%s' loaded into Kind cluster '%s'", imageTag, clusterName)
	return nil
}

// HasImageDir checks if a challenge directory contains an image/ directory with a Dockerfile.
func HasImageDir(challengeDir string) bool {
	dockerfile := filepath.Join(challengeDir, "image", "Dockerfile")
	_, err := os.Stat(dockerfile)
	return err == nil
}
