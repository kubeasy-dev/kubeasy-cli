package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
)

// GetStateDir returns the per-challenge state directory (~/.kubeasy/state/<slug>).
func GetStateDir(slug string) string {
	return filepath.Join(constants.GetKubeasyConfigDir(), "state", slug)
}

// SaveTimestamp writes the current UTC time to the challenge state directory.
func SaveTimestamp(slug string) error {
	dir := GetStateDir(slug)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create state dir: %w", err)
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	return os.WriteFile(filepath.Join(dir, "timestamp"), []byte(ts), 0o600)
}

// LoadTimestamp reads the saved timestamp for the challenge.
// Returns a zero time and an error if the file does not exist or cannot be parsed.
func LoadTimestamp(slug string) (time.Time, error) {
	path := filepath.Join(GetStateDir(slug), "timestamp")
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}
	ts, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	return ts, nil
}

// ClearState removes the per-challenge state directory.
func ClearState(slug string) error {
	return os.RemoveAll(GetStateDir(slug))
}
