package deployer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
)

// applyManifestDirs walks the "manifests" and "policies" subdirectories of baseDir
// and applies every .yaml/.yml file to the cluster namespace.
func applyManifestDirs(
	ctx context.Context,
	baseDir string,
	namespace string,
	mapper meta.RESTMapper,
	dynamicClient dynamic.Interface,
) error {
	dirs := []string{"manifests", "policies"}
	for _, dir := range dirs {
		dirPath := filepath.Join(baseDir, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			logger.Debug("Directory '%s' not found, skipping", dirPath)
			continue
		}

		var files []string
		if err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
				files = append(files, path)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("failed to walk %s: %w", dir, err)
		}

		for _, f := range files {
			logger.Debug("Applying manifest: %s", f)
			data, err := os.ReadFile(f)
			if err != nil {
				return fmt.Errorf("failed to read manifest %s: %w", f, err)
			}
			if err := kube.ApplyManifest(ctx, data, namespace, mapper, dynamicClient); err != nil {
				return fmt.Errorf("failed to apply manifest %s: %w", filepath.Base(f), err)
			}
		}
	}
	return nil
}
