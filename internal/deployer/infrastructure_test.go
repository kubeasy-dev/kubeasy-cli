package deployer

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

// --- cert-manager URL tests ---

func TestCertManagerURLs(t *testing.T) {
	t.Run("CRDs URL contains version", func(t *testing.T) {
		url := certManagerCRDsURL()
		assert.Contains(t, url, CertManagerVersion, "cert-manager CRDs URL should contain the configured version")
	})

	t.Run("CRDs URL points to correct host", func(t *testing.T) {
		url := certManagerCRDsURL()
		assert.Contains(t, url, "github.com/cert-manager", "cert-manager CRDs URL should point to github.com/cert-manager")
	})

	t.Run("CRDs URL points to correct file", func(t *testing.T) {
		url := certManagerCRDsURL()
		assert.Contains(t, url, "cert-manager.crds.yaml", "cert-manager CRDs URL should point to cert-manager.crds.yaml")
	})

	t.Run("install URL contains version", func(t *testing.T) {
		url := certManagerInstallURL()
		assert.Contains(t, url, CertManagerVersion, "cert-manager install URL should contain the configured version")
	})

	t.Run("install URL points to correct host", func(t *testing.T) {
		url := certManagerInstallURL()
		assert.Contains(t, url, "github.com/cert-manager", "cert-manager install URL should point to github.com/cert-manager")
	})

	t.Run("install URL points to correct file", func(t *testing.T) {
		url := certManagerInstallURL()
		assert.Contains(t, url, "cert-manager.yaml", "cert-manager install URL should point to cert-manager.yaml")
	})
}

// --- isCertManagerReadyWithClient tests ---

func TestIsCertManagerReady(t *testing.T) {
	tests := []struct {
		name      string
		objects   []runtime.Object
		wantReady bool
	}{
		{
			name:      "namespace missing",
			objects:   []runtime.Object{},
			wantReady: false,
		},
		{
			name: "webhook deployment missing",
			objects: []runtime.Object{
				makeNamespace(certManagerNamespace),
				makeDeployment(certManagerNamespace, "cert-manager", 1, true),
				makeDeployment(certManagerNamespace, "cert-manager-cainjector", 1, true),
			},
			wantReady: false,
		},
		{
			name: "webhook deployment not ready",
			objects: []runtime.Object{
				makeNamespace(certManagerNamespace),
				makeDeployment(certManagerNamespace, "cert-manager", 1, true),
				makeDeployment(certManagerNamespace, "cert-manager-cainjector", 1, true),
				makeDeployment(certManagerNamespace, "cert-manager-webhook", 1, false),
			},
			wantReady: false,
		},
		{
			name: "all deployments ready",
			objects: []runtime.Object{
				makeNamespace(certManagerNamespace),
				makeDeployment(certManagerNamespace, "cert-manager", 1, true),
				makeDeployment(certManagerNamespace, "cert-manager-cainjector", 1, true),
				makeDeployment(certManagerNamespace, "cert-manager-webhook", 1, true),
			},
			wantReady: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewClientset(tt.objects...)

			ready, err := isCertManagerReadyWithClient(context.Background(), clientset)
			require.NoError(t, err)
			assert.Equal(t, tt.wantReady, ready)
		})
	}
}

// --- waitForCertManagerWebhookEndpoints tests ---

func TestWaitForCertManagerWebhookEndpoints_AlreadyReady(t *testing.T) {
	// Endpoint has addresses already present — should return nil immediately.
	// corev1.Endpoints is deprecated in K8s v1.33+ in favour of EndpointSlice,
	// but waitForCertManagerWebhookEndpoints uses the legacy Endpoints API so tests
	// must use the same type to populate the fake clientset.
	ep := &corev1.Endpoints{ //nolint:staticcheck // matches production API
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager-webhook",
			Namespace: certManagerNamespace,
		},
		Subsets: []corev1.EndpointSubset{ //nolint:staticcheck
			{
				Addresses: []corev1.EndpointAddress{
					{IP: "10.0.0.1"},
				},
			},
		},
	}
	clientset := fake.NewClientset(ep)
	ctx := context.Background()
	err := waitForCertManagerWebhookEndpoints(ctx, clientset)
	assert.NoError(t, err, "should return nil when endpoint addresses are already present")
}

