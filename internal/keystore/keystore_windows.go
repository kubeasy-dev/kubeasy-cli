//go:build windows

package keystore

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
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
// On Windows, POSIX file permissions don't apply. This function sets a Windows
// ACL (Access Control List) that grants access only to the current user,
// blocking inheritance from the parent directory.
func restrictFilePermissions(path string) error {
	// Get the current user's SID from the process token.
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return fmt.Errorf("failed to open process token: %w", err)
	}
	defer token.Close()

	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("failed to get token user: %w", err)
	}

	sid := tokenUser.User.Sid

	// Build a DACL with a single entry granting full access only to the current user.
	ea := windows.EXPLICIT_ACCESS{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}

	// nil = create a fresh ACL, don't merge with an existing one.
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{ea}, nil)
	if err != nil {
		return fmt.Errorf("failed to create ACL: %w", err)
	}

	// Apply the DACL to the file. PROTECTED_DACL_SECURITY_INFORMATION prevents
	// the file from inheriting permissions from its parent directory.
	err = windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil,
	)
	if err != nil {
		return fmt.Errorf("failed to set file security: %w", err)
	}

	return nil
}
