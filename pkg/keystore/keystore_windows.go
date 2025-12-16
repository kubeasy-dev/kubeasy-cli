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
// from the parent directory.
//
// TODO(security): Implement Windows ACL restrictions using golang.org/x/sys/windows
// to properly restrict file access beyond APPDATA directory permissions.
// This would involve setting an ACL that grants access only to the current user.
// See: https://pkg.go.dev/golang.org/x/sys/windows for ACL APIs.
//
// Note: The Windows Credential Manager (used by go-keyring) is the preferred
// storage method on Windows and provides proper security isolation. File-based
// storage should only be used as a fallback when Credential Manager is unavailable.
func restrictFilePermissions(_ string) error {
	// On Windows, file permissions work differently than Unix.
	// The 0600 mode passed to os.WriteFile is essentially ignored.
	// For proper security, Windows ACLs would need to be configured.
	// However, the file is stored in the user's APPDATA directory which
	// is already protected by Windows user permissions.
	return nil
}