func TestWaitForCertManagerWebhookEndpoints_Timeout(t *testing.T) {
	// Endpoint exists but has no addresses — should return error after deadline.
	ep := &corev1.Endpoints{ //nolint:staticcheck // matches production API
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager-webhook",
			Namespace: certManagerNamespace,
		},
		Subsets: []corev1.EndpointSubset{}, //nolint:staticcheck
	}
	clientset := fake.NewClientset(ep)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := waitForCertManagerWebhookEndpoints(ctx, clientset)
	assert.Error(t, err, "should return error when context deadline exceeds before addresses appear")
}

// --- nginx-ingress URL and readiness tests ---

func TestNginxIngressURL(t *testing.T) {
	t.Run("URL contains version", func(t *testing.T) {
		url := nginxIngressKindManifestURL()
		assert.Contains(t, url, NginxIngressVersion)
	})
	t.Run("URL contains correct host", func(t *testing.T) {
		url := nginxIngressKindManifestURL()
		assert.Contains(t, url, "raw.githubusercontent.com/kubernetes/ingress-nginx")
	})
	t.Run("URL is HTTPS", func(t *testing.T) {
		url := nginxIngressKindManifestURL()
		assert.Contains(t, url, "https://")
	})
}

func TestGatewayAPIURL(t *testing.T) {
	t.Run("URL contains version", func(t *testing.T) {
		url := gatewayAPICRDsURL()
		assert.Contains(t, url, GatewayAPICRDsVersion)
	})
	t.Run("URL contains correct host", func(t *testing.T) {
		url := gatewayAPICRDsURL()
		assert.Contains(t, url, "github.com/kubernetes-sigs/gateway-api")
	})
	t.Run("URL is HTTPS", func(t *testing.T) {
		url := gatewayAPICRDsURL()
		assert.Contains(t, url, "https://")
	})
}

func TestIsNginxIngressReady(t *testing.T) {
	const ns = "ingress-nginx"
	const depName = "ingress-nginx-controller"

	t.Run("namespace missing returns false", func(t *testing.T) {
		clientset := fake.NewClientset()
		ready, err := isNginxIngressReadyWithClient(context.Background(), clientset)
		require.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("deployment not found returns false", func(t *testing.T) {
		clientset := fake.NewClientset(makeNamespace(ns))
		ready, err := isNginxIngressReadyWithClient(context.Background(), clientset)
		require.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("deployment not ready returns false", func(t *testing.T) {
		clientset := fake.NewClientset(
			makeNamespace(ns),
			makeDeployment(ns, depName, 1, false),
		)
		ready, err := isNginxIngressReadyWithClient(context.Background(), clientset)
		require.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("deployment ready returns true", func(t *testing.T) {
		clientset := fake.NewClientset(
			makeNamespace(ns),
			makeDeployment(ns, depName, 1, true),
		)
		ready, err := isNginxIngressReadyWithClient(context.Background(), clientset)
		require.NoError(t, err)
		assert.True(t, ready)
	})
}

func TestIsGatewayAPICRDsInstalled(t *testing.T) {
	t.Run("fake clientset returns false (not installed)", func(t *testing.T) {
		// fake.NewClientset() Discovery().ServerResourcesForGroupVersion returns not-found
		// which means Gateway API CRDs are not installed — this is the expected behavior.
		clientset := fake.NewClientset()
		installed, err := isGatewayAPICRDsInstalled(context.Background(), clientset)
		require.NoError(t, err)
		assert.False(t, installed)
	})
}

// --- kindConfigMatchesAt tests ---

func refCluster() *kindv1alpha4.Cluster {
	return &kindv1alpha4.Cluster{
		TypeMeta: kindv1alpha4.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "kind.x-k8s.io/v1alpha4",
		},
		Nodes: []kindv1alpha4.Node{
			{
				Role: kindv1alpha4.ControlPlaneRole,
				ExtraPortMappings: []kindv1alpha4.PortMapping{
					{ContainerPort: 80, HostPort: 8080, Protocol: kindv1alpha4.PortMappingProtocolTCP},
					{ContainerPort: 443, HostPort: 8443, Protocol: kindv1alpha4.PortMappingProtocolTCP},
				},
			},
		},
	}
}

func TestKindConfigMatches_FileNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.yaml")
	assert.False(t, kindConfigMatchesAt(refCluster(), path), "should return false when config file does not exist")
}

func TestKindConfigMatches_IdenticalConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "kind-config.yaml")
	ref := refCluster()
	require.NoError(t, writeKindConfigToPath(ref, path))
	assert.True(t, kindConfigMatchesAt(ref, path), "should return true when installed config matches reference")
}

func TestKindConfigMatches_DifferentConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "kind-config.yaml")

	// Write a config with different port mappings (simulates old/outdated cluster).
	old := &kindv1alpha4.Cluster{
		TypeMeta: kindv1alpha4.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "kind.x-k8s.io/v1alpha4",
		},
		Nodes: []kindv1alpha4.Node{
			{Role: kindv1alpha4.ControlPlaneRole},
		},
	}
	require.NoError(t, writeKindConfigToPath(old, path))
	assert.False(t, kindConfigMatchesAt(refCluster(), path), "should return false when installed config differs from reference")
}

