package keystore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func TestMain(m *testing.M) {
	// Initialize mock keyring for all tests
	keyring.MockInit()
	os.Exit(m.Run())
}

func cleanupTestEnv(t *testing.T) {
	t.Helper()
	// Clear environment variable
	os.Unsetenv(EnvVarName)
	// Clear keyring
	_ = keyring.Delete("kubeasy-cli", "api_key")
	// Clear file storage
	_ = deleteFromFile()
}

func TestGet_FromEnvironment(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Set environment variable
	os.Setenv(EnvVarName, "env-token-123")

	token, err := Get()
	require.NoError(t, err)
	assert.Equal(t, "env-token-123", token)

	// Verify storage type
	assert.Equal(t, StorageEnv, GetStorageType())
}

func TestGet_FromKeyring(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Set keyring token
	err := keyring.Set("kubeasy-cli", "api_key", "keyring-token-456")
	require.NoError(t, err)

	token, err := Get()
	require.NoError(t, err)
	assert.Equal(t, "keyring-token-456", token)

	// Verify storage type
	assert.Equal(t, StorageKeyring, GetStorageType())
}

func TestGet_FromFile(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Set file-based token
	err := setToFile("file-token-789")
	require.NoError(t, err)

	token, err := Get()
	require.NoError(t, err)
	assert.Equal(t, "file-token-789", token)

	// Verify storage type
	assert.Equal(t, StorageFile, GetStorageType())
}

func TestGet_Priority(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Set all three storage types
	os.Setenv(EnvVarName, "env-token")
	_ = keyring.Set("kubeasy-cli", "api_key", "keyring-token")
	_ = setToFile("file-token")

	// Environment should have highest priority
	token, err := Get()
	require.NoError(t, err)
	assert.Equal(t, "env-token", token)

	// Remove env, keyring should be next
	os.Unsetenv(EnvVarName)
	token, err = Get()
	require.NoError(t, err)
	assert.Equal(t, "keyring-token", token)

	// Remove keyring, file should be last
	_ = keyring.Delete("kubeasy-cli", "api_key")
	token, err = Get()
	require.NoError(t, err)
	assert.Equal(t, "file-token", token)
}

func TestGet_NotFound(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	_, err := Get()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "please run 'kubeasy login'")
}

func TestSet_ToKeyring(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Mock keyring should work
	storageType, err := Set("test-api-key")
	require.NoError(t, err)
	assert.Equal(t, StorageKeyring, storageType)

	// Verify it was stored
	token, err := keyring.Get("kubeasy-cli", "api_key")
	require.NoError(t, err)
	assert.Equal(t, "test-api-key", token)
}

func TestDelete(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Set in both keyring and file
	_ = keyring.Set("kubeasy-cli", "api_key", "keyring-token")
	_ = setToFile("file-token")

	// Delete should clear both
	err := Delete()
	require.NoError(t, err)

	// Verify keyring is cleared
	_, err = keyring.Get("kubeasy-cli", "api_key")
	assert.Error(t, err)

	// Verify file is cleared
	_, err = getFromFile()
	assert.Error(t, err)
}

func TestFileStorage_Permissions(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Set file-based token
	err := setToFile("secure-token")
	require.NoError(t, err)

	// Get credentials path
	credPath, err := getCredentialsPath()
	require.NoError(t, err)

	// Check file permissions (should be 0600)
	info, err := os.Stat(credPath)
	require.NoError(t, err)

	// On Unix-like systems, check permissions
	// On Windows, this check may not work the same way
	mode := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0600), mode, "File should have restricted permissions")
}

func TestFileStorage_DirectoryCreation(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Get config dir path
	configDir, err := getConfigDir()
	require.NoError(t, err)

	// Remove directory if exists
	_ = os.RemoveAll(configDir)

	// Set token (should create directory)
	err = setToFile("new-token")
	require.NoError(t, err)

	// Verify directory was created
	info, err := os.Stat(configDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Check directory permissions (should be 0700)
	mode := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0700), mode, "Directory should have restricted permissions")
}

func TestGetStorageType_Empty(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// No storage set
	storageType := GetStorageType()
	assert.Equal(t, StorageType(""), storageType)
}

func TestIsKeyringAvailable(t *testing.T) {
	// With mock keyring, this should return true
	available := IsKeyringAvailable()
	assert.True(t, available)
}

func TestGetConfigDir_XDGConfigHome(t *testing.T) {
	// Save original value
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)

	// Set custom XDG_CONFIG_HOME
	tempDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tempDir)

	configDir, err := getConfigDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tempDir, configDirName), configDir)
}

func TestCredentialsFileFormat(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Set token
	err := setToFile("json-format-token")
	require.NoError(t, err)

	// Read raw file content
	credPath, err := getCredentialsPath()
	require.NoError(t, err)

	data, err := os.ReadFile(credPath)
	require.NoError(t, err)

	// Should be valid JSON
	assert.Contains(t, string(data), `"api_key"`)
	assert.Contains(t, string(data), `"json-format-token"`)
}

