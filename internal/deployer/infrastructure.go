package deployer

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	kindv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	sigsyaml "sigs.k8s.io/yaml"
)

// ComponentStatus represents the readiness status of a single infrastructure component.
type ComponentStatus string

const (
	// StatusReady indicates the component is installed and ready.
	StatusReady ComponentStatus = "ready"
	// StatusNotReady indicates the component exists but is not yet ready.
	StatusNotReady ComponentStatus = "not-ready"
	// StatusMissing indicates the component is not installed.
	StatusMissing ComponentStatus = "missing"
)

// ComponentResult holds the name, status, and optional message for an infrastructure component check.
type ComponentResult struct {
	Name    string
	Status  ComponentStatus
	Message string
}

// notReady returns a ComponentResult with StatusNotReady. Used by all installers
// to reduce repetition when returning errors. Defined here (Wave 1) so it is
// available to both Wave 2 plans (02 and 03) without conflict.
func notReady(name string, err error) ComponentResult {
	return ComponentResult{Name: name, Status: StatusNotReady, Message: err.Error()}
}

// writeKindConfig marshals the Kind cluster config to YAML and writes it to GetKindConfigPath().
// Creates the ~/.kubeasy directory if it does not exist.
// Use WriteKindConfig (exported) from setup.go as the canonical call site.
func writeKindConfig(cfg *kindv1alpha4.Cluster) error { //nolint:unused // internal; WriteKindConfig is the exported canonical path
	return writeKindConfigToPath(cfg, constants.GetKindConfigPath())
}

// writeKindConfigToPath marshals the Kind cluster config to YAML and writes it to the given path.
// Creates the parent directory if it does not exist. This testable variant accepts an explicit path.
func writeKindConfigToPath(cfg *kindv1alpha4.Cluster, path string) error {
	data, err := sigsyaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal kind config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("failed to create kubeasy config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write kind config: %w", err)
	}
	return nil
}

// kindConfigMatchesAt reports whether the installed kind-config.yaml at path has identical
// content to the YAML serialisation of ref. Returns false when the file is missing,
// unreadable, or its content differs — all are treated as "config has drifted".
func kindConfigMatchesAt(ref *kindv1alpha4.Cluster, path string) bool {
	installed, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	want, err := sigsyaml.Marshal(ref)
	if err != nil {
		return false
	}
	return bytes.Equal(installed, want)
}

const (
	kyvernoNamespace             = "kyverno"
	localPathStorageNamespace    = "local-path-storage"
	certManagerNamespace         = "cert-manager"
	defaultInfrastructureTimeout = 10 * time.Minute
)

// kyvernoInstallURL returns the URL for the Kyverno install manifest.
func kyvernoInstallURL() string {
	return fmt.Sprintf("https://github.com/kyverno/kyverno/releases/download/%s/install.yaml", KyvernoVersion)
}

// localPathProvisionerInstallURL returns the URL for the local-path-provisioner install manifest.
func localPathProvisionerInstallURL() string {
	return fmt.Sprintf("https://raw.githubusercontent.com/rancher/local-path-provisioner/%s/deploy/local-path-storage.yaml", LocalPathProvisionerVersion)
}

// isKyvernoReadyWithClient checks whether Kyverno is installed and all four deployments are ready.
// Extracted from IsInfrastructureReadyWithClient for per-component use.
func isKyvernoReadyWithClient(ctx context.Context, clientset kubernetes.Interface) (bool, error) {
	_, err := clientset.CoreV1().Namespaces().Get(ctx, kyvernoNamespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("Kyverno namespace does not exist")
			return false, nil
		}
		return false, fmt.Errorf("error checking kyverno namespace: %w", err)
	}

	kyvernoDeployments := []string{
		"kyverno-admission-controller",
		"kyverno-background-controller",
		"kyverno-cleanup-controller",
		"kyverno-reports-controller",
	}
	for _, name := range kyvernoDeployments {
		dep, err := clientset.AppsV1().Deployments(kyvernoNamespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				logger.Debug("Kyverno deployment '%s' not found", name)
				return false, nil
			}
			return false, fmt.Errorf("error checking kyverno deployment '%s': %w", name, err)
		}
		if dep.Status.ReadyReplicas == 0 || dep.Status.ReadyReplicas != dep.Status.Replicas {
			logger.Debug("Kyverno deployment '%s' not ready (Ready: %d/%d)", name, dep.Status.ReadyReplicas, dep.Status.Replicas)
			return false, nil
		}
	}
	return true, nil
}

