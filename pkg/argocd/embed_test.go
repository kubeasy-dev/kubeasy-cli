package argocd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestGetArgoCDAppManifest tests that the ArgoCD application manifest is properly embedded
func TestGetArgoCDAppManifest(t *testing.T) {
	t.Run("returns valid YAML content", func(t *testing.T) {
		manifest, err := GetArgoCDAppManifest()
		require.NoError(t, err)
		require.NotEmpty(t, manifest)

		// Verify it's valid YAML
		var parsed map[string]interface{}
		err = yaml.Unmarshal(manifest, &parsed)
		require.NoError(t, err, "Manifest should be valid YAML")
	})

	t.Run("contains ArgoCD Application resource", func(t *testing.T) {
		manifest, err := GetArgoCDAppManifest()
		require.NoError(t, err)

		var app map[string]interface{}
		err = yaml.Unmarshal(manifest, &app)
		require.NoError(t, err)

		assert.Equal(t, "argoproj.io/v1alpha1", app["apiVersion"])
		assert.Equal(t, "Application", app["kind"])

		metadata, ok := app["metadata"].(map[string]interface{})
		require.True(t, ok, "Metadata should be a map")
		assert.Equal(t, "argocd", metadata["name"])
		assert.Equal(t, "argocd", metadata["namespace"])
	})

	t.Run("has correct source configuration", func(t *testing.T) {
		manifest, err := GetArgoCDAppManifest()
		require.NoError(t, err)

		var app map[string]interface{}
		err = yaml.Unmarshal(manifest, &app)
		require.NoError(t, err)

		spec, ok := app["spec"].(map[string]interface{})
		require.True(t, ok, "Spec should be a map")

		source, ok := spec["source"].(map[string]interface{})
		require.True(t, ok, "Source should be a map")

		assert.Contains(t, source["repoURL"], "argoproj/argo-cd")
		// Version should be a semver tag (e.g., v3.0.2) managed by Renovate
		assert.Regexp(t, `^v\d+\.\d+\.\d+$`, source["targetRevision"])
	})

	t.Run("has automated sync policy", func(t *testing.T) {
		manifest, err := GetArgoCDAppManifest()
		require.NoError(t, err)

		var app map[string]interface{}
		err = yaml.Unmarshal(manifest, &app)
		require.NoError(t, err)

		spec, ok := app["spec"].(map[string]interface{})
		require.True(t, ok)

		syncPolicy, ok := spec["syncPolicy"].(map[string]interface{})
		require.True(t, ok, "SyncPolicy should be present")

		automated, ok := syncPolicy["automated"].(map[string]interface{})
		require.True(t, ok, "Automated sync should be configured")
		assert.Equal(t, true, automated["prune"])
		assert.Equal(t, true, automated["selfHeal"])
	})
}

