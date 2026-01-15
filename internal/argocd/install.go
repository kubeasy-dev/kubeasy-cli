package argocd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors" // Add alias
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
)

const (
	// ArgoCD namespace name
	ArgoCDNamespace = "argocd"

	// Default ArgoCD installation manifest URL
	DefaultManifestURL = "https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/core-install.yaml"

	// Default timeout for installation
	DefaultInstallTimeout = 5 * time.Minute
)

// InstallOptions defines options for installing ArgoCD
type InstallOptions struct {
	ManifestURL    string
	InstallTimeout time.Duration
	WaitForReady   bool
}

// DefaultInstallOptions returns the default options
func DefaultInstallOptions() *InstallOptions {
	return &InstallOptions{
		ManifestURL:    DefaultManifestURL,
		InstallTimeout: DefaultInstallTimeout,
		WaitForReady:   true,
	}
}

// generateSecretKey generates a cryptographically secure random secret key
func generateSecretKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate random secret key: %w", err)
	}
	// Encode to hex string for readability in the secret
	return []byte(hex.EncodeToString(key)), nil
}

// ensureServerSecretKey ensures the server.secretkey exists in argocd-secret
// and restarts the application controller if it was added.
// See: https://github.com/argoproj/argo-cd/issues/22931
func ensureServerSecretKey(ctx context.Context, clientset kubernetes.Interface) error {
	logger.Info("Checking server.secretkey in argocd-secret...")
	secret, err := clientset.CoreV1().Secrets(ArgoCDNamespace).Get(ctx, "argocd-secret", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get argocd-secret: %w", err)
	}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}

	// Add server.secretkey if it doesn't exist
	if _, exists := secret.Data["server.secretkey"]; !exists {
		logger.Info("Adding server.secretkey to argocd-secret...")
		secretKey, err := generateSecretKey()
		if err != nil {
			return err
		}
		secret.Data["server.secretkey"] = secretKey
		if _, err = clientset.CoreV1().Secrets(ArgoCDNamespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update argocd-secret with server.secretkey: %w", err)
		}
		logger.Info("server.secretkey added to argocd-secret")

		// Restart application controller to pick up the new secret
		if err = restartApplicationController(ctx, clientset); err != nil {
			return err
		}
	} else {
		logger.Debug("server.secretkey already exists in argocd-secret")
	}

	return nil
}

// restartApplicationController triggers a restart of the ArgoCD application controller
func restartApplicationController(ctx context.Context, clientset kubernetes.Interface) error {
	logger.Info("Restarting ArgoCD application controller...")
	sts, err := clientset.AppsV1().StatefulSets(ArgoCDNamespace).Get(ctx, "argocd-application-controller", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get argocd-application-controller StatefulSet: %w", err)
	}

	if sts.Spec.Template.Annotations == nil {
		sts.Spec.Template.Annotations = make(map[string]string)
	}
	sts.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	if _, err = clientset.AppsV1().StatefulSets(ArgoCDNamespace).Update(ctx, sts, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to restart argocd-application-controller: %w", err)
	}
	logger.Info("ArgoCD application controller restarted")
	return nil
}