// isLocalPathProvisionerReadyWithClient checks whether local-path-provisioner is installed and ready.
// Extracted from IsInfrastructureReadyWithClient for per-component use.
func isLocalPathProvisionerReadyWithClient(ctx context.Context, clientset kubernetes.Interface) (bool, error) {
	_, err := clientset.CoreV1().Namespaces().Get(ctx, localPathStorageNamespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("local-path-storage namespace does not exist")
			return false, nil
		}
		return false, fmt.Errorf("error checking local-path-storage namespace: %w", err)
	}

	dep, err := clientset.AppsV1().Deployments(localPathStorageNamespace).Get(ctx, "local-path-provisioner", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("local-path-provisioner deployment not found")
			return false, nil
		}
		return false, fmt.Errorf("error checking local-path-provisioner deployment: %w", err)
	}
	if dep.Status.ReadyReplicas == 0 || dep.Status.ReadyReplicas != dep.Status.Replicas {
		logger.Debug("local-path-provisioner not ready (Ready: %d/%d)", dep.Status.ReadyReplicas, dep.Status.Replicas)
		return false, nil
	}
	return true, nil
}

// installKyverno installs Kyverno into the cluster. If already ready, returns StatusReady immediately.
// Returns ComponentResult — never an error. Called by SetupAllComponents.
func installKyverno(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface, mapper meta.RESTMapper) ComponentResult {
	const name = "kyverno"

	ready, err := isKyvernoReadyWithClient(ctx, clientset)
	if err != nil {
		return notReady(name, err)
	}
	if ready {
		logger.Info("Kyverno is already installed and ready, skipping installation")
		return ComponentResult{Name: name, Status: StatusReady, Message: "already installed"}
	}

	logger.Info("Installing Kyverno %s...", KyvernoVersion)
	if err := kube.CreateNamespace(ctx, clientset, kyvernoNamespace); err != nil {
		return notReady(name, fmt.Errorf("failed to create kyverno namespace: %w", err))
	}

	kyvernoURL := kyvernoInstallURL()
	logger.Debug("Fetching Kyverno manifest from %s", kyvernoURL)
	kyvernoManifest, err := kube.FetchManifest(kyvernoURL)
	if err != nil {
		return notReady(name, fmt.Errorf("failed to download Kyverno manifest: %w", err))
	}
	logger.Debug("Kyverno manifest fetched (%d bytes)", len(kyvernoManifest))

	if err := kube.ApplyManifest(ctx, kyvernoManifest, kyvernoNamespace, mapper, dynamicClient); err != nil {
		return notReady(name, fmt.Errorf("failed to apply Kyverno manifest: %w", err))
	}
	logger.Info("Kyverno manifest applied.")

	kyvernoDeployments := []string{
		"kyverno-admission-controller",
		"kyverno-background-controller",
		"kyverno-cleanup-controller",
		"kyverno-reports-controller",
	}

	// kube.WaitForDeploymentsReady requires *kubernetes.Clientset.
	cs, ok := clientset.(*kubernetes.Clientset)
	if !ok {
		return notReady(name, fmt.Errorf("internal error: clientset is not *kubernetes.Clientset"))
	}
	if err := kube.WaitForDeploymentsReady(ctx, cs, kyvernoNamespace, kyvernoDeployments); err != nil {
		return notReady(name, fmt.Errorf("kyverno deployments failed to become ready: %w", err))
	}

	logger.Info("Kyverno installed and ready.")
	return ComponentResult{Name: name, Status: StatusReady, Message: "installed successfully"}
}

