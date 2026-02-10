package deployer

import (
	"context"
	"fmt"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	kyvernoNamespace             = "kyverno"
	localPathStorageNamespace    = "local-path-storage"
	defaultInfrastructureTimeout = 5 * time.Minute
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

	if err := kube.ApplyManifest(ctx, kyvernoManifest, kyvernoNamespace, clientset, dynamicClient); err != nil {
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

	if err := kube.ApplyManifest(ctx, localPathManifest, localPathStorageNamespace, clientset, dynamicClient); err != nil {
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
