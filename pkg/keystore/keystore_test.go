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
	os.Unsetenv(EnvAPIKey)
	// Clear keyring
	_ = keyring.Delete("kubeasy-cli", "api_key")
	// Clear file storage
	_ = deleteFromFile()
}

func TestGet_FromEnvironment(t *testing.T) {
	cleanupTestEnv(t)
	defer cleanupTestEnv(t)

	// Set environment variable
	os.Setenv(EnvAPIKey, "env-token-123")

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
	os.Setenv(EnvAPIKey, "env-token")
	_ = keyring.Set("kubeasy-cli", "api_key", "keyring-token")
	_ = setToFile("file-token")

	// Environment should have highest priority
	token, err := Get()
	require.NoError(t, err)
	assert.Equal(t, "env-token", token)

	// Remove env, keyring should be next
	os.Unsetenv(EnvAPIKey)
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