// installLocalPathProvisioner installs local-path-provisioner into the cluster. If already ready,
// returns StatusReady immediately. Returns ComponentResult — never an error. Called by SetupAllComponents.
func installLocalPathProvisioner(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface, mapper meta.RESTMapper) ComponentResult {
	const name = "local-path-provisioner"

	ready, err := isLocalPathProvisionerReadyWithClient(ctx, clientset)
	if err != nil {
		return notReady(name, err)
	}
	if ready {
		logger.Info("local-path-provisioner is already installed and ready, skipping installation")
		return ComponentResult{Name: name, Status: StatusReady, Message: "already installed"}
	}

	logger.Info("Installing local-path-provisioner %s...", LocalPathProvisionerVersion)
	if err := kube.CreateNamespace(ctx, clientset, localPathStorageNamespace); err != nil {
		return notReady(name, fmt.Errorf("failed to create local-path-storage namespace: %w", err))
	}

	localPathURL := localPathProvisionerInstallURL()
	logger.Debug("Fetching local-path-provisioner manifest from %s", localPathURL)
	localPathManifest, err := kube.FetchManifest(localPathURL)
	if err != nil {
		return notReady(name, fmt.Errorf("failed to download local-path-provisioner manifest: %w", err))
	}
	logger.Debug("local-path-provisioner manifest fetched (%d bytes)", len(localPathManifest))

	if err := kube.ApplyManifest(ctx, localPathManifest, localPathStorageNamespace, mapper, dynamicClient); err != nil {
		return notReady(name, fmt.Errorf("failed to apply local-path-provisioner manifest: %w", err))
	}
	logger.Info("local-path-provisioner manifest applied.")

	// kube.WaitForDeploymentsReady requires *kubernetes.Clientset.
	cs, ok := clientset.(*kubernetes.Clientset)
	if !ok {
		return notReady(name, fmt.Errorf("internal error: clientset is not *kubernetes.Clientset"))
	}
	if err := kube.WaitForDeploymentsReady(ctx, cs, localPathStorageNamespace, []string{"local-path-provisioner"}); err != nil {
		return notReady(name, fmt.Errorf("local-path-provisioner deployment failed to become ready: %w", err))
	}

	logger.Info("local-path-provisioner installed and ready.")
	return ComponentResult{Name: name, Status: StatusReady, Message: "installed successfully"}
}

// WriteKindConfig is an exported wrapper around writeKindConfig for use by setup.go.
// It writes the Kind cluster config to ~/.kubeasy/kind-config.yaml.
func WriteKindConfig(cfg *kindv1alpha4.Cluster) error {
	return writeKindConfigToPath(cfg, constants.GetKindConfigPath())
}

// KindConfigMatches reports whether the installed kind-config.yaml at GetKindConfigPath()
// matches ref. Returns false when the file is missing or its content differs from ref —
// both signal that the cluster should be recreated.
func KindConfigMatches(ref *kindv1alpha4.Cluster) bool {
	return kindConfigMatchesAt(ref, constants.GetKindConfigPath())
}

// clusterIssuerManifest is the ClusterIssuer that delegates to the kubeasy-ca Secret.
// Applied after installKubeasyCA creates the Secret so the cert-manager CA issuer can load it.
const clusterIssuerManifest = `apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: kubeasy-ca
spec:
  ca:
    secretName: kubeasy-ca
`

