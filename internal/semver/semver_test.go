package semver_test

import (
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/semver"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"v2.7.0-rc.1", "2.7.0"},
		{"1.2.3+build.1", "1.2.3"},
		{"v0.0.1-alpha+meta", "0.0.1"},
		{"  v1.0.0  ", "1.0.0"},
	}
	for _, tc := range tests {
		got := semver.Normalize(tc.input)
		if got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
		{"1.10.0", "1.9.0", 1},
		{"0.0.0", "0.0.1", -1},
	}
	for _, tc := range tests {
		got := semver.Compare(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestIsPreRelease(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"dev", true},
		{"nightly-abc1234", true},
		{"v1.0.0", false},
		{"1.2.3", false},
		{"v2.7.0-rc.1", false}, // starts with digit after v
		{"", true},
	}
	for _, tc := range tests {
		got := semver.IsPreRelease(tc.input)
		if got != tc.want {
			t.Errorf("IsPreRelease(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
