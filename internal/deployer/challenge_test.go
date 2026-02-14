package deployer

import (
	"fmt"
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

func TestChallengesOCIReference(t *testing.T) {
	slugs := []string{"pod-evicted", "first-deployment", "env-config"}

	for _, slug := range slugs {
		ref := fmt.Sprintf("%s/%s:latest", ChallengesOCIRegistry, slug)
		assert.Contains(t, ref, "ghcr.io/", "OCI reference should use GitHub Container Registry")
		assert.True(t, strings.HasSuffix(ref, ":latest"), "OCI reference should use :latest tag")
		assert.Contains(t, ref, slug, "OCI reference should contain the challenge slug")
	}
}

func TestChallengesOCIRegistryFormat(t *testing.T) {
	assert.True(t, strings.HasPrefix(ChallengesOCIRegistry, "ghcr.io/"),
		"OCI registry should be on ghcr.io")
	assert.False(t, strings.HasSuffix(ChallengesOCIRegistry, "/"),
		"OCI registry should not have trailing slash")
}

func TestWalkDirSkipsNonYAML(t *testing.T) {
	tmpDir := t.TempDir()
	manifestsDir := filepath.Join(tmpDir, "manifests")
	require.NoError(t, os.MkdirAll(manifestsDir, 0o755))

	// Create files with various extensions
	files := map[string]bool{
		"deployment.yaml": true,
		"service.yml":     false, // Only .yaml is matched
		"readme.md":       false,
		"config.json":     false,
		"notes.txt":       false,
	}

	for name := range files {
		require.NoError(t, os.WriteFile(filepath.Join(manifestsDir, name), []byte("content"), 0o600))
	}

	var found []string
	err := filepath.WalkDir(manifestsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".yaml") {
			found = append(found, filepath.Base(path))
		}
		return nil
	})
	require.NoError(t, err)

	assert.Len(t, found, 1)
	assert.Contains(t, found, "deployment.yaml")
}

func TestWalkDirMissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	missingDir := filepath.Join(tmpDir, "nonexistent")

	// os.Stat should fail for missing directory, matching DeployChallenge behavior
	_, err := os.Stat(missingDir)
	assert.True(t, os.IsNotExist(err), "missing directory should be detected")
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
