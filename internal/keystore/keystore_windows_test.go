//go:build windows

package keystore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestrictFilePermissions_Windows(t *testing.T) {
	// Create a temporary file
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test-acl")
	err := os.WriteFile(tempFile, []byte("sensitive-data"), 0600)
	require.NoError(t, err)

	// Apply ACL restrictions
	err = restrictFilePermissions(tempFile)
	require.NoError(t, err)

	// File should still be readable by the current user
	data, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, "sensitive-data", string(data))

	// File should still be writable by the current user
	err = os.WriteFile(tempFile, []byte("updated-data"), 0600)
	require.NoError(t, err)

	data, err = os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Equal(t, "updated-data", string(data))
}

func TestRestrictFilePermissions_Windows_NonExistentFile(t *testing.T) {
	err := restrictFilePermissions(filepath.Join(t.TempDir(), "nonexistent"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to set file security")
}

func TestGetConfigDir_Windows_APPDATA(t *testing.T) {
	// Save original value
	origAppData := os.Getenv("APPDATA")
	defer os.Setenv("APPDATA", origAppData)

	// Set custom APPDATA
	tempDir := t.TempDir()
	os.Setenv("APPDATA", tempDir)

	configDir, err := getConfigDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tempDir, configDirName), configDir)
}

func TestGetConfigDir_Windows_FallbackToHomeDir(t *testing.T) {
	// Save original value
	origAppData := os.Getenv("APPDATA")
	defer func() {
		if origAppData != "" {
			os.Setenv("APPDATA", origAppData)
		} else {
			os.Unsetenv("APPDATA")
		}
	}()

	// Unset APPDATA to trigger fallback
	os.Unsetenv("APPDATA")

	configDir, err := getConfigDir()
	require.NoError(t, err)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	expected := filepath.Join(homeDir, "AppData", "Roaming", configDirName)
	assert.Equal(t, expected, configDir)
}