// TestGetKyvernoAppManifest tests that the Kyverno application manifest is properly embedded
func TestGetKyvernoAppManifest(t *testing.T) {
	t.Run("returns valid YAML content", func(t *testing.T) {
		manifest, err := GetKyvernoAppManifest()
		require.NoError(t, err)
		require.NotEmpty(t, manifest)

		// Verify it's valid YAML
		var parsed map[string]interface{}
		err = yaml.Unmarshal(manifest, &parsed)
		require.NoError(t, err, "Manifest should be valid YAML")
	})

	t.Run("contains Kyverno Application resource", func(t *testing.T) {
		manifest, err := GetKyvernoAppManifest()
		require.NoError(t, err)

		var app map[string]interface{}
		err = yaml.Unmarshal(manifest, &app)
		require.NoError(t, err)

		assert.Equal(t, "argoproj.io/v1alpha1", app["apiVersion"])
		assert.Equal(t, "Application", app["kind"])

		metadata, ok := app["metadata"].(map[string]interface{})
		require.True(t, ok, "Metadata should be a map")
		assert.Equal(t, "kyverno", metadata["name"])
		assert.Equal(t, "argocd", metadata["namespace"])
	})

	t.Run("has Helm chart source configuration", func(t *testing.T) {
		manifest, err := GetKyvernoAppManifest()
		require.NoError(t, err)

		var app map[string]interface{}
		err = yaml.Unmarshal(manifest, &app)
		require.NoError(t, err)

		spec, ok := app["spec"].(map[string]interface{})
		require.True(t, ok, "Spec should be a map")

		source, ok := spec["source"].(map[string]interface{})
		require.True(t, ok, "Source should be a map")

		assert.Contains(t, source["repoURL"], "kyverno.github.io/kyverno")
		assert.Equal(t, "kyverno", source["chart"])
	})

	t.Run("targets kyverno namespace", func(t *testing.T) {
		manifest, err := GetKyvernoAppManifest()
		require.NoError(t, err)

		var app map[string]interface{}
		err = yaml.Unmarshal(manifest, &app)
		require.NoError(t, err)

		spec, ok := app["spec"].(map[string]interface{})
		require.True(t, ok)

		destination, ok := spec["destination"].(map[string]interface{})
		require.True(t, ok, "Destination should be present")
		assert.Equal(t, "kyverno", destination["namespace"])
	})

	t.Run("has automated sync policy", func(t *testing.T) {
		manifest, err := GetKyvernoAppManifest()
		require.NoError(t, err)

		var app map[string]interface{}
		err = yaml.Unmarshal(manifest, &app)
		require.NoError(t, err)

		spec, ok := app["spec"].(map[string]interface{})
		require.True(t, ok)

		syncPolicy, ok := spec["syncPolicy"].(map[string]interface{})
		require.True(t, ok, "SyncPolicy should be present")

		automated, ok := syncPolicy["automated"].(map[string]interface{})
		require.True(t, ok, "Automated sync should be configured")
		assert.Equal(t, true, automated["prune"])
		assert.Equal(t, true, automated["selfHeal"])
	})
}

// TestGetAllAppManifests tests that all manifests are returned correctly
func TestGetAllAppManifests(t *testing.T) {
	t.Run("returns all expected manifests", func(t *testing.T) {
		manifests, err := GetAllAppManifests()
		require.NoError(t, err)
		require.NotNil(t, manifests)

		assert.Len(t, manifests, 2, "Should return 2 manifests")
		assert.Contains(t, manifests, "argocd")
		assert.Contains(t, manifests, "kyverno")
	})

	t.Run("argocd manifest matches GetArgoCDAppManifest", func(t *testing.T) {
		manifests, err := GetAllAppManifests()
		require.NoError(t, err)

		argocdDirect, err := GetArgoCDAppManifest()
		require.NoError(t, err)

		assert.Equal(t, argocdDirect, manifests["argocd"])
	})

	t.Run("kyverno manifest matches GetKyvernoAppManifest", func(t *testing.T) {
		manifests, err := GetAllAppManifests()
		require.NoError(t, err)

		kyvernoDirect, err := GetKyvernoAppManifest()
		require.NoError(t, err)

		assert.Equal(t, kyvernoDirect, manifests["kyverno"])
	})

	t.Run("all manifests are valid YAML", func(t *testing.T) {
		manifests, err := GetAllAppManifests()
		require.NoError(t, err)

		for name, manifest := range manifests {
			var parsed map[string]interface{}
			err := yaml.Unmarshal(manifest, &parsed)
			assert.NoError(t, err, "Manifest %s should be valid YAML", name)
		}
	})
}

// TestEmbeddedManifests tests the embedded filesystem
func TestEmbeddedManifests(t *testing.T) {
	t.Run("embedded FS contains expected files", func(t *testing.T) {
		entries, err := EmbeddedManifests.ReadDir("manifests")
		require.NoError(t, err)

		fileNames := make([]string, 0, len(entries))
		for _, entry := range entries {
			fileNames = append(fileNames, entry.Name())
		}

		assert.Contains(t, fileNames, "argocd.yaml")
		assert.Contains(t, fileNames, "kyverno.yaml")
	})

	t.Run("manifests have non-zero size", func(t *testing.T) {
		entries, err := EmbeddedManifests.ReadDir("manifests")
		require.NoError(t, err)

		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".yaml") {
				info, err := entry.Info()
				require.NoError(t, err)
				assert.True(t, info.Size() > 0, "File %s should have non-zero size", entry.Name())
			}
		}
	})
}
