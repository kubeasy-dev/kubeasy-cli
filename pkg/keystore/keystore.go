// Package keystore provides a unified interface for storing and retrieving
// credentials with automatic fallback for headless environments.
//
// The storage priority is:
//  1. Environment variable (KUBEASY_API_KEY) - read only, useful for CI/CD
//  2. System keyring (go-keyring) - preferred for GUI environments
//  3. File-based storage - fallback for headless environments:
//     - Linux/macOS: ~/.config/kubeasy-cli/credentials (XDG spec)
//     - Windows: %APPDATA%/kubeasy-cli/credentials
//
// Security Notes:
//   - File permissions are restricted to owner-only access (0600) on Unix systems
//   - On Windows, the file is stored in the user's APPDATA directory
//   - The keyring is always preferred when available for maximum security
package keystore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/constants"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/logger"
	"github.com/zalando/go-keyring"
)

const (
	// EnvVarName is the name of the environment variable used to provide the API key.
	// This is NOT a credential itself, just the variable name to look up.
	EnvVarName = "KUBEASY_API_KEY"

	// credentialsFileName is the name of the file used for file-based storage
	credentialsFileName = "credentials"

	// configDirName is the name of the config directory
	configDirName = "kubeasy-cli"
)

var (
	// ErrNotFound is returned when no credential is found
	ErrNotFound = errors.New("credential not found")

	// mu protects concurrent access to the file-based storage
	mu sync.Mutex
)

// credentials represents the structure of the credentials file
type credentials struct {
	APIKey string `json:"api_key,omitempty"`
}

// StorageType indicates which storage backend is being used
type StorageType string

const (
	StorageEnv     StorageType = "environment"
	StorageKeyring StorageType = "keyring"
	StorageFile    StorageType = "file"
)

// Get retrieves the API key from available storage backends.
// It checks in order: environment variable, keyring, file-based storage.
func Get() (string, error) {
	// 1. Check environment variable first
	if envKey := os.Getenv(EnvVarName); envKey != "" {
		logger.Debug("API key found in environment variable")
		return envKey, nil
	}

	// 2. Try system keyring
	key, err := keyring.Get(constants.KeyringServiceName, "api_key")
	if err == nil && key != "" {
		logger.Debug("API key found in system keyring")
		return key, nil
	}

	// Log keyring error for debugging (but don't fail yet)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		logger.Debug("Keyring access failed: %v", err)
	}

	// 3. Fall back to file-based storage
	key, err = getFromFile()
	if err == nil && key != "" {
		logger.Debug("API key found in file-based storage")
		return key, nil
	}

	return "", fmt.Errorf("%w: please run 'kubeasy login'", ErrNotFound)
}

// Set stores the API key in the best available storage backend.
// It tries keyring first, then falls back to file-based storage.
// Returns the storage type used and any error.
func Set(apiKey string) (StorageType, error) {
	// Try system keyring first
	err := keyring.Set(constants.KeyringServiceName, "api_key", apiKey)
	if err == nil {
		logger.Debug("API key stored in system keyring")
		return StorageKeyring, nil
	}

	logger.Debug("Keyring storage failed: %v, falling back to file", err)

	// Fall back to file-based storage
	if err := setToFile(apiKey); err != nil {
		return "", fmt.Errorf("failed to store API key: %w", err)
	}

	logger.Debug("API key stored in file-based storage")
	return StorageFile, nil
}

// Delete removes the API key from all storage backends.
// Returns a combined error if multiple backends fail.
func Delete() error {
	var errs []error

	// Delete from keyring
	if err := keyring.Delete(constants.KeyringServiceName, "api_key"); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		logger.Debug("Failed to delete from keyring: %v", err)
		errs = append(errs, fmt.Errorf("keyring: %w", err))
	}

	// Delete from file-based storage
	if err := deleteFromFile(); err != nil {
		logger.Debug("Failed to delete from file: %v", err)
		errs = append(errs, fmt.Errorf("file: %w", err))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// GetStorageType returns which storage backend currently holds the API key.
func GetStorageType() StorageType {
	if os.Getenv(EnvVarName) != "" {
		return StorageEnv
	}

	if key, err := keyring.Get(constants.KeyringServiceName, "api_key"); err == nil && key != "" {
		return StorageKeyring
	}

	if key, err := getFromFile(); err == nil && key != "" {
		return StorageFile
	}

	return ""
}

// IsKeyringAvailable checks if the system keyring is accessible.
func IsKeyringAvailable() bool {
	// Try to get a non-existent key to test keyring availability
	_, err := keyring.Get(constants.KeyringServiceName, "__test_availability__")
	// ErrNotFound means keyring is available but key doesn't exist
	// Other errors mean keyring is not available
	return err == nil || errors.Is(err, keyring.ErrNotFound)
}

// getCredentialsPath returns the full path to the credentials file
func getCredentialsPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, credentialsFileName), nil
}

// getFromFile retrieves the API key from file-based storage
func getFromFile() (string, error) {
	mu.Lock()
	defer mu.Unlock()

	credPath, err := getCredentialsPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(credPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("failed to read credentials file: %w", err)
	}

	var creds credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", fmt.Errorf("failed to parse credentials file: %w", err)
	}

	if creds.APIKey == "" {
		return "", ErrNotFound
	}

	return creds.APIKey, nil
}

// setToFile stores the API key in file-based storage
func setToFile(apiKey string) error {
	mu.Lock()
	defer mu.Unlock()

	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	// Create config directory with restricted permissions
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	credPath := filepath.Join(configDir, credentialsFileName)

	creds := credentials{
		APIKey: apiKey,
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Write with restricted permissions (owner read/write only)
	if err := os.WriteFile(credPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	// Apply platform-specific permission restrictions
	if err := restrictFilePermissions(credPath); err != nil {
		return fmt.Errorf("failed to restrict file permissions: %w", err)
	}

	return nil
}

// deleteFromFile removes the credentials file
func deleteFromFile() error {
	mu.Lock()
	defer mu.Unlock()

	credPath, err := getCredentialsPath()
	if err != nil {
		return err
	}

	if err := os.Remove(credPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete credentials file: %w", err)
	}

	return nil
}
