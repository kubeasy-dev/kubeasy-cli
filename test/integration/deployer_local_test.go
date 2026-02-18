//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/deployer"
	"github.com/kubeasy-dev/kubeasy-cli/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestDeployLocalChallenge_AppliesManifests verifies that DeployLocalChallenge
// reads YAML files from manifests/ and applies them to the cluster.
func TestDeployLocalChallenge_AppliesManifests(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	// Create a temp challenge directory with a ConfigMap manifest
	challengeDir := t.TempDir()
	manifestsDir := filepath.Join(challengeDir, "manifests")
	err := os.MkdirAll(manifestsDir, 0755)
	require.NoError(t, err)

	configMapYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`
	err = os.WriteFile(filepath.Join(manifestsDir, "configmap.yaml"), []byte(configMapYAML), 0600)
	require.NoError(t, err)

	// Deploy
	err = deployer.DeployLocalChallenge(ctx, env.Clientset, env.DynamicClient, challengeDir, env.Namespace)
	require.NoError(t, err)

	// Verify the ConfigMap was created
	cm, err := env.Clientset.CoreV1().ConfigMaps(env.Namespace).Get(ctx, "test-config", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "value", cm.Data["key"])
}

// TestDeployLocalChallenge_AppliesPolicies verifies that DeployLocalChallenge
// also applies YAML files from the policies/ directory.
func TestDeployLocalChallenge_AppliesPolicies(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	challengeDir := t.TempDir()

	// Create manifests dir (empty) and policies dir with a ConfigMap
	err := os.MkdirAll(filepath.Join(challengeDir, "manifests"), 0755)
	require.NoError(t, err)

	policiesDir := filepath.Join(challengeDir, "policies")
	err = os.MkdirAll(policiesDir, 0755)
	require.NoError(t, err)

	configMapYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: policy-config
data:
  policy: enabled
`
	err = os.WriteFile(filepath.Join(policiesDir, "policy.yaml"), []byte(configMapYAML), 0600)
	require.NoError(t, err)

	err = deployer.DeployLocalChallenge(ctx, env.Clientset, env.DynamicClient, challengeDir, env.Namespace)
	require.NoError(t, err)

	cm, err := env.Clientset.CoreV1().ConfigMaps(env.Namespace).Get(ctx, "policy-config", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "enabled", cm.Data["policy"])
}

// TestDeployLocalChallenge_SkipsMissingDirs verifies that missing manifests/
// or policies/ directories are silently skipped.
func TestDeployLocalChallenge_SkipsMissingDirs(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	// Create an empty challenge directory (no manifests/ or policies/)
	challengeDir := t.TempDir()

	err := deployer.DeployLocalChallenge(ctx, env.Clientset, env.DynamicClient, challengeDir, env.Namespace)
	assert.NoError(t, err, "should succeed even without manifests/ or policies/ dirs")
}

// TestDeployLocalChallenge_IgnoresNonYAMLFiles verifies that non-.yaml files
// in manifests/ are ignored.
func TestDeployLocalChallenge_IgnoresNonYAMLFiles(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	challengeDir := t.TempDir()
	manifestsDir := filepath.Join(challengeDir, "manifests")
	err := os.MkdirAll(manifestsDir, 0755)
	require.NoError(t, err)

	// Create a non-yaml file (should be ignored)
	err = os.WriteFile(filepath.Join(manifestsDir, ".gitkeep"), []byte{}, 0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(manifestsDir, "README.md"), []byte("# Readme"), 0600)
	require.NoError(t, err)

	// Also create a valid YAML file
	configMapYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: only-yaml
data:
  key: value
`
	err = os.WriteFile(filepath.Join(manifestsDir, "config.yaml"), []byte(configMapYAML), 0600)
	require.NoError(t, err)

	err = deployer.DeployLocalChallenge(ctx, env.Clientset, env.DynamicClient, challengeDir, env.Namespace)
	require.NoError(t, err)

	// Verify only the YAML file was applied
	cm, err := env.Clientset.CoreV1().ConfigMaps(env.Namespace).Get(ctx, "only-yaml", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "value", cm.Data["key"])
}

// TestDeployLocalChallenge_MultipleManifests verifies that all YAML files in
// manifests/ are applied, including in subdirectories.
func TestDeployLocalChallenge_MultipleManifests(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	challengeDir := t.TempDir()
	manifestsDir := filepath.Join(challengeDir, "manifests")
	subDir := filepath.Join(manifestsDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	cm1 := `apiVersion: v1
kind: ConfigMap
metadata:
  name: config-one
data:
  key: one
`
	cm2 := `apiVersion: v1
kind: ConfigMap
metadata:
  name: config-two
data:
  key: two
`
	err = os.WriteFile(filepath.Join(manifestsDir, "cm1.yaml"), []byte(cm1), 0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "cm2.yaml"), []byte(cm2), 0600)
	require.NoError(t, err)

	err = deployer.DeployLocalChallenge(ctx, env.Clientset, env.DynamicClient, challengeDir, env.Namespace)
	require.NoError(t, err)

	// Verify both ConfigMaps exist
	cm, err := env.Clientset.CoreV1().ConfigMaps(env.Namespace).Get(ctx, "config-one", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "one", cm.Data["key"])

	cm, err = env.Clientset.CoreV1().ConfigMaps(env.Namespace).Get(ctx, "config-two", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "two", cm.Data["key"])
}

// TestDeployLocalChallenge_InvalidManifest verifies that an invalid YAML file
// causes an error.
func TestDeployLocalChallenge_InvalidManifest(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	challengeDir := t.TempDir()
	manifestsDir := filepath.Join(challengeDir, "manifests")
	err := os.MkdirAll(manifestsDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(manifestsDir, "bad.yaml"), []byte("not: valid: kubernetes: manifest"), 0600)
	require.NoError(t, err)

	err = deployer.DeployLocalChallenge(ctx, env.Clientset, env.DynamicClient, challengeDir, env.Namespace)
	assert.Error(t, err, "should fail on invalid manifest")
	assert.Contains(t, err.Error(), "failed to apply manifest")
}
