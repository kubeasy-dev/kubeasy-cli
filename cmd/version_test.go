package cmd

import (
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/semver"
)

func TestIsPreRelease(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"dev", true},
		{"nightly-abc1234", true},
		{"nightly", true},
		{"", true},
		{"2.6.0", false},
		{"v2.6.0", false},
		{"2.7.0-rc.1", false},
		{"v2.7.0-rc.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			if got := semver.IsPreRelease(tt.version); got != tt.want {
				t.Errorf("semver.IsPreRelease(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}
