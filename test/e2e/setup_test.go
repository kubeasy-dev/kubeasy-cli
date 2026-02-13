//go:build kindintegration

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/deployer"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kind/pkg/cluster"
)

const e2eClusterName = "kubeasy-e2e"

func TestMain(m *testing.M) {
	// Override the cluster context so all internal packages target the e2e cluster
	constants.KubeasyClusterContext = "kind-" + e2eClusterName

	provider := cluster.NewProvider()

	// Always create a fresh cluster for the test run
	fmt.Printf("Creating Kind cluster '%s'...\n", e2eClusterName)
	if err := provider.Create(
		e2eClusterName,
		cluster.CreateWithNodeImage(constants.KindNodeImage),
	); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create Kind cluster: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Kind cluster '%s' created successfully.\n", e2eClusterName)

	// Run tests
	code := m.Run()

	// Always clean up after ourselves
	fmt.Printf("Deleting Kind cluster '%s'...\n", e2eClusterName)
	if err := provider.Delete(e2eClusterName, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to delete Kind cluster: %v\n", err)
	}

	os.Exit(code)
}

// =============================================================================
// Cluster connectivity
// =============================================================================

func TestKindClusterReachable(t *testing.T) {
	clientset, err := kube.GetKubernetesClient()
	require.NoError(t, err, "should get a Kubernetes client for %s context", constants.KubeasyClusterContext)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	require.NoError(t, err, "should be able to list namespaces")
	assert.NotEmpty(t, namespaces.Items, "cluster should have at least one namespace")
}

func TestKubernetesVersion(t *testing.T) {
	serverVersion, err := kube.GetServerVersion()
	require.NoError(t, err, "should get server version")

	expectedVersion := constants.GetKubernetesVersion()
	assert.True(t,
		constants.VersionsCompatible(serverVersion, expectedVersion),
		"server version %s should be compatible with expected %s", serverVersion, expectedVersion,
	)
}

// =============================================================================
// Infrastructure setup
// =============================================================================

func TestInfrastructureNotReadyBeforeSetup(t *testing.T) {
	ready, err := deployer.IsInfrastructureReady()
	require.NoError(t, err, "IsInfrastructureReady should not error")
	assert.False(t, ready, "infrastructure should not be ready on a fresh cluster")
}

func TestSetupInfrastructure(t *testing.T) {
	err := deployer.SetupInfrastructure()
	require.NoError(t, err, "SetupInfrastructure should succeed")

	clientset, err := kube.GetKubernetesClient()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Verify kyverno namespace and deployments
	_, err = clientset.CoreV1().Namespaces().Get(ctx, "kyverno", metav1.GetOptions{})
	require.NoError(t, err, "kyverno namespace should exist")

	kyvernoDeployments := []string{
		"kyverno-admission-controller",
		"kyverno-background-controller",
		"kyverno-cleanup-controller",
		"kyverno-reports-controller",
	}
	for _, name := range kyvernoDeployments {
		dep, err := clientset.AppsV1().Deployments("kyverno").Get(ctx, name, metav1.GetOptions{})
		require.NoError(t, err, "deployment %s should exist", name)
		assert.Greater(t, dep.Status.ReadyReplicas, int32(0),
			"deployment %s should have ready replicas", name)
		assert.Equal(t, dep.Status.Replicas, dep.Status.ReadyReplicas,
			"deployment %s should have all replicas ready", name)
	}

	// Verify local-path-storage namespace and deployment
	_, err = clientset.CoreV1().Namespaces().Get(ctx, "local-path-storage", metav1.GetOptions{})
	require.NoError(t, err, "local-path-storage namespace should exist")

	dep, err := clientset.AppsV1().Deployments("local-path-storage").Get(ctx, "local-path-provisioner", metav1.GetOptions{})
	require.NoError(t, err, "local-path-provisioner deployment should exist")
	assert.Greater(t, dep.Status.ReadyReplicas, int32(0),
		"local-path-provisioner should have ready replicas")
	assert.Equal(t, dep.Status.Replicas, dep.Status.ReadyReplicas,
		"local-path-provisioner should have all replicas ready")
}

func TestIsInfrastructureReadyAfterSetup(t *testing.T) {
	ready, err := deployer.IsInfrastructureReady()
	require.NoError(t, err, "IsInfrastructureReady should not error")
	assert.True(t, ready, "infrastructure should be ready after setup")
}

func TestSetupIdempotency(t *testing.T) {
	err := deployer.SetupInfrastructure()
	require.NoError(t, err, "SetupInfrastructure should be idempotent")

	ready, err := deployer.IsInfrastructureReady()
	require.NoError(t, err)
	assert.True(t, ready, "infrastructure should still be ready after re-running setup")
}

// =============================================================================
// Challenge deploy & cleanup
// =============================================================================

const testChallengeSlug = "pod-evicted"

func TestDeployChallenge(t *testing.T) {
	clientset, err := kube.GetKubernetesClient()
	require.NoError(t, err)

	dynamicClient, err := kube.GetDynamicClient()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create the challenge namespace (same as cmd/start.go flow)
	err = kube.CreateNamespace(ctx, clientset, testChallengeSlug)
	require.NoError(t, err, "should create challenge namespace")

	// Deploy the challenge from OCI registry
	err = deployer.DeployChallenge(ctx, clientset, dynamicClient, testChallengeSlug)
	require.NoError(t, err, "DeployChallenge should succeed")

	// Verify namespace exists
	ns, err := clientset.CoreV1().Namespaces().Get(ctx, testChallengeSlug, metav1.GetOptions{})
	require.NoError(t, err, "challenge namespace should exist")
	assert.Equal(t, testChallengeSlug, ns.Name)

	// Verify at least one resource was created in the namespace
	pods, err := clientset.CoreV1().Pods(testChallengeSlug).List(ctx, metav1.ListOptions{})
	require.NoError(t, err, "should list pods in challenge namespace")

	deployments, err := clientset.AppsV1().Deployments(testChallengeSlug).List(ctx, metav1.ListOptions{})
	require.NoError(t, err, "should list deployments in challenge namespace")

	assert.True(t, len(pods.Items) > 0 || len(deployments.Items) > 0,
		"challenge should have created at least one pod or deployment")
}

func TestCleanupChallenge(t *testing.T) {
	clientset, err := kube.GetKubernetesClient()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Ensure the namespace exists before cleanup (from previous test)
	_, err = clientset.CoreV1().Namespaces().Get(ctx, testChallengeSlug, metav1.GetOptions{})
	require.NoError(t, err, "challenge namespace should exist before cleanup")

	// Run cleanup
	err = deployer.CleanupChallenge(ctx, clientset, testChallengeSlug)
	require.NoError(t, err, "CleanupChallenge should succeed")

	// Wait for namespace to actually disappear (deletion is async)
	assert.Eventually(t, func() bool {
		_, err := clientset.CoreV1().Namespaces().Get(ctx, testChallengeSlug, metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 2*time.Minute, 2*time.Second, "challenge namespace should be deleted after cleanup")
}
