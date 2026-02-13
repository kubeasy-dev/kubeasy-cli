package deployer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalkDirFindsNestedYAML(t *testing.T) {
	// Create a temp directory structure mimicking a challenge artifact
	tmpDir := t.TempDir()

	// manifests/deployment.yaml (root level)
	manifestsDir := filepath.Join(tmpDir, "manifests")
	require.NoError(t, os.MkdirAll(manifestsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(manifestsDir, "deployment.yaml"), []byte("kind: Deployment"), 0o600))

	// manifests/subdir/nested.yaml (nested level)
	subDir := filepath.Join(manifestsDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "nested.yaml"), []byte("kind: Service"), 0o600))

	// manifests/deep/nested/config.yaml (deeply nested)
	deepDir := filepath.Join(manifestsDir, "deep", "nested")
	require.NoError(t, os.MkdirAll(deepDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(deepDir, "config.yaml"), []byte("kind: ConfigMap"), 0o600))

	// manifests/not-yaml.txt (should be ignored)
	require.NoError(t, os.WriteFile(filepath.Join(manifestsDir, "not-yaml.txt"), []byte("ignore me"), 0o600))

	// Walk and collect YAML files
	var files []string
	err := filepath.WalkDir(manifestsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".yaml") {
			files = append(files, path)
		}
		return nil
	})
	require.NoError(t, err)

	assert.Len(t, files, 3)

	// Verify all expected files are found
	var basenames []string
	for _, f := range files {
		basenames = append(basenames, filepath.Base(f))
	}
	assert.Contains(t, basenames, "deployment.yaml")
	assert.Contains(t, basenames, "nested.yaml")
	assert.Contains(t, basenames, "config.yaml")
	assert.NotContains(t, basenames, "not-yaml.txt")
}

func TestWalkDirEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	manifestsDir := filepath.Join(tmpDir, "manifests")
	require.NoError(t, os.MkdirAll(manifestsDir, 0o755))

	var files []string
	err := filepath.WalkDir(manifestsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".yaml") {
			files = append(files, path)
		}
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, files)
}
