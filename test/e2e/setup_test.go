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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kind/pkg/cluster"
)

// clusterPreExisted tracks whether the Kind cluster existed before the test run.
// If true, we skip deletion on teardown (useful for local development).
var clusterPreExisted bool

func TestMain(m *testing.M) {
	provider := cluster.NewProvider()

	// Check if cluster already exists
	clusters, err := provider.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list Kind clusters: %v\n", err)
		os.Exit(1)
	}
	for _, c := range clusters {
		if c == "kubeasy" {
			clusterPreExisted = true
			break
		}
	}

	// Create cluster if it doesn't exist
	if !clusterPreExisted {
		fmt.Println("Creating Kind cluster 'kubeasy'...")
		if err := provider.Create(
			"kubeasy",
			cluster.CreateWithNodeImage(constants.KindNodeImage),
		); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Kind cluster: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Kind cluster 'kubeasy' created successfully.")
	} else {
		fmt.Println("Kind cluster 'kubeasy' already exists, reusing it.")
	}

	// Run tests
	code := m.Run()

	// Teardown: only delete if we created the cluster
	if !clusterPreExisted {
		fmt.Println("Deleting Kind cluster 'kubeasy'...")
		if err := provider.Delete("kubeasy", ""); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete Kind cluster: %v\n", err)
		}
	}

	os.Exit(code)
}

func TestKindClusterReachable(t *testing.T) {
	clientset, err := kube.GetKubernetesClient()
	require.NoError(t, err, "should get a Kubernetes client for kind-kubeasy context")

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

func TestInfrastructureNotReadyBeforeSetup(t *testing.T) {
	// On a fresh cluster (CI), infrastructure should not be ready yet.
	// On a reused cluster (local dev), it may already be ready — skip in that case.
	ready, err := deployer.IsInfrastructureReady()
	require.NoError(t, err, "IsInfrastructureReady should not error")

	if clusterPreExisted && ready {
		t.Skip("cluster was pre-existing with infrastructure already installed")
	}

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