// InstallArgoCD installs ArgoCD into the cluster
func InstallArgoCD(options *InstallOptions) error {
	if options == nil {
		options = DefaultInstallOptions()
	}
	logger.Info("Starting ArgoCD installation with options: ManifestURL=%s, Timeout=%s, WaitForReady=%t",
		options.ManifestURL, options.InstallTimeout, options.WaitForReady)

	ctx, cancel := context.WithTimeout(context.Background(), options.InstallTimeout)
	defer cancel()

	// Get Kubernetes clients
	logger.Debug("Getting Kubernetes clients...")
	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		logger.Error("Failed to get Kubernetes clientset: %v", err)
		return err
	}
	dynamicClient, err := kube.GetDynamicClient()
	if err != nil {
		logger.Error("Failed to get Kubernetes dynamic client: %v", err)
		return err
	}
	logger.Debug("Kubernetes clients obtained successfully.")

	// Create namespace
	logger.Info("Ensuring ArgoCD namespace '%s' exists...", ArgoCDNamespace)
	if err = kube.CreateNamespace(ctx, clientset, ArgoCDNamespace); err != nil {
		logger.Error("Failed to ensure namespace '%s' exists: %v", ArgoCDNamespace, err)
		return err
	}
	logger.Info("Namespace '%s' ensured.", ArgoCDNamespace)

	// Fetch ArgoCD installation manifest
	logger.Info("Fetching ArgoCD manifest from %s...", options.ManifestURL)
	manifestBytes, err := kube.FetchManifest(options.ManifestURL)
	if err != nil {
		logger.Error("Failed to fetch ArgoCD manifest: %v", err)
		return err
	}
	logger.Info("ArgoCD manifest fetched successfully (%d bytes).", len(manifestBytes))

	// Apply the main ArgoCD manifest
	logger.Info("Applying main ArgoCD manifest...")
	if err = kube.ApplyManifest(ctx, manifestBytes, ArgoCDNamespace, clientset, dynamicClient); err != nil {
		// ApplyManifest already logs details, just log the final outcome here
		logger.Error("Failed to apply main ArgoCD manifest: %v", err)
		return err
	}
	logger.Info("Main ArgoCD manifest applied.")

	// Wait for ArgoCD core deployments to be ready if requested
	if options.WaitForReady {
		logger.Info("Waiting for ArgoCD core components to be ready...")
		deployments := []string{
			"argocd-applicationset-controller",
			"argocd-redis",
			"argocd-repo-server",
		}
		logger.Debug("Waiting for Deployments: %s", strings.Join(deployments, ", "))
		if err = kube.WaitForDeploymentsReady(ctx, clientset, ArgoCDNamespace, deployments); err != nil {
			logger.Error("ArgoCD core Deployments failed to become ready: %v", err)
			return fmt.Errorf("ArgoCD core Deployments failed to become ready: %w", err)
		}
		logger.Info("ArgoCD core Deployments are ready.")

		// Wait for ArgoCD core StatefulSets to be ready
		statefulsets := []string{
			"argocd-application-controller",
		}
		logger.Debug("Waiting for StatefulSets: %s", strings.Join(statefulsets, ", "))
		if err = kube.WaitForStatefulSetsReady(ctx, clientset, ArgoCDNamespace, statefulsets); err != nil {
			logger.Error("ArgoCD core StatefulSets failed to become ready: %v", err)
			return fmt.Errorf("ArgoCD core StatefulSets failed to become ready: %w", err)
		}
		logger.Info("ArgoCD core StatefulSets are ready.")
	}

	// Fix ArgoCD Core v3.0.0+ bug: add server.secretkey to argocd-secret
	if err = ensureServerSecretKey(ctx, clientset); err != nil {
		logger.Error("Failed to ensure server.secretkey: %v", err)
		return err
	}

	// Create default AppProject FIRST (ArgoCD Core doesn't create it automatically)
	logger.Info("Creating default ArgoCD AppProject...")
	defaultProjectManifest := `apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: default
  namespace: argocd
spec:
  description: Default project
  sourceRepos:
  - '*'
  destinations:
  - namespace: '*'
    server: '*'
  clusterResourceWhitelist:
  - group: '*'
    kind: '*'
`
	logger.Debug("Applying default AppProject manifest...")
	if err = kube.ApplyManifest(ctx, []byte(defaultProjectManifest), ArgoCDNamespace, clientset, dynamicClient); err != nil {
		logger.Error("Failed to apply default AppProject manifest: %v", err)
		return err
	}
	logger.Info("Default AppProject created successfully.")

	// Install ArgoCD applications from embedded manifests
	logger.Info("Applying embedded ArgoCD application manifests...")

	// Apply ArgoCD self-management application
	logger.Debug("Loading embedded ArgoCD application manifest...")
	argocdAppManifest, err := GetArgoCDAppManifest()
	if err != nil {
		logger.Error("Failed to load embedded ArgoCD application manifest: %v", err)
		return err
	}
	logger.Debug("ArgoCD application manifest loaded (%d bytes).", len(argocdAppManifest))

	logger.Info("Applying ArgoCD application manifest to namespace '%s'...", ArgoCDNamespace)
	if err = kube.ApplyManifest(ctx, argocdAppManifest, ArgoCDNamespace, clientset, dynamicClient); err != nil {
		logger.Error("Failed to apply ArgoCD application manifest: %v", err)
		return err
	}
	logger.Info("ArgoCD application manifest applied successfully.")

	// Apply Kyverno application
	logger.Debug("Loading embedded Kyverno application manifest...")
	kyvernoAppManifest, err := GetKyvernoAppManifest()
	if err != nil {
		logger.Error("Failed to load embedded Kyverno application manifest: %v", err)
		return err
	}
	logger.Debug("Kyverno application manifest loaded (%d bytes).", len(kyvernoAppManifest))

	logger.Info("Applying Kyverno application manifest to namespace '%s'...", ArgoCDNamespace)
	if err = kube.ApplyManifest(ctx, kyvernoAppManifest, ArgoCDNamespace, clientset, dynamicClient); err != nil {
		logger.Error("Failed to apply Kyverno application manifest: %v", err)
		return err
	}
	logger.Info("Kyverno application manifest applied successfully.")

	// Apply Local Path Provisioner application
	logger.Debug("Loading embedded Local Path Provisioner application manifest...")
	localPathProvisionerAppManifest, err := GetLocalPathProvisionerAppManifest()
	if err != nil {
		logger.Error("Failed to load embedded Local Path Provisioner application manifest: %v", err)
		return err
	}
	logger.Debug("Local Path Provisioner application manifest loaded (%d bytes).", len(localPathProvisionerAppManifest))

	logger.Info("Applying Local Path Provisioner application manifest to namespace '%s'...", ArgoCDNamespace)
	if err = kube.ApplyManifest(ctx, localPathProvisionerAppManifest, ArgoCDNamespace, clientset, dynamicClient); err != nil {
		logger.Error("Failed to apply Local Path Provisioner application manifest: %v", err)
		return err
	}
	logger.Info("Local Path Provisioner application manifest applied successfully.")

	logger.Info("ArgoCD installation process completed.")
	return nil
}