// installKubeasyCA generates a local CA, stores it in the well-known Secret
// (cert-manager/kubeasy-ca), and creates the matching ClusterIssuer so challenge
// manifests can request certificates signed by that CA.
// The function is idempotent: if the Secret already exists it returns StatusReady immediately.
func installKubeasyCA(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface) ComponentResult {
	const name = "kubeasy-ca"

	// Idempotency check — if the Secret already exists, nothing to do.
	_, err := clientset.CoreV1().Secrets(constants.KubeasyCASecretNamespace).
		Get(ctx, constants.KubeasyCASecretName, metav1.GetOptions{})
	if err == nil {
		logger.Info("kubeasy-ca Secret already exists, skipping CA generation")
		return ComponentResult{Name: name, Status: StatusReady, Message: "already exists"}
	}
	if !apierrors.IsNotFound(err) {
		return notReady(name, fmt.Errorf("failed to check kubeasy-ca Secret: %w", err))
	}

	// Generate ECDSA P-256 CA key.
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return notReady(name, fmt.Errorf("failed to generate CA key: %w", err))
	}

	// Build self-signed CA certificate template.
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return notReady(name, fmt.Errorf("failed to generate serial number: %w", err))
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "Kubeasy Local CA"},
		NotBefore:    now.Add(-time.Minute), // small back-date to handle clock skew
		NotAfter:     now.Add(10 * 365 * 24 * time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &caKey.PublicKey, caKey)
	if err != nil {
		return notReady(name, fmt.Errorf("failed to create CA certificate: %w", err))
	}

	// PEM-encode cert and key.
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(caKey)
	if err != nil {
		return notReady(name, fmt.Errorf("failed to marshal CA key: %w", err))
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	// Create the well-known TLS Secret in cert-manager namespace.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.KubeasyCASecretName,
			Namespace: constants.KubeasyCASecretNamespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			constants.KubeasyCASecretCertKey: certPEM,
			constants.KubeasyCASecretKeyKey:  keyPEM,
		},
	}
	if _, err := clientset.CoreV1().Secrets(constants.KubeasyCASecretNamespace).
		Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		return notReady(name, fmt.Errorf("failed to create kubeasy-ca Secret: %w", err))
	}
	logger.Info("kubeasy-ca Secret created.")

	// Build a fresh REST mapper so the cert-manager ClusterIssuer CRD is resolvable.
	freshGroups, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		return notReady(name, fmt.Errorf("failed to refresh API resources for ClusterIssuer: %w", err))
	}
	freshMapper := restmapper.NewDiscoveryRESTMapper(freshGroups)

	// Apply the ClusterIssuer that references the CA Secret.
	if err := kube.ApplyManifest(ctx, []byte(clusterIssuerManifest), "", freshMapper, dynamicClient); err != nil {
		return notReady(name, fmt.Errorf("failed to apply kubeasy-ca ClusterIssuer: %w", err))
	}
	logger.Info("kubeasy-ca ClusterIssuer created.")

	return ComponentResult{Name: name, Status: StatusReady, Message: "CA generated and ClusterIssuer created"}
}

// SetupAllComponents installs all infrastructure components and returns a ComponentResult for each.
// The order is: kyverno, local-path-provisioner, nginx-ingress, gateway-api, cert-manager, cloud-provider-kind.
// Execution continues regardless of individual component failures — all six results are always returned.
func SetupAllComponents(ctx context.Context, clientset *kubernetes.Clientset, dynamicClient dynamic.Interface) []ComponentResult {
	// Build REST mapper from API discovery — used for components that don't rebuild their own mapper.
	// Gateway API rebuilds its mapper internally after CRD install (two-pass apply).
	groups, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	var mapper meta.RESTMapper
	if err != nil {
		logger.Debug("Failed to discover API resources for REST mapper: %v — component installs may fail", err)
		mapper = nil
	} else {
		mapper = restmapper.NewDiscoveryRESTMapper(groups)
	}

	results := make([]ComponentResult, 0, 7)

	results = append(results, installKyverno(ctx, clientset, dynamicClient, mapper))
	results = append(results, installLocalPathProvisioner(ctx, clientset, dynamicClient, mapper))
	results = append(results, installNginxIngress(ctx, clientset, dynamicClient, mapper))
	results = append(results, installGatewayAPI(ctx, clientset, dynamicClient))
	results = append(results, installCertManager(ctx, clientset, dynamicClient, mapper))
	// kubeasy-ca must run after cert-manager is ready (ClusterIssuer CRD must exist).
	results = append(results, installKubeasyCA(ctx, clientset, dynamicClient))
	results = append(results, ensureCloudProviderKind(ctx))

	return results
}

