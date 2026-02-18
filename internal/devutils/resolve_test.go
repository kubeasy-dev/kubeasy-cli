package devutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveLocalChallengeDir(t *testing.T) {
	t.Run("with dir flag pointing to valid challenge", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := os.WriteFile(filepath.Join(tmpDir, "challenge.yaml"), []byte("title: test"), 0600)
		require.NoError(t, err)

		result, err := ResolveLocalChallengeDir("any-slug", tmpDir)
		require.NoError(t, err)
		assert.Equal(t, tmpDir, result)
	})

	t.Run("with dir flag missing challenge.yaml", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := ResolveLocalChallengeDir("any-slug", tmpDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "challenge.yaml not found")
	})

	t.Run("with dir flag nonexistent path", func(t *testing.T) {
		_, err := ResolveLocalChallengeDir("any-slug", "/nonexistent/path/that/does/not/exist")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "challenge.yaml not found")
	})

	t.Run("finds slug subdirectory", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldWd) }()

		// Create <slug>/challenge.yaml
		slugDir := filepath.Join(tmpDir, "my-challenge")
		err := os.MkdirAll(slugDir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(slugDir, "challenge.yaml"), []byte("title: test"), 0600)
		require.NoError(t, err)

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		result, err := ResolveLocalChallengeDir("my-challenge", "")
		require.NoError(t, err)

		// Resolve symlinks for macOS (/var -> /private/var)
		expectedDir, _ := filepath.EvalSymlinks(slugDir)
		resultResolved, _ := filepath.EvalSymlinks(result)
		assert.Equal(t, expectedDir, resultResolved)
	})

	t.Run("finds current directory when inside challenge dir", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldWd) }()

		// Create challenge.yaml in tmpDir itself
		err := os.WriteFile(filepath.Join(tmpDir, "challenge.yaml"), []byte("title: test"), 0600)
		require.NoError(t, err)

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		result, err := ResolveLocalChallengeDir("some-slug", "")
		require.NoError(t, err)

		absTmpDir, _ := filepath.Abs(".")
		assert.Equal(t, absTmpDir, result)
	})

	t.Run("prefers slug subdirectory over current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldWd) }()

		// Create both: challenge.yaml in cwd AND <slug>/challenge.yaml
		err := os.WriteFile(filepath.Join(tmpDir, "challenge.yaml"), []byte("title: cwd"), 0600)
		require.NoError(t, err)

		slugDir := filepath.Join(tmpDir, "my-challenge")
		err = os.MkdirAll(slugDir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(slugDir, "challenge.yaml"), []byte("title: slug"), 0600)
		require.NoError(t, err)

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		result, err := ResolveLocalChallengeDir("my-challenge", "")
		require.NoError(t, err)

		// Resolve symlinks for macOS (/var -> /private/var)
		expectedDir, _ := filepath.EvalSymlinks(slugDir)
		resultResolved, _ := filepath.EvalSymlinks(result)
		assert.Equal(t, expectedDir, resultResolved)
	})

	t.Run("returns error when nothing found", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldWd) }()

		err := os.Chdir(tmpDir)
		require.NoError(t, err)

		_, err = ResolveLocalChallengeDir("nonexistent", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not find challenge directory")
		assert.Contains(t, err.Error(), "--dir")
	})

	t.Run("dir flag takes precedence over slug subdirectory", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(oldWd) }()

		// Create slug subdir with challenge.yaml
		slugDir := filepath.Join(tmpDir, "my-challenge")
		err := os.MkdirAll(slugDir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(slugDir, "challenge.yaml"), []byte("title: slug"), 0600)
		require.NoError(t, err)

		// Create separate dir with challenge.yaml
		otherDir := filepath.Join(tmpDir, "other-dir")
		err = os.MkdirAll(otherDir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(otherDir, "challenge.yaml"), []byte("title: other"), 0600)
		require.NoError(t, err)

		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		result, err := ResolveLocalChallengeDir("my-challenge", otherDir)
		require.NoError(t, err)
		assert.Equal(t, otherDir, result)
	})
}
