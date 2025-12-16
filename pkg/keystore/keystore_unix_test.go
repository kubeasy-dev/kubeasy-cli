//go:build !windows

package keystore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfigDir_Unix_XDGConfigHome(t *testing.T) {
	// Save original values
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if origXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	// Test with custom XDG_CONFIG_HOME
	tempDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tempDir)

	configDir, err := getConfigDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tempDir, configDirName), configDir)
}

func TestGetConfigDir_Unix_Default(t *testing.T) {
	// Save original values
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if origXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	// Unset XDG_CONFIG_HOME
	os.Unsetenv("XDG_CONFIG_HOME")

	configDir, err := getConfigDir()
	require.NoError(t, err)

	// Should use ~/.config/kubeasy-cli
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homeDir, ".config", configDirName), configDir)
}

func TestRestrictFilePermissions_Unix_NoOp(t *testing.T) {
	// Create a test file
	tempFile := filepath.Join(t.TempDir(), "test-restrict")
	err := os.WriteFile(tempFile, []byte("test"), 0644)
	require.NoError(t, err)

	// restrictFilePermissions is a no-op on Unix
	err = restrictFilePermissions(tempFile)
	require.NoError(t, err)

	// File should still be readable
	_, err = os.ReadFile(tempFile)
	require.NoError(t, err)
}