// SetupInfrastructure installs Kyverno and local-path-provisioner directly into the cluster.
// Use SetupAllComponents for per-component status across all 6 infrastructure components.
func SetupInfrastructure() error {
	logger.Info("Starting infrastructure setup (Kyverno + local-path-provisioner)...")

	ctx, cancel := context.WithTimeout(context.Background(), defaultInfrastructureTimeout)
	defer cancel()

	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes client: %w", err)
	}
	dynamicClient, err := kube.GetDynamicClient()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes dynamic client: %w", err)
	}

	// Build REST mapper from API discovery (used for all ApplyManifest calls).
	// This is a point-in-time snapshot: CRD types registered by the manifests applied
	// below won't be resolvable within this call. That is acceptable here because neither
	// the Kyverno nor the local-path-provisioner install manifest applies instances of
	// their own CRD types — they only create the CRDs themselves (CustomResourceDefinition,
	// a standard type always present in discovery).
	groups, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		return fmt.Errorf("failed to discover API resources: %w", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groups)

	// Install Kyverno
	logger.Info("Installing Kyverno %s...", KyvernoVersion)
	if err := kube.CreateNamespace(ctx, clientset, kyvernoNamespace); err != nil {
		return fmt.Errorf("failed to create kyverno namespace: %w", err)
	}

	kyvernoURL := kyvernoInstallURL()
	logger.Debug("Fetching Kyverno manifest from %s", kyvernoURL)
	kyvernoManifest, err := kube.FetchManifest(kyvernoURL)
	if err != nil {
		return fmt.Errorf("failed to download Kyverno manifest: %w", err)
	}
	logger.Debug("Kyverno manifest fetched (%d bytes)", len(kyvernoManifest))

	if err := kube.ApplyManifest(ctx, kyvernoManifest, kyvernoNamespace, mapper, dynamicClient); err != nil {
		return fmt.Errorf("failed to apply Kyverno manifest: %w", err)
	}
	logger.Info("Kyverno manifest applied.")

	// Install local-path-provisioner
	logger.Info("Installing local-path-provisioner %s...", LocalPathProvisionerVersion)
	if err := kube.CreateNamespace(ctx, clientset, localPathStorageNamespace); err != nil {
		return fmt.Errorf("failed to create local-path-storage namespace: %w", err)
	}

	localPathURL := localPathProvisionerInstallURL()
	logger.Debug("Fetching local-path-provisioner manifest from %s", localPathURL)
	localPathManifest, err := kube.FetchManifest(localPathURL)
	if err != nil {
		return fmt.Errorf("failed to download local-path-provisioner manifest: %w", err)
	}
	logger.Debug("local-path-provisioner manifest fetched (%d bytes)", len(localPathManifest))

	if err := kube.ApplyManifest(ctx, localPathManifest, localPathStorageNamespace, mapper, dynamicClient); err != nil {
		return fmt.Errorf("failed to apply local-path-provisioner manifest: %w", err)
	}
	logger.Info("local-path-provisioner manifest applied.")

	// Wait for all deployments to be ready
	logger.Info("Waiting for infrastructure components to be ready...")

	kyvernoDeployments := []string{
		"kyverno-admission-controller",
		"kyverno-background-controller",
		"kyverno-cleanup-controller",
		"kyverno-reports-controller",
	}
	if err := kube.WaitForDeploymentsReady(ctx, clientset, kyvernoNamespace, kyvernoDeployments); err != nil {
		return fmt.Errorf("kyverno deployments failed to become ready: %w", err)
	}
	logger.Info("Kyverno is ready.")

	localPathDeployments := []string{"local-path-provisioner"}
	if err := kube.WaitForDeploymentsReady(ctx, clientset, localPathStorageNamespace, localPathDeployments); err != nil {
		return fmt.Errorf("local-path-provisioner deployment failed to become ready: %w", err)
	}
	logger.Info("local-path-provisioner is ready.")

	logger.Info("Infrastructure setup completed successfully.")
	return nil
}

// IsInfrastructureReady checks if Kyverno and local-path-provisioner are installed and ready.
func IsInfrastructureReady() (bool, error) {
	logger.Debug("Checking if infrastructure is already installed...")

	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		return false, fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	return IsInfrastructureReadyWithClient(context.Background(), clientset)
}

// IsInfrastructureReadyWithClient checks infrastructure readiness using the provided client.
// Delegates to the per-component checkers to avoid duplicating readiness logic.
func IsInfrastructureReadyWithClient(ctx context.Context, clientset kubernetes.Interface) (bool, error) {
	if ok, err := isKyvernoReadyWithClient(ctx, clientset); !ok || err != nil {
		return ok, err
	}
	ok, err := isLocalPathProvisionerReadyWithClient(ctx, clientset)
	if ok {
		logger.Info("Infrastructure is already installed and ready")
	}
	return ok, err
}

// certManagerCRDsURL returns the URL for the cert-manager CRDs manifest.
func certManagerCRDsURL() string {
	return fmt.Sprintf("https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.crds.yaml", CertManagerVersion)
}

