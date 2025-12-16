//go:build windows

package keystore

import (
	"fmt"
	"os"
	"path/filepath"
)

// getConfigDir returns the path to the config directory.
// On Windows, this uses %APPDATA% which is the standard location for application data.
func getConfigDir() (string, error) {
	// Use APPDATA on Windows (typically C:\Users\<user>\AppData\Roaming)
	appData := os.Getenv("APPDATA")
	if appData == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		appData = filepath.Join(homeDir, "AppData", "Roaming")
	}

	return filepath.Join(appData, configDirName), nil
}

// restrictFilePermissions ensures the file has proper restricted permissions.
// On Windows, POSIX file permissions don't apply. The file inherits permissions
// from the parent directory. For production use, consider implementing proper
// Windows ACL restrictions using golang.org/x/sys/windows.
//
// Note: The Windows Credential Manager (used by go-keyring) is the preferred
// storage method on Windows and provides proper security isolation.
func restrictFilePermissions(_ string) error {
	// On Windows, file permissions work differently than Unix.
	// The 0600 mode passed to os.WriteFile is essentially ignored.
	// For proper security, Windows ACLs would need to be configured.
	// However, the file is stored in the user's APPDATA directory which
	// is already protected by Windows user permissions.
	return nil
}
