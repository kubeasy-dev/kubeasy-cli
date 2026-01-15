//go:build !windows

package keystore

import (
	"fmt"
	"os"
	"path/filepath"
)

// getConfigDir returns the path to the config directory.
// On Unix systems, this follows the XDG Base Directory specification.
func getConfigDir() (string, error) {
	// Use XDG_CONFIG_HOME if set, otherwise use ~/.config
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		configHome = filepath.Join(homeDir, ".config")
	}

	return filepath.Join(configHome, configDirName), nil
}

// restrictFilePermissions ensures the file has proper restricted permissions.
// On Unix systems, os.WriteFile already sets permissions correctly via the mode parameter.
func restrictFilePermissions(_ string) error {
	return nil
}