// certManagerInstallURL returns the URL for the cert-manager controller install manifest.
func certManagerInstallURL() string {
	return fmt.Sprintf("https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml", CertManagerVersion)
}

// isCertManagerReadyWithClient checks cert-manager readiness using the provided client.
// It returns true when the cert-manager namespace exists and all three deployments
// (cert-manager, cert-manager-cainjector, cert-manager-webhook) have all replicas ready.
func isCertManagerReadyWithClient(ctx context.Context, clientset kubernetes.Interface) (bool, error) {
	// Check cert-manager namespace exists
	_, err := clientset.CoreV1().Namespaces().Get(ctx, certManagerNamespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("cert-manager namespace does not exist")
			return false, nil
		}
		return false, fmt.Errorf("error checking cert-manager namespace: %w", err)
	}

	// Check all three cert-manager deployments
	certManagerDeployments := []string{
		"cert-manager",
		"cert-manager-cainjector",
		"cert-manager-webhook",
	}
	for _, name := range certManagerDeployments {
		dep, err := clientset.AppsV1().Deployments(certManagerNamespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				logger.Debug("cert-manager deployment '%s' not found", name)
				return false, nil
			}
			return false, fmt.Errorf("error checking cert-manager deployment '%s': %w", name, err)
		}
		if dep.Status.ReadyReplicas == 0 || dep.Status.ReadyReplicas != dep.Status.Replicas {
			logger.Debug("cert-manager deployment '%s' not ready (Ready: %d/%d)", name, dep.Status.ReadyReplicas, dep.Status.Replicas)
			return false, nil
		}
	}

	logger.Info("cert-manager is already installed and ready")
	return true, nil
}

// waitForCertManagerWebhookEndpoints polls the cert-manager-webhook Endpoints object
// until at least one address is present in any subset. It polls every 5 seconds up to
// the deadline of ctx (max 60 seconds recommended at call site).
func waitForCertManagerWebhookEndpoints(ctx context.Context, clientset kubernetes.Interface) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, 60*time.Second, true, func(ctx context.Context) (bool, error) {
		ep, err := clientset.CoreV1().Endpoints(certManagerNamespace).Get(ctx, "cert-manager-webhook", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		for _, subset := range ep.Subsets {
			if len(subset.Addresses) > 0 {
				return true, nil
			}
		}
		return false, nil
	})
}

// installCertManager installs cert-manager using a two-pass apply: CRDs first, then the
// controller manifest. After the deployments are ready it polls the webhook Endpoints
// until at least one address is present. Returns a ComponentResult — never an error.
// Called by setup.go (plan 04) when setting up the infrastructure.
func installCertManager(ctx context.Context, clientset *kubernetes.Clientset, dynamicClient dynamic.Interface, mapper meta.RESTMapper) ComponentResult {
	// Idempotency check
	ready, err := isCertManagerReadyWithClient(ctx, clientset)
	if err != nil {
		return notReady("cert-manager", err)
	}
	if ready {
		return ComponentResult{Name: "cert-manager", Status: StatusReady, Message: "already installed"}
	}

	// Pass 1: CRDs
	logger.Info("Installing cert-manager %s (pass 1: CRDs)...", CertManagerVersion)
	crdsManifest, err := kube.FetchManifest(certManagerCRDsURL())
	if err != nil {
		return notReady("cert-manager", err)
	}
	if err := kube.CreateNamespace(ctx, clientset, certManagerNamespace); err != nil {
		return notReady("cert-manager", err)
	}
	if err := kube.ApplyManifest(ctx, crdsManifest, certManagerNamespace, mapper, dynamicClient); err != nil {
		return notReady("cert-manager", err)
	}

	// Pass 2: controller (cert-manager.yaml includes CRDs too — apply is idempotent)
	logger.Info("Installing cert-manager %s (pass 2: controller)...", CertManagerVersion)
	ctrlManifest, err := kube.FetchManifest(certManagerInstallURL())
	if err != nil {
		return notReady("cert-manager", err)
	}
	if err := kube.ApplyManifest(ctx, ctrlManifest, certManagerNamespace, mapper, dynamicClient); err != nil {
		return notReady("cert-manager", err)
	}

	// Wait for all three deployments to be ready
	certManagerDeployments := []string{"cert-manager", "cert-manager-cainjector", "cert-manager-webhook"}
	if err := kube.WaitForDeploymentsReady(ctx, clientset, certManagerNamespace, certManagerDeployments); err != nil {
		return notReady("cert-manager", err)
	}

	// Extra webhook endpoint polling — cert-manager webhook needs time after pods are ready
	if err := waitForCertManagerWebhookEndpoints(ctx, clientset); err != nil {
		return notReady("cert-manager", err)
	}

	logger.Info("cert-manager installed and webhook ready.")
	return ComponentResult{Name: "cert-manager", Status: StatusReady}
}

