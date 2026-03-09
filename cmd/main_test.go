package cmd

import (
	"os"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
)

// TestMain enables CI mode for all cmd package tests to avoid pterm spinner
// goroutine data races during testing.
func TestMain(m *testing.M) {
	ui.SetCIMode(true)
	os.Exit(m.Run())
}
