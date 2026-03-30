//go:build !windows

package deployer

import (
	"fmt"
	"os/exec"
	"syscall"

	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
)

// isCloudProviderKindRunning checks whether a cloud-provider-kind process is running
// by querying pgrep. Returns true if pgrep exits with code 0 (process found).
func isCloudProviderKindRunning() bool {
	cmd := exec.Command("pgrep", "-f", "cloud-provider-kind")
	return cmd.Run() == nil
}

// startCloudProviderKindDetached starts cloud-provider-kind as a detached background process.
// It uses Setsid=true to create a new session so the process survives terminal closure.
// cmd.Start() is called but never cmd.Wait(), leaving the process running independently.
func startCloudProviderKindDetached(binPath string) error {
	cmd := exec.Command(binPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start cloud-provider-kind: %w", err)
	}

	logger.Info("cloud-provider-kind started as detached process (PID %d)", cmd.Process.Pid)
	return nil
}