// nginxIngressKindManifestURL returns the URL for the nginx-ingress Kind-specific deploy manifest.
func nginxIngressKindManifestURL() string {
	return fmt.Sprintf(
		"https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-%s/deploy/static/provider/kind/deploy.yaml",
		NginxIngressVersion,
	)
}

// gatewayAPICRDsURL returns the URL for the Gateway API CRDs manifest.
func gatewayAPICRDsURL() string {
	return fmt.Sprintf(
		"https://github.com/kubernetes-sigs/gateway-api/releases/download/%s/standard-install.yaml",
		GatewayAPICRDsVersion,
	)
}

const nginxIngressNamespace = "ingress-nginx"

// gatewayClassManifest is the GatewayClass resource for cloud-provider-kind.
// Applied after the Gateway API CRDs so the GatewayClass API is available.
// Used by installGatewayAPI (called by setup.go in plan 04).
var gatewayClassManifest = "apiVersion: gateway.networking.k8s.io/v1\nkind: GatewayClass\nmetadata:\n  name: cloud-provider-kind\nspec:\n  controllerName: sigs.k8s.io/cloud-provider-kind\n"

// isNginxIngressReadyWithClient checks nginx-ingress readiness using the provided client.
// It returns true when the ingress-nginx namespace exists and the ingress-nginx-controller
// deployment has all replicas ready.
func isNginxIngressReadyWithClient(ctx context.Context, clientset kubernetes.Interface) (bool, error) {
	// Check ingress-nginx namespace exists
	_, err := clientset.CoreV1().Namespaces().Get(ctx, nginxIngressNamespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("ingress-nginx namespace does not exist")
			return false, nil
		}
		return false, fmt.Errorf("error checking ingress-nginx namespace: %w", err)
	}

	// Check ingress-nginx-controller deployment
	dep, err := clientset.AppsV1().Deployments(nginxIngressNamespace).Get(ctx, "ingress-nginx-controller", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("ingress-nginx-controller deployment not found")
			return false, nil
		}
		return false, fmt.Errorf("error checking ingress-nginx-controller deployment: %w", err)
	}
	if dep.Status.ReadyReplicas == 0 || dep.Status.ReadyReplicas != dep.Status.Replicas {
		logger.Debug("ingress-nginx-controller not ready (Ready: %d/%d)", dep.Status.ReadyReplicas, dep.Status.Replicas)
		return false, nil
	}

	logger.Info("nginx-ingress is already installed and ready")
	return true, nil
}

// isGatewayAPICRDsInstalled checks if Gateway API CRDs are installed by querying the
// discovery API for the gateway.networking.k8s.io/v1 group.
// Returns true if the API group is registered, false otherwise.
func isGatewayAPICRDsInstalled(_ context.Context, clientset kubernetes.Interface) (bool, error) {
	_, err := clientset.Discovery().ServerResourcesForGroupVersion("gateway.networking.k8s.io/v1")
	if err != nil {
		logger.Debug("Gateway API CRDs not installed: %v", err)
		return false, nil
	}
	return true, nil
}

