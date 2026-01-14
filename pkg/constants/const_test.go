package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetKubernetesVersion(t *testing.T) {
	t.Run("extracts version from KindNodeImage", func(t *testing.T) {
		version := GetKubernetesVersion()

		// Should not be empty or UnknownVersion
		assert.NotEmpty(t, version)
		assert.NotEqual(t, UnknownVersion, version)

		// Should be a valid semver format (X.Y.Z or X.Y.Z-prerelease)
		assert.Regexp(t, `^\d+\.\d+\.\d+`, version)
	})

	t.Run("returns version without v prefix", func(t *testing.T) {
		version := GetKubernetesVersion()

		// Should not start with "v"
		assert.NotEqual(t, "v", string(version[0]), "version should not start with 'v' prefix")
	})

	t.Run("matches expected version from KindNodeImage", func(t *testing.T) {
		version := GetKubernetesVersion()

		// Verify it matches the version in KindNodeImage
		assert.Contains(t, KindNodeImage, version, "version should be extracted from KindNodeImage")
	})
}

func TestGetMajorMinorVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Standard versions
		{
			name:     "standard version",
			input:    "1.35.0",
			expected: "1.35",
		},
		{
			name:     "with v prefix",
			input:    "v1.35.0",
			expected: "1.35",
		},
		// Build metadata
		{
			name:     "k3s build metadata",
			input:    "1.35.0+k3s1",
			expected: "1.35",
		},
		{
			name:     "eks build metadata",
			input:    "1.35.0-eks",
			expected: "1.35",
		},
		{
			name:     "eks with build number",
			input:    "1.35.0-eks-a1b2c3",
			expected: "1.35",
		},
		{
			name:     "gke build metadata",
			input:    "1.35.0-gke.1234",
			expected: "1.35",
		},
		// Prerelease versions
		{
			name:     "alpha prerelease",
			input:    "1.35.0-alpha.1",
			expected: "1.35",
		},
		{
			name:     "beta prerelease",
			input:    "1.35.0-beta.2",
			expected: "1.35",
		},
		{
			name:     "rc prerelease",
			input:    "1.35.0-rc.1",
			expected: "1.35",
		},
		// Edge cases
		{
			name:     "major.minor only",
			input:    "1.35",
			expected: "1.35",
		},
		{
			name:     "single number",
			input:    "1",
			expected: UnknownVersion,
		},
		{
			name:     "empty string",
			input:    "",
			expected: UnknownVersion,
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: UnknownVersion,
		},
		{
			name:     "v prefix with build metadata",
			input:    "v1.35.0+k3s1",
			expected: "1.35",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMajorMinorVersion(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVersionsCompatible(t *testing.T) {
	tests := []struct {
		name       string
		version1   string
		version2   string
		compatible bool
	}{
		// Compatible versions (same major.minor)
		{
			name:       "identical versions",
			version1:   "1.35.0",
			version2:   "1.35.0",
			compatible: true,
		},
		{
			name:       "different patch versions",
			version1:   "1.35.0",
			version2:   "1.35.1",
			compatible: true,
		},
		{
			name:       "different build metadata",
			version1:   "1.35.0",
			version2:   "1.35.0+k3s1",
			compatible: true,
		},
		{
			name:       "eks vs standard",
			version1:   "1.35.0-eks",
			version2:   "1.35.0",
			compatible: true,
		},
		{
			name:       "with and without v prefix",
			version1:   "v1.35.0",
			version2:   "1.35.0",
			compatible: true,
		},
		{
			name:       "k3s vs gke",
			version1:   "1.35.0+k3s1",
			version2:   "1.35.0-gke.1234",
			compatible: true,
		},
		// Incompatible versions (different major.minor)
		{
			name:       "different minor versions",
			version1:   "1.35.0",
			version2:   "1.34.0",
			compatible: false,
		},
		{
			name:       "different major versions",
			version1:   "1.35.0",
			version2:   "2.35.0",
			compatible: false,
		},
		{
			name:       "minor version difference with build metadata",
			version1:   "1.35.0+k3s1",
			version2:   "1.34.0+k3s1",
			compatible: false,
		},
		// Edge cases
		{
			name:       "invalid version 1",
			version1:   "invalid",
			version2:   "1.35.0",
			compatible: false,
		},
		{
			name:       "invalid version 2",
			version1:   "1.35.0",
			version2:   "invalid",
			compatible: false,
		},
		{
			name:       "both invalid",
			version1:   "invalid",
			version2:   "also-invalid",
			compatible: false,
		},
		{
			name:       "empty versions",
			version1:   "",
			version2:   "",
			compatible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VersionsCompatible(tt.version1, tt.version2)
			assert.Equal(t, tt.compatible, result)
		})
	}
}
