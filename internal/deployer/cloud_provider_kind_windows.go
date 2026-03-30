//go:build windows

package deployer

import "fmt"

// isCloudProviderKindRunning always returns false on Windows.
// cloud-provider-kind is not supported on Windows.
func isCloudProviderKindRunning() bool {
	return false
}

// startCloudProviderKindDetached always returns an error on Windows.
// cloud-provider-kind is not supported on Windows.
func startCloudProviderKindDetached(_ string) error {
	return fmt.Errorf("cloud-provider-kind is not supported on Windows")
}
