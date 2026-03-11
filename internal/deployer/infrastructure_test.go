package deployer

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	kindv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
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

// --- ComponentResult tests ---

func TestComponentResult_StatusReady(t *testing.T) {
	r := ComponentResult{Name: "test", Status: StatusReady, Message: "ok"}
	assert.Equal(t, ComponentStatus("ready"), r.Status)
}

func TestComponentResult_StatusNotReady(t *testing.T) {
	r := ComponentResult{Name: "test", Status: StatusNotReady, Message: "not ready"}
	assert.Equal(t, ComponentStatus("not-ready"), r.Status)
}

func TestComponentResult_StatusMissing(t *testing.T) {
	r := ComponentResult{Name: "test", Status: StatusMissing, Message: "missing"}
	assert.Equal(t, ComponentStatus("missing"), r.Status)
}

// --- notReady helper tests ---

func TestNotReady(t *testing.T) {
	err := errors.New("boom")
	r := notReady("foo", err)
	assert.Equal(t, "foo", r.Name)
	assert.Equal(t, StatusNotReady, r.Status)
	assert.Equal(t, "boom", r.Message)
}

// --- hasExtraPortMappingsAt tests ---

func TestHasExtraPortMappings_FileNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.yaml")
	result := hasExtraPortMappingsAt(path)
	assert.False(t, result, "should return false when config file does not exist")
}

func TestHasExtraPortMappings_WithCorrectPorts(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "kind-config.yaml")

	cfg := &kindv1alpha4.Cluster{
		TypeMeta: kindv1alpha4.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "kind.x-k8s.io/v1alpha4",
		},
		Nodes: []kindv1alpha4.Node{
			{
				Role: kindv1alpha4.ControlPlaneRole,
				ExtraPortMappings: []kindv1alpha4.PortMapping{
					{ContainerPort: 80, HostPort: 8080},
					{ContainerPort: 443, HostPort: 8443},
				},
			},
		},
	}
	require.NoError(t, writeKindConfigToPath(cfg, path))
	assert.True(t, hasExtraPortMappingsAt(path), "should return true when 8080 and 8443 are mapped")
}

func TestHasExtraPortMappings_WithDifferentPorts(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "kind-config.yaml")

	cfg := &kindv1alpha4.Cluster{
		TypeMeta: kindv1alpha4.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "kind.x-k8s.io/v1alpha4",
		},
		Nodes: []kindv1alpha4.Node{
			{
				Role: kindv1alpha4.ControlPlaneRole,
				ExtraPortMappings: []kindv1alpha4.PortMapping{
					{ContainerPort: 80, HostPort: 9080},
					{ContainerPort: 443, HostPort: 9443},
				},
			},
		},
	}
	require.NoError(t, writeKindConfigToPath(cfg, path))
	assert.False(t, hasExtraPortMappingsAt(path), "should return false when ports are different from 8080/8443")
}

// --- writeKindConfig round-trip test ---

func TestWriteKindConfig_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "kind-config.yaml")

	cfg := &kindv1alpha4.Cluster{
		TypeMeta: kindv1alpha4.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "kind.x-k8s.io/v1alpha4",
		},
		Nodes: []kindv1alpha4.Node{
			{
				Role: kindv1alpha4.ControlPlaneRole,
				ExtraPortMappings: []kindv1alpha4.PortMapping{
					{ContainerPort: 80, HostPort: 8080},
					{ContainerPort: 443, HostPort: 8443},
				},
			},
		},
	}

	require.NoError(t, writeKindConfigToPath(cfg, path))
	assert.True(t, hasExtraPortMappingsAt(path), "round-trip: write then read should return true")
}
