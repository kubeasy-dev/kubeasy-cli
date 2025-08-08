package argocd

import (
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors" // Add alias
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/logger"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/kube"
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

	// Install ArgoCD application (App of Apps) AFTER core components are ready
	logger.Info("Applying App-of-Apps manifest (kubeasy-cli-setup)...")
	appManifestURL := "https://raw.githubusercontent.com/kubeasy-dev/cli-setup/refs/heads/main/app-of-apps.yaml"
	logger.Debug("Fetching App-of-Apps manifest from %s...", appManifestURL)
	appManifestBytes, err := kube.FetchManifest(appManifestURL)
	if err != nil {
		logger.Error("Failed to fetch App-of-Apps manifest: %v", err)
		return err
	}
	logger.Debug("App-of-Apps manifest fetched successfully (%d bytes).", len(appManifestBytes))

	logger.Info("Applying App-of-Apps manifest to namespace '%s'...", ArgoCDNamespace)
	if err = kube.ApplyManifest(ctx, appManifestBytes, ArgoCDNamespace, clientset, dynamicClient); err != nil {
		logger.Error("Failed to apply App-of-Apps manifest: %v", err)
		return err
	}
	logger.Info("App-of-Apps manifest applied successfully.")

	logger.Info("ArgoCD installation process completed.")
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
		CheckHealth:    true,
		CheckSync:      true,
		CheckOperation: false,
		Timeout:        timeout,
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

	ctx := context.Background()

	// Check if ArgoCD namespace exists
	_, err = clientset.CoreV1().Namespaces().Get(ctx, ArgoCDNamespace, metav1.GetOptions{})
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
