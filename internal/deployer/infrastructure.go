package deployer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// Called by setup.go (plan 04) when creating the Kind cluster with port mappings.
func writeKindConfig(cfg *kindv1alpha4.Cluster) error { //nolint:unused // used by setup.go in plan 04
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

// hasExtraPortMappings reports whether the kind-config.yaml at GetKindConfigPath() contains
// ExtraPortMappings for both HostPort 8080 and 8443 on the first node.
// Returns false if the file is missing or ports are absent — absence is not an error.
// Called by setup.go (plan 04) to detect whether cluster recreation is needed.
func hasExtraPortMappings() bool { //nolint:unused // used by setup.go in plan 04
	return hasExtraPortMappingsAt(constants.GetKindConfigPath())
}

// hasExtraPortMappingsAt is the testable variant of hasExtraPortMappings that reads from the given path.
func hasExtraPortMappingsAt(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		// File absent means cluster was created without this config — not an error.
		return false
	}
	var cfg kindv1alpha4.Cluster
	if err := sigsyaml.Unmarshal(data, &cfg); err != nil {
		return false
	}
	if len(cfg.Nodes) == 0 {
		return false
	}
	has8080, has8443 := false, false
	for _, pm := range cfg.Nodes[0].ExtraPortMappings {
		if pm.HostPort == 8080 {
			has8080 = true
		}
		if pm.HostPort == 8443 {
			has8443 = true
		}
	}
	return has8080 && has8443
}

const (
	kyvernoNamespace             = "kyverno"
	localPathStorageNamespace    = "local-path-storage"
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

// SetupInfrastructure installs Kyverno and local-path-provisioner directly into the cluster.
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
func IsInfrastructureReadyWithClient(ctx context.Context, clientset kubernetes.Interface) (bool, error) {
	// Check Kyverno namespace exists
	_, err := clientset.CoreV1().Namespaces().Get(ctx, kyvernoNamespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("Kyverno namespace does not exist")
			return false, nil
		}
		return false, fmt.Errorf("error checking kyverno namespace: %w", err)
	}

	// Check Kyverno deployments
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

	// Check local-path-storage namespace exists
	_, err = clientset.CoreV1().Namespaces().Get(ctx, localPathStorageNamespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("local-path-storage namespace does not exist")
			return false, nil
		}
		return false, fmt.Errorf("error checking local-path-storage namespace: %w", err)
	}

	// Check local-path-provisioner deployment
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

	logger.Info("Infrastructure is already installed and ready")
	return true, nil
}
