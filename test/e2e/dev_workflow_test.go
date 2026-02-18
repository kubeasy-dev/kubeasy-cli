//go:build kindintegration
// +build kindintegration

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/deployer"
	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const devTestSlug = "dev-e2e-test"

// TestDevWorkflow validates the full dev challenge lifecycle:
// scaffold → apply → validate → clean
func TestDevWorkflow(t *testing.T) {
	// Create a temporary working directory for the challenge scaffold
	workDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	err := os.Chdir(workDir)
	require.NoError(t, err)

	t.Run("scaffold", func(t *testing.T) {
		// Test slug generation
		slug := devutils.GenerateSlug("Dev E2E Test")
		assert.Equal(t, devTestSlug, slug)

		// Create challenge directory structure (simulates dev create)
		challengeDir := filepath.Join(workDir, slug)
		dirs := []string{
			challengeDir,
			filepath.Join(challengeDir, "manifests"),
			filepath.Join(challengeDir, "policies"),
		}
		for _, dir := range dirs {
			err := os.MkdirAll(dir, 0755)
			require.NoError(t, err)
		}

		// Write challenge.yaml with a condition validation
		challengeYAML := `title: "Dev E2E Test"
description: "E2E test for dev workflow"
theme: "pods-containers"
type: "build"
difficulty: "easy"
estimatedTime: 10
initialSituation: ""
objective: "Deploy a configmap"
ofTheWeek: false
starterFriendly: false
objectives:
  - key: "configmap-exists"
    title: "ConfigMap exists"
    description: "The test-config ConfigMap should exist"
    order: 1
    type: status
    spec:
      target:
        kind: ConfigMap
        name: test-config
      checks:
        - field: data.key
          operator: "=="
          value: "e2e-value"
`
		err := os.WriteFile(filepath.Join(challengeDir, "challenge.yaml"), []byte(challengeYAML), 0600)
		require.NoError(t, err)

		// Write a ConfigMap manifest
		configMapManifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: e2e-value
`
		err = os.WriteFile(filepath.Join(challengeDir, "manifests", "configmap.yaml"), []byte(configMapManifest), 0600)
		require.NoError(t, err)

		// Verify structure
		assert.FileExists(t, filepath.Join(challengeDir, "challenge.yaml"))
		assert.FileExists(t, filepath.Join(challengeDir, "manifests", "configmap.yaml"))
		assert.DirExists(t, filepath.Join(challengeDir, "policies"))
	})

	t.Run("resolve-dir", func(t *testing.T) {
		// Test ResolveLocalChallengeDir finds our scaffolded challenge
		dir, err := devutils.ResolveLocalChallengeDir(devTestSlug, "")
		require.NoError(t, err)
		assert.Contains(t, dir, devTestSlug)
	})

	t.Run("apply", func(t *testing.T) {
		clientset, err := kube.GetKubernetesClient()
		require.NoError(t, err)

		dynamicClient, err := kube.GetDynamicClient()
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Create namespace
		err = kube.CreateNamespace(ctx, clientset, devTestSlug)
		require.NoError(t, err, "should create namespace for dev challenge")

		// Resolve and deploy
		challengeDir, err := devutils.ResolveLocalChallengeDir(devTestSlug, "")
		require.NoError(t, err)

		err = deployer.DeployLocalChallenge(ctx, clientset, dynamicClient, challengeDir, devTestSlug)
		require.NoError(t, err, "DeployLocalChallenge should succeed")

		// Verify the ConfigMap was created
		cm, err := clientset.CoreV1().ConfigMaps(devTestSlug).Get(ctx, "test-config", metav1.GetOptions{})
		require.NoError(t, err, "ConfigMap should exist after deploy")
		assert.Equal(t, "e2e-value", cm.Data["key"])
	})

	t.Run("validate", func(t *testing.T) {
		challengeDir, err := devutils.ResolveLocalChallengeDir(devTestSlug, "")
		require.NoError(t, err)

		// Load validations from local file
		config, err := validation.LoadFromFile(filepath.Join(challengeDir, "challenge.yaml"))
		require.NoError(t, err, "should load validations from challenge.yaml")
		require.NotEmpty(t, config.Validations, "should have at least one validation")

		// Get clients
		clientset, err := kube.GetKubernetesClient()
		require.NoError(t, err)

		dynamicClient, err := kube.GetDynamicClient()
		require.NoError(t, err)

		restConfig, err := kube.GetRestConfig()
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()

		// Execute validations
		executor := validation.NewExecutor(clientset, dynamicClient, restConfig, devTestSlug)
		results := executor.ExecuteAll(ctx, config.Validations)

		require.Len(t, results, len(config.Validations))

		// Display results
		allPassed := devutils.DisplayValidationResults(config.Validations, results)

		for _, r := range results {
			t.Logf("  %s: passed=%v message=%q", r.Key, r.Passed, r.Message)
		}

		assert.True(t, allPassed, "all validations should pass since we deployed the ConfigMap")
	})

	t.Run("clean", func(t *testing.T) {
		clientset, err := kube.GetKubernetesClient()
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Verify namespace exists before cleanup
		_, err = clientset.CoreV1().Namespaces().Get(ctx, devTestSlug, metav1.GetOptions{})
		require.NoError(t, err, "namespace should exist before cleanup")

		// Clean up
		err = deployer.CleanupChallenge(ctx, clientset, devTestSlug)
		require.NoError(t, err, "CleanupChallenge should succeed")

		// Wait for namespace to disappear
		assert.Eventually(t, func() bool {
			_, err := clientset.CoreV1().Namespaces().Get(ctx, devTestSlug, metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		}, 2*time.Minute, 2*time.Second, "namespace should be deleted after cleanup")
	})
}