// --- installKyverno idempotency tests ---

func TestInstallKyverno_AlreadyReady(t *testing.T) {
	// When all Kyverno deployments are already ready, installKyverno should
	// return StatusReady without calling FetchManifest (idempotency path).
	clientset := fake.NewClientset(
		makeNamespace(kyvernoNamespace),
		makeDeployment(kyvernoNamespace, "kyverno-admission-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-background-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-cleanup-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-reports-controller", 1, true),
	)

	result := installKyverno(context.Background(), clientset, nil, nil)
	assert.Equal(t, StatusReady, result.Status, "installKyverno should return StatusReady when already ready")
	assert.Equal(t, "kyverno", result.Name)
}

func TestInstallLocalPathProvisioner_AlreadyReady(t *testing.T) {
	// When local-path-provisioner deployment is already ready, installLocalPathProvisioner
	// should return StatusReady without calling FetchManifest (idempotency path).
	clientset := fake.NewClientset(
		makeNamespace(localPathStorageNamespace),
		makeDeployment(localPathStorageNamespace, "local-path-provisioner", 1, true),
	)

	result := installLocalPathProvisioner(context.Background(), clientset, nil, nil)
	assert.Equal(t, StatusReady, result.Status, "installLocalPathProvisioner should return StatusReady when already ready")
	assert.Equal(t, "local-path-provisioner", result.Name)
}

func TestIsKyvernoReady_AllDeploymentsReady(t *testing.T) {
	clientset := fake.NewClientset(
		makeNamespace(kyvernoNamespace),
		makeDeployment(kyvernoNamespace, "kyverno-admission-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-background-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-cleanup-controller", 1, true),
		makeDeployment(kyvernoNamespace, "kyverno-reports-controller", 1, true),
	)

	ready, err := isKyvernoReadyWithClient(context.Background(), clientset)
	require.NoError(t, err)
	assert.True(t, ready)
}

func TestIsKyvernoReady_NamespaceMissing(t *testing.T) {
	clientset := fake.NewClientset()

	ready, err := isKyvernoReadyWithClient(context.Background(), clientset)
	require.NoError(t, err)
	assert.False(t, ready)
}

func TestIsLocalPathProvisionerReady_Ready(t *testing.T) {
	clientset := fake.NewClientset(
		makeNamespace(localPathStorageNamespace),
		makeDeployment(localPathStorageNamespace, "local-path-provisioner", 1, true),
	)

	ready, err := isLocalPathProvisionerReadyWithClient(context.Background(), clientset)
	require.NoError(t, err)
	assert.True(t, ready)
}

func TestIsLocalPathProvisionerReady_NamespaceMissing(t *testing.T) {
	clientset := fake.NewClientset()

	ready, err := isLocalPathProvisionerReadyWithClient(context.Background(), clientset)
	require.NoError(t, err)
	assert.False(t, ready)
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
	assert.True(t, kindConfigMatchesAt(cfg, path), "round-trip: write then read should match reference")
}
