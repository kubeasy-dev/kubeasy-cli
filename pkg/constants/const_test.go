package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetKubernetesVersion(t *testing.T) {
	t.Run("extracts version from KindNodeImage", func(t *testing.T) {
		version := GetKubernetesVersion()

		// Should not be empty or "unknown"
		assert.NotEmpty(t, version)
		assert.NotEqual(t, "unknown", version)

		// Should be a valid semver format (X.Y.Z)
		assert.Regexp(t, `^\d+\.\d+\.\d+$`, version)
	})

	t.Run("returns version without v prefix", func(t *testing.T) {
		version := GetKubernetesVersion()

		// Should not start with "v"
		assert.NotEqual(t, "v", string(version[0]), "version should not start with 'v' prefix")
	})

	t.Run("matches expected version from KindNodeImage", func(t *testing.T) {
		// KindNodeImage is "kindest/node:v1.35.0"
		// GetKubernetesVersion() should return "1.35.0"
		version := GetKubernetesVersion()

		// Verify it matches the version in KindNodeImage
		assert.Contains(t, KindNodeImage, version, "version should be extracted from KindNodeImage")
	})
}