// EnsureArgoCDResources ensures that default project and app-of-apps exist
// This function can be called even if ArgoCD is already installed
func EnsureArgoCDResources() error {
	logger.Info("Ensuring ArgoCD resources (default project and app-of-apps)...")
	ctx := context.Background()

	// Get Kubernetes clients
	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		logger.Error("Failed to get Kubernetes clientset: %v", err)
		return err
	}
	dynamicClient, err := kube.GetDynamicClient()
	if err != nil {
		logger.Error("Failed to get Kubernetes dynamic client: %v", err)
		return err
	}

	// Fix ArgoCD Core v3.0.0+ bug: add server.secretkey to argocd-secret if missing
	if err = ensureServerSecretKey(ctx, clientset); err != nil {
		logger.Error("Failed to ensure server.secretkey: %v", err)
		return err
	}

	// Create default AppProject
	logger.Info("Creating/updating default ArgoCD AppProject...")
	defaultProjectManifest := `apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: default
  namespace: argocd
spec:
  description: Default project
  sourceRepos:
  - '*'
  destinations:
  - namespace: '*'
    server: '*'
  clusterResourceWhitelist:
  - group: '*'
    kind: '*'
`
	if err = kube.ApplyManifest(ctx, []byte(defaultProjectManifest), ArgoCDNamespace, clientset, dynamicClient); err != nil {
		logger.Error("Failed to apply default AppProject manifest: %v", err)
		return err
	}
	logger.Info("Default AppProject ensured.")

	// Apply embedded ArgoCD applications
	logger.Info("Creating/updating ArgoCD application manifests...")

	// Apply ArgoCD self-management application
	argocdAppManifest, err := GetArgoCDAppManifest()
	if err != nil {
		logger.Error("Failed to load embedded ArgoCD application manifest: %v", err)
		return err
	}
	if err = kube.ApplyManifest(ctx, argocdAppManifest, ArgoCDNamespace, clientset, dynamicClient); err != nil {
		logger.Error("Failed to apply ArgoCD application manifest: %v", err)
		return err
	}
	logger.Info("ArgoCD application manifest ensured.")

	// Apply Kyverno application
	kyvernoAppManifest, err := GetKyvernoAppManifest()
	if err != nil {
		logger.Error("Failed to load embedded Kyverno application manifest: %v", err)
		return err
	}
	if err = kube.ApplyManifest(ctx, kyvernoAppManifest, ArgoCDNamespace, clientset, dynamicClient); err != nil {
		logger.Error("Failed to apply Kyverno application manifest: %v", err)
		return err
	}
	logger.Info("Kyverno application manifest ensured.")

	// Apply Local Path Provisioner application
	localPathProvisionerAppManifest, err := GetLocalPathProvisionerAppManifest()
	if err != nil {
		logger.Error("Failed to load embedded Local Path Provisioner application manifest: %v", err)
		return err
	}
	if err = kube.ApplyManifest(ctx, localPathProvisionerAppManifest, ArgoCDNamespace, clientset, dynamicClient); err != nil {
		logger.Error("Failed to apply Local Path Provisioner application manifest: %v", err)
		return err
	}
	logger.Info("Local Path Provisioner application manifest ensured.")

	logger.Info("ArgoCD resources ensured successfully.")
	return nil
}