func TestGetFromFile_InvalidJSON(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Create credentials file with invalid JSON
	credPath, err := getCredentialsPath()
	require.NoError(t, err)

	// Ensure directory exists
	configDir, err := getConfigDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(configDir, 0700))

	// Write invalid JSON
	err = os.WriteFile(credPath, []byte("not valid json"), 0600)
	require.NoError(t, err)

	// getFromFile should return parse error
	_, err = getFromFile()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse credentials file")
}

func TestGetFromFile_EmptyAPIKey(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Create credentials file with empty API key
	credPath, err := getCredentialsPath()
	require.NoError(t, err)

	// Ensure directory exists
	configDir, err := getConfigDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(configDir, 0700))

	// Write valid JSON with empty api_key
	err = os.WriteFile(credPath, []byte(`{"api_key": ""}`), 0600)
	require.NoError(t, err)

	// getFromFile should return ErrNotFound
	_, err = getFromFile()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetFromFile_FileNotExist(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Ensure file doesn't exist
	credPath, err := getCredentialsPath()
	require.NoError(t, err)
	_ = os.Remove(credPath)

	// getFromFile should return ErrNotFound
	_, err = getFromFile()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetCredentialsPath(t *testing.T) {
	credPath, err := getCredentialsPath()
	require.NoError(t, err)
	assert.Contains(t, credPath, credentialsFileName)
	assert.Contains(t, credPath, configDirName)
}

func TestDeleteFromFile_NoFile(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Ensure file doesn't exist
	credPath, err := getCredentialsPath()
	require.NoError(t, err)
	_ = os.Remove(credPath)

	// deleteFromFile should not error when file doesn't exist
	err = deleteFromFile()
	require.NoError(t, err)
}

func TestDelete_NoCredentialsStored(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Delete when nothing is stored should succeed
	err := Delete()
	require.NoError(t, err)
}

func TestConcurrentAccess(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Test concurrent writes to file storage
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			_ = setToFile("token-concurrent")
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should successfully retrieve a token
	token, err := getFromFile()
	require.NoError(t, err)
	assert.Equal(t, "token-concurrent", token)
}

func TestConcurrentRead(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Set a token first
	err := setToFile("token-for-read")
	require.NoError(t, err)

	// Test concurrent reads
	done := make(chan string, 10)
	for i := 0; i < 10; i++ {
		go func() {
			token, err := getFromFile()
			if err != nil {
				done <- ""
			} else {
				done <- token
			}
		}()
	}

	// All reads should succeed with the same value
	for i := 0; i < 10; i++ {
		token := <-done
		assert.Equal(t, "token-for-read", token)
	}
}

func TestErrNotFound_Error(t *testing.T) {
	// Test that ErrNotFound has the expected error message
	assert.Equal(t, "credential not found", ErrNotFound.Error())
}

func TestStorageType_Values(t *testing.T) {
	// Test storage type constant values
	assert.Equal(t, StorageType("environment"), StorageEnv)
	assert.Equal(t, StorageType("keyring"), StorageKeyring)
	assert.Equal(t, StorageType("file"), StorageFile)
}

func TestRestrictFilePermissions(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Create a test file
	tempFile := filepath.Join(t.TempDir(), "test-perms")
	err := os.WriteFile(tempFile, []byte("test"), 0644)
	require.NoError(t, err)

	// restrictFilePermissions should succeed (no-op on Unix)
	err = restrictFilePermissions(tempFile)
	require.NoError(t, err)
}

func TestGetConfigDir_DefaultPath(t *testing.T) {
	// Save original value
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if origXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	// Unset XDG_CONFIG_HOME to test default behavior
	os.Unsetenv("XDG_CONFIG_HOME")

	configDir, err := getConfigDir()
	require.NoError(t, err)

	// Should use home directory based path
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	// On Unix, should be ~/.config/kubeasy-cli
	// On Windows, should be based on APPDATA
	assert.Contains(t, configDir, configDirName)
	assert.True(t, filepath.IsAbs(configDir))

	// Path should contain the home directory or be an absolute path
	// This is a loose check since Windows and Unix behave differently
	if os.Getenv("APPDATA") == "" {
		assert.Contains(t, configDir, homeDir)
	}
}

func TestSetToFile_CreateDirectory(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Get config directory and remove it
	configDir, err := getConfigDir()
	require.NoError(t, err)
	_ = os.RemoveAll(configDir)

	// Verify directory doesn't exist
	_, err = os.Stat(configDir)
	require.True(t, os.IsNotExist(err))

	// setToFile should create the directory
	err = setToFile("test-create-dir")
	require.NoError(t, err)

	// Verify directory now exists
	info, err := os.Stat(configDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestGet_KeyringErrorLogged(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Set only file-based token
	// Keyring will fail with ErrNotFound but continue to file
	err := setToFile("file-after-keyring-miss")
	require.NoError(t, err)

	// Get should succeed from file
	token, err := Get()
	require.NoError(t, err)
	assert.Equal(t, "file-after-keyring-miss", token)
}