// installNginxIngress installs the nginx-ingress controller for Kind clusters.
// It returns ComponentResult{Status: StatusReady} if already installed or on success,
// and ComponentResult{Status: StatusNotReady} on any installation error.
// Called by setup.go (plan 04) when setting up the infrastructure.
func installNginxIngress(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface, mapper meta.RESTMapper) ComponentResult {
	const name = "nginx-ingress"

	ready, err := isNginxIngressReadyWithClient(ctx, clientset)
	if err != nil {
		return notReady(name, err)
	}
	if ready {
		logger.Info("nginx-ingress is already installed and ready, skipping installation")
		return ComponentResult{Name: name, Status: StatusReady, Message: "already installed"}
	}

	logger.Info("Installing nginx-ingress %s...", NginxIngressVersion)

	if err := kube.CreateNamespace(ctx, clientset, nginxIngressNamespace); err != nil {
		return notReady(name, fmt.Errorf("failed to create ingress-nginx namespace: %w", err))
	}

	manifestURL := nginxIngressKindManifestURL()
	logger.Debug("Fetching nginx-ingress manifest from %s", manifestURL)
	manifest, err := kube.FetchManifest(manifestURL)
	if err != nil {
		return notReady(name, fmt.Errorf("failed to download nginx-ingress manifest: %w", err))
	}

	if err := kube.ApplyManifest(ctx, manifest, nginxIngressNamespace, mapper, dynamicClient); err != nil {
		return notReady(name, fmt.Errorf("failed to apply nginx-ingress manifest: %w", err))
	}

	// kube.WaitForDeploymentsReady requires *kubernetes.Clientset.
	cs, ok := clientset.(*kubernetes.Clientset)
	if !ok {
		return notReady(name, fmt.Errorf("internal error: clientset is not *kubernetes.Clientset"))
	}
	if err := kube.WaitForDeploymentsReady(ctx, cs, nginxIngressNamespace, []string{"ingress-nginx-controller"}); err != nil {
		return notReady(name, fmt.Errorf("nginx-ingress deployment failed to become ready: %w", err))
	}

	logger.Info("nginx-ingress installed and ready.")
	return ComponentResult{Name: name, Status: StatusReady, Message: "installed successfully"}
}

// installGatewayAPI installs Gateway API CRDs and creates the cloud-provider-kind GatewayClass.
// It performs a two-pass apply: first the CRDs, then rebuilds the REST mapper, then the GatewayClass.
// Returns ComponentResult{Status: StatusReady} if already installed or on success,
// and ComponentResult{Status: StatusNotReady} on any installation error.
func installGatewayAPI(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface) ComponentResult {
	const name = "gateway-api"

	installed, err := isGatewayAPICRDsInstalled(ctx, clientset)
	if err != nil {
		return notReady(name, err)
	}
	if installed {
		logger.Info("Gateway API CRDs are already installed, skipping installation")
		return ComponentResult{Name: name, Status: StatusReady, Message: "already installed"}
	}

	logger.Info("Installing Gateway API CRDs %s...", GatewayAPICRDsVersion)

	// Pass 1: Apply CRDs manifest (cluster-scoped, empty namespace)
	crdsURL := gatewayAPICRDsURL()
	logger.Debug("Fetching Gateway API CRDs manifest from %s", crdsURL)
	crdsManifest, err := kube.FetchManifest(crdsURL)
	if err != nil {
		return notReady(name, fmt.Errorf("failed to download Gateway API CRDs manifest: %w", err))
	}

	// Build REST mapper for initial apply
	groups, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		return notReady(name, fmt.Errorf("failed to discover API resources: %w", err))
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groups)

	if err := kube.ApplyManifest(ctx, crdsManifest, "", mapper, dynamicClient); err != nil {
		return notReady(name, fmt.Errorf("failed to apply Gateway API CRDs: %w", err))
	}
	logger.Info("Gateway API CRDs applied.")

	// Pass 2: Rebuild REST mapper so GatewayClass type is resolvable, then apply GatewayClass
	freshGroups, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		return notReady(name, fmt.Errorf("failed to refresh API resources after CRD install: %w", err))
	}
	freshMapper := restmapper.NewDiscoveryRESTMapper(freshGroups)

	if err := kube.ApplyManifest(ctx, []byte(gatewayClassManifest), "", freshMapper, dynamicClient); err != nil {
		return notReady(name, fmt.Errorf("failed to apply GatewayClass manifest: %w", err))
	}
	logger.Info("GatewayClass cloud-provider-kind created.")

	return ComponentResult{Name: name, Status: StatusReady, Message: "installed successfully"}
}