// WaitForArgoCDAppsReadyCore uses the Kubernetes API directly to wait for apps to be Healthy and Synced
func WaitForArgoCDAppsReadyCore(appNames []string, timeout time.Duration) error {
	logger.Info("Waiting for specified ArgoCD applications to be Healthy and Synced: %s (Timeout: %s)", strings.Join(appNames, ", "), timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Get Kubernetes dynamic client configured for the correct context
	logger.Debug("Getting Kubernetes dynamic client for checking app status...")
	dynamicClient, err := kube.GetDynamicClient() // This already uses KubeasyClusterContext
	if err != nil {
		logger.Error("Failed to get Kubernetes dynamic client: %v", err)
		return fmt.Errorf("failed to get Kubernetes dynamic client: %w", err)
	}
	logger.Debug("Successfully obtained Kubernetes dynamic client.")

	// Wait briefly for API server stability after setup?
	logger.Debug("Waiting 5s for potential API server stabilization...")
	time.Sleep(5 * time.Second)

	// Use the new unified approach for all apps
	options := WaitOptions{
		CheckHealth: true,
		CheckSync:   true,
		Timeout:     timeout,
	}

	for _, appName := range appNames {
		logger.Info("Waiting for ArgoCD Application '%s'...", appName)
		if err := WaitForApplicationStatus(ctx, dynamicClient, appName, ArgoCDNamespace, options); err != nil {
			logger.Error("Failed waiting for ArgoCD Application '%s': %v", appName, err)
			return fmt.Errorf("error waiting for %s: %w", appName, err)
		}
		logger.Info("ArgoCD Application '%s' is ready.", appName)
	}

	logger.Info("All specified ArgoCD applications are ready!")
	return nil
}

// IsArgoCDInstalled checks if ArgoCD is already installed in the cluster
func IsArgoCDInstalled() (bool, error) {
	logger.Debug("Checking if ArgoCD is already installed...")

	// Get Kubernetes clients
	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		logger.Error("Failed to get Kubernetes clientset: %v", err)
		return false, err
	}

	return IsArgoCDInstalledWithClient(context.Background(), clientset)
}

// IsArgoCDInstalledWithClient checks if ArgoCD is already installed using the provided client.
// This function is useful for testing with mock clients.
func IsArgoCDInstalledWithClient(ctx context.Context, clientset kubernetes.Interface) (bool, error) {
	// Check if ArgoCD namespace exists
	_, err := clientset.CoreV1().Namespaces().Get(ctx, ArgoCDNamespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("ArgoCD namespace '%s' does not exist", ArgoCDNamespace)
			return false, nil
		}
		logger.Error("Error checking for ArgoCD namespace: %v", err)
		return false, err
	}

	// Check if core ArgoCD deployments exist and are ready
	deployments := []string{
		"argocd-applicationset-controller",
		"argocd-redis",
		"argocd-repo-server",
	}

	for _, deployment := range deployments {
		dep, err := clientset.AppsV1().Deployments(ArgoCDNamespace).Get(ctx, deployment, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				logger.Debug("ArgoCD deployment '%s' not found", deployment)
				return false, nil
			}
			logger.Error("Error checking deployment '%s': %v", deployment, err)
			return false, err
		}

		// Check if deployment is ready
		if dep.Status.ReadyReplicas == 0 || dep.Status.ReadyReplicas != dep.Status.Replicas {
			logger.Debug("ArgoCD deployment '%s' is not ready (ReadyReplicas: %d, TotalReplicas: %d)",
				deployment, dep.Status.ReadyReplicas, dep.Status.Replicas)
			return false, nil
		}
	}

	// Check if ArgoCD StatefulSet exists and is ready
	statefulsets := []string{
		"argocd-application-controller",
	}

	for _, statefulset := range statefulsets {
		sts, err := clientset.AppsV1().StatefulSets(ArgoCDNamespace).Get(ctx, statefulset, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				logger.Debug("ArgoCD StatefulSet '%s' not found", statefulset)
				return false, nil
			}
			logger.Error("Error checking StatefulSet '%s': %v", statefulset, err)
			return false, err
		}

		// Check if StatefulSet is ready
		if sts.Status.ReadyReplicas == 0 || sts.Status.ReadyReplicas != sts.Status.Replicas {
			logger.Debug("ArgoCD StatefulSet '%s' is not ready (ReadyReplicas: %d, TotalReplicas: %d)",
				statefulset, sts.Status.ReadyReplicas, sts.Status.Replicas)
			return false, nil
		}
	}

	logger.Info("ArgoCD is already installed and ready")
	return true, nil
}
