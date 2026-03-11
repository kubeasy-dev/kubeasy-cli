package deployer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
)

// cloudProviderKindBinaryURLForPlatform returns the download URL for the cloud-provider-kind binary
// for the given OS and architecture. The version tag uses the full "v"-prefixed version, while
// the filename uses the version without the "v" prefix.
func cloudProviderKindBinaryURLForPlatform(goos, goarch string) string {
	version := strings.TrimPrefix(CloudProviderKindVersion, "v")
	return fmt.Sprintf(
		"https://github.com/kubernetes-sigs/cloud-provider-kind/releases/download/%s/cloud-provider-kind_%s_%s_%s.tar.gz",
		CloudProviderKindVersion, version, goos, goarch,
	)
}

// cloudProviderKindBinaryURL returns the download URL for the cloud-provider-kind binary
// for the current platform (runtime.GOOS and runtime.GOARCH).
func cloudProviderKindBinaryURL() string {
	return cloudProviderKindBinaryURLForPlatform(runtime.GOOS, runtime.GOARCH)
}

// isCloudProviderKindRunning checks whether a cloud-provider-kind process is running
// by querying pgrep. Returns true if pgrep exits with code 0 (process found).
func isCloudProviderKindRunning() bool {
	cmd := exec.Command("pgrep", "-f", "cloud-provider-kind")
	return cmd.Run() == nil
}

// downloadCloudProviderKind downloads the cloud-provider-kind binary tar.gz from the given URL,
// extracts the binary, writes it to destPath, and sets permissions to 0755.
// This function uses net/http directly (not kube.FetchManifest) because FetchManifest
// validates URLs against a Kubernetes manifest allowlist that excludes binary downloads.
func downloadCloudProviderKind(url, destPath string) error {
	logger.Info("Downloading cloud-provider-kind from %s", url)

	// Create destination directory if it does not exist
	if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil {
		return fmt.Errorf("failed to create directory for cloud-provider-kind binary: %w", err)
	}

	// Download the tar.gz archive
	resp, err := http.Get(url) //nolint:gosec // URL is constructed from the trusted CloudProviderKindVersion constant
	if err != nil {
		return fmt.Errorf("failed to download cloud-provider-kind: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download cloud-provider-kind: HTTP %d", resp.StatusCode)
	}

	// Read into memory
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read cloud-provider-kind response body: %w", err)
	}

	// Parse the tar.gz archive from memory using bytes.NewReader for correctness
	// (strings.NewReader would corrupt binary data).
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader for cloud-provider-kind archive: %w", err)
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)

	// Find the binary in the archive
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read cloud-provider-kind archive: %w", err)
		}

		// The binary name in the archive is "cloud-provider-kind"
		if filepath.Base(hdr.Name) == "cloud-provider-kind" {
			out, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create cloud-provider-kind binary file: %w", err)
			}
			defer func() { _ = out.Close() }()

			if _, err := io.Copy(out, tr); err != nil { //nolint:gosec // binary from trusted source
				return fmt.Errorf("failed to write cloud-provider-kind binary: %w", err)
			}

			if err := os.Chmod(destPath, 0o755); err != nil {
				return fmt.Errorf("failed to chmod cloud-provider-kind binary: %w", err)
			}

			logger.Info("cloud-provider-kind binary extracted to %s", destPath)
			return nil
		}
	}

	return fmt.Errorf("cloud-provider-kind binary not found in archive from %s", url)
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

// ensureCloudProviderKind ensures the cloud-provider-kind process is running.
// If it is already running, returns StatusReady immediately.
// If the binary is not present, downloads it first.
// Then starts it as a detached process and returns StatusReady.
// Returns StatusNotReady on any error.
func ensureCloudProviderKind(_ context.Context) ComponentResult {
	const name = "cloud-provider-kind"

	// Already running — nothing to do
	if isCloudProviderKindRunning() {
		logger.Info("cloud-provider-kind is already running")
		return ComponentResult{Name: name, Status: StatusReady, Message: "already running"}
	}

	binPath := constants.GetCloudProviderKindBinPath()

	// Download binary if not present
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		url := cloudProviderKindBinaryURL()
		if err := downloadCloudProviderKind(url, binPath); err != nil {
			return notReady(name, fmt.Errorf("failed to download cloud-provider-kind: %w", err))
		}
	}

	// Start the process detached
	if err := startCloudProviderKindDetached(binPath); err != nil {
		return notReady(name, err)
	}

	return ComponentResult{Name: name, Status: StatusReady, Message: "started"}
}
