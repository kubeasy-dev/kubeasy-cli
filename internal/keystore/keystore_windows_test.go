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
	// Note: This test verifies that the current user retains access after ACL
	// restrictions are applied. Verifying that other users are denied access
	// would require running as a different user, which needs OS-level
	// integration testing with multiple Windows accounts.

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
}

func TestGetConfigDir_Windows_APPDATA(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("APPDATA", tempDir)

	configDir, err := getConfigDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tempDir, configDirName), configDir)
}

func TestGetConfigDir_Windows_FallbackToHomeDir(t *testing.T) {
	t.Setenv("APPDATA", "")

	configDir, err := getConfigDir()
	require.NoError(t, err)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	expected := filepath.Join(homeDir, "AppData", "Roaming", configDirName)
	assert.Equal(t, expected, configDir)
}
