package deployer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasImageDir(t *testing.T) {
	t.Run("returns true when image/Dockerfile exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		imageDir := filepath.Join(tmpDir, "image")
		err := os.MkdirAll(imageDir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(imageDir, "Dockerfile"), []byte("FROM nginx"), 0600)
		require.NoError(t, err)

		assert.True(t, HasImageDir(tmpDir))
	})

	t.Run("returns false when image dir missing", func(t *testing.T) {
		tmpDir := t.TempDir()

		assert.False(t, HasImageDir(tmpDir))
	})

	t.Run("returns false when image dir exists without Dockerfile", func(t *testing.T) {
		tmpDir := t.TempDir()
		imageDir := filepath.Join(tmpDir, "image")
		err := os.MkdirAll(imageDir, 0755)
		require.NoError(t, err)

		assert.False(t, HasImageDir(tmpDir))
	})

	t.Run("returns false for nonexistent directory", func(t *testing.T) {
		assert.False(t, HasImageDir("/nonexistent/path"))
	})
}
