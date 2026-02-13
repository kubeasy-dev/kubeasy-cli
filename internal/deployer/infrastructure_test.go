package deployer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func makeDeployment(namespace, name string, replicas int32, ready bool) *appsv1.Deployment {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			Replicas: replicas,
		},
	}
	if ready {
		dep.Status.ReadyReplicas = replicas
	}
	return dep
}

func makeNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func TestIsInfrastructureReady_AllReady(t *testing.T) {
	clientset := fake.NewClientset(
		makeNamespace(kyvernoNamespace),
		makeNamespace(localPathStorageNamespace),
		makeDeployment(kyvernoNamespace, "kyverno-admission-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-background-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-cleanup-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-reports-controller", 1, true),
		makeDeployment(localPathStorageNamespace, "local-path-provisioner", 1, true),
	)

	ready, err := IsInfrastructureReadyWithClient(context.Background(), clientset)
	require.NoError(t, err)
	assert.True(t, ready)
}

func TestIsInfrastructureReady_NoNamespace(t *testing.T) {
	clientset := fake.NewClientset()

	ready, err := IsInfrastructureReadyWithClient(context.Background(), clientset)
	require.NoError(t, err)
	assert.False(t, ready)
}

func TestIsInfrastructureReady_KyvernoNamespaceOnly(t *testing.T) {
	clientset := fake.NewClientset(
		makeNamespace(kyvernoNamespace),
	)

	ready, err := IsInfrastructureReadyWithClient(context.Background(), clientset)
	require.NoError(t, err)
	assert.False(t, ready, "should be false when kyverno deployments are missing")
}

func TestIsInfrastructureReady_DeploymentsNotReady(t *testing.T) {
	clientset := fake.NewClientset(
		makeNamespace(kyvernoNamespace),
		makeNamespace(localPathStorageNamespace),
		makeDeployment(kyvernoNamespace, "kyverno-admission-controller", 1, false),
		makeDeployment(kyvernoNamespace, "kyverno-background-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-cleanup-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-reports-controller", 1, true),
		makeDeployment(localPathStorageNamespace, "local-path-provisioner", 1, true),
	)

	ready, err := IsInfrastructureReadyWithClient(context.Background(), clientset)
	require.NoError(t, err)
	assert.False(t, ready, "should be false when kyverno-admission-controller is not ready")
}

func TestIsInfrastructureReady_PartiallyReady(t *testing.T) {
	// Kyverno ready but local-path-provisioner not
	clientset := fake.NewClientset(
		makeNamespace(kyvernoNamespace),
		makeNamespace(localPathStorageNamespace),
		makeDeployment(kyvernoNamespace, "kyverno-admission-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-background-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-cleanup-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-reports-controller", 1, true),
		makeDeployment(localPathStorageNamespace, "local-path-provisioner", 1, false),
	)

	ready, err := IsInfrastructureReadyWithClient(context.Background(), clientset)
	require.NoError(t, err)
	assert.False(t, ready, "should be false when local-path-provisioner is not ready")
}

func TestIsInfrastructureReady_LocalPathNamespaceMissing(t *testing.T) {
	// Kyverno namespace and deployments ready, but local-path-storage namespace missing
	clientset := fake.NewClientset(
		makeNamespace(kyvernoNamespace),
		makeDeployment(kyvernoNamespace, "kyverno-admission-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-background-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-cleanup-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-reports-controller", 1, true),
	)

	ready, err := IsInfrastructureReadyWithClient(context.Background(), clientset)
	require.NoError(t, err)
	assert.False(t, ready, "should be false when local-path-storage namespace is missing")
}

func TestIsInfrastructureReady_MultipleReplicas(t *testing.T) {
	clientset := fake.NewClientset(
		makeNamespace(kyvernoNamespace),
		makeNamespace(localPathStorageNamespace),
		makeDeployment(kyvernoNamespace, "kyverno-admission-controller", 3, true),
		makeDeployment(kyvernoNamespace, "kyverno-background-controller", 2, true),
		makeDeployment(kyvernoNamespace, "kyverno-cleanup-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-reports-controller", 1, true),
		makeDeployment(localPathStorageNamespace, "local-path-provisioner", 2, true),
	)

	ready, err := IsInfrastructureReadyWithClient(context.Background(), clientset)
	require.NoError(t, err)
	assert.True(t, ready, "should be true when all deployments have all replicas ready")
}

func TestIsInfrastructureReady_MultipleReplicasPartiallyReady(t *testing.T) {
	// 3 desired but only 1 ready
	dep := makeDeployment(kyvernoNamespace, "kyverno-admission-controller", 3, false)
	dep.Status.ReadyReplicas = 1

	clientset := fake.NewClientset(
		makeNamespace(kyvernoNamespace),
		makeNamespace(localPathStorageNamespace),
		dep,
		makeDeployment(kyvernoNamespace, "kyverno-background-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-cleanup-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-reports-controller", 1, true),
		makeDeployment(localPathStorageNamespace, "local-path-provisioner", 1, true),
	)

	ready, err := IsInfrastructureReadyWithClient(context.Background(), clientset)
	require.NoError(t, err)
	assert.False(t, ready, "should be false when ReadyReplicas < Replicas")
}

func TestInfrastructureURLs(t *testing.T) {
	t.Run("kyverno URL is HTTPS", func(t *testing.T) {
		url := kyvernoInstallURL()
		assert.Contains(t, url, "https://")
		assert.Contains(t, url, "kyverno/kyverno")
		assert.Contains(t, url, "install.yaml")
	})

	t.Run("local-path-provisioner URL is HTTPS", func(t *testing.T) {
		url := localPathProvisionerInstallURL()
		assert.Contains(t, url, "https://")
		assert.Contains(t, url, "rancher/local-path-provisioner")
		assert.Contains(t, url, "local-path-storage.yaml")
	})

	t.Run("kyverno URL embeds version", func(t *testing.T) {
		url := kyvernoInstallURL()
		assert.Contains(t, url, KyvernoVersion,
			"kyverno install URL should contain the configured version")
	})

	t.Run("local-path-provisioner URL embeds version", func(t *testing.T) {
		url := localPathProvisionerInstallURL()
		assert.Contains(t, url, LocalPathProvisionerVersion,
			"local-path-provisioner install URL should contain the configured version")
	})
}
