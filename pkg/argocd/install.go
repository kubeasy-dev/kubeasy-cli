package argocd

import (
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors" // Add alias
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/logger"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/kube"
)

const (
	// ArgoCD namespace name
	ArgoCDNamespace = "argocd"

	// Default ArgoCD installation manifest URL
	DefaultManifestURL = "https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml"

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
			"argocd-server",
			"argocd-repo-server",
			// "argocd-application-controller", // This is a StatefulSet
			"argocd-redis",
			"argocd-dex-server",
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
	appManifestUrl := "https://raw.githubusercontent.com/kubeasy-dev/cli-setup/refs/heads/main/app-of-apps.yaml"
	logger.Debug("Fetching App-of-Apps manifest from %s...", appManifestUrl)
	appManifestBytes, err := kube.FetchManifest(appManifestUrl)
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

	gvr := schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}

	// --- Wait for kubeasy-cli-setup first ---
	cliSetupAppName := "kubeasy-cli-setup"
	logger.Info("Waiting specifically for %s to be Healthy and Synced...", cliSetupAppName)
	if err := waitForSpecificApp(ctx, dynamicClient, gvr, cliSetupAppName, ArgoCDNamespace); err != nil {
		// Error already logged in waitForSpecificApp
		return fmt.Errorf("error waiting for %s: %w", cliSetupAppName, err)
	}
	logger.Info("%s is Healthy and Synced.", cliSetupAppName)

	// --- Wait for the remaining apps ---
	remainingApps := []string{}
	for _, app := range appNames {
		if app != cliSetupAppName {
			remainingApps = append(remainingApps, app)
		}
	}

	if len(remainingApps) > 0 {
		logger.Info("Waiting for remaining applications (%s) via Kubernetes API...", strings.Join(remainingApps, ", "))
		ticker := time.NewTicker(5 * time.Second) // Increased ticker interval slightly
		defer ticker.Stop()

		for {
			allReady := true
			appsStatus := []string{}

			for _, app := range remainingApps {
				healthStatus, syncStatus, err := getAppStatus(ctx, dynamicClient, gvr, app, ArgoCDNamespace)
				if err != nil {
					// Log the specific error for this app
					statusStr := fmt.Sprintf("%s: Error getting status (%v)", app, err)
					appsStatus = append(appsStatus, statusStr)
					logger.Warning("Status check failed for app '%s': %v", app, err)
					allReady = false
					continue // Try next app
				}

				statusStr := fmt.Sprintf("%s: Health=%s Sync=%s", app, healthStatus, syncStatus)
				appsStatus = append(appsStatus, statusStr)

				if healthStatus != "Healthy" || syncStatus != "Synced" {
					allReady = false
				}
			}

			logger.Info("Current remaining app statuses: [%s]", strings.Join(appsStatus, "; "))

			if allReady {
				logger.Info("All remaining ArgoCD applications are ready.")
				break // Exit the loop for remaining apps
			}

			select {
			case <-ctx.Done():
				finalStatuses := []string{}
				// Get final statuses on timeout for better error reporting
				for _, app := range remainingApps {
					h, s, e := getAppStatus(context.Background(), dynamicClient, gvr, app, ArgoCDNamespace) // Use fresh context
					if e != nil {
						finalStatuses = append(finalStatuses, fmt.Sprintf("%s: Error (%v)", app, e))
					} else {
						finalStatuses = append(finalStatuses, fmt.Sprintf("%s: Health=%s Sync=%s", app, h, s))
					}
				}
				errMsg := fmt.Sprintf("timeout waiting for remaining ArgoCD applications (%s) to be ready. Final statuses: [%s]", strings.Join(remainingApps, ", "), strings.Join(finalStatuses, "; "))
				logger.Error("%s", errMsg)
				return fmt.Errorf("%s", errMsg)
			case <-ticker.C:
				logger.Debug("Retrying status check for remaining apps...")
				// Continue loop
			}
		}
	}

	logger.Info("All specified ArgoCD applications are ready!")
	return nil
}

// Helper function to wait for a specific app
func waitForSpecificApp(ctx context.Context, dynamicClient dynamic.Interface, gvr schema.GroupVersionResource, appName, namespace string) error {
	ticker := time.NewTicker(5 * time.Second) // Increased ticker interval slightly
	defer ticker.Stop()

	for {
		healthStatus, syncStatus, err := getAppStatus(ctx, dynamicClient, gvr, appName, namespace)
		if err != nil {
			// Log error but continue retrying, check context cancellation
			logger.Warning("Error getting status for %s: %v. Retrying...", appName, err)
		} else {
			logger.Info("Current status for %s: Health=%s Sync=%s", appName, healthStatus, syncStatus)
			if healthStatus == "Healthy" && syncStatus == "Synced" {
				return nil // App is ready
			}
		}

		select {
		case <-ctx.Done():
			// Get final status on timeout
			h, s, e := getAppStatus(context.Background(), dynamicClient, gvr, appName, namespace) // Use fresh context
			// finalStatus := "Unknown" // Removed ineffectual assignment
			var finalStatus string
			if e != nil {
				finalStatus = fmt.Sprintf("Error (%v)", e)
			} else {
				finalStatus = fmt.Sprintf("Health=%s Sync=%s", h, s)
			}
			errMsg := fmt.Sprintf("timeout waiting for application %s to be ready. Final status: %s", appName, finalStatus)
			logger.Error("%s", errMsg)
			return fmt.Errorf("%s", errMsg)
		case <-ticker.C:
			logger.Debug("Retrying status check for app '%s'...", appName)
			// Continue loop
		}
	}
}

// Helper function to get app status
func getAppStatus(ctx context.Context, dynamicClient dynamic.Interface, gvr schema.GroupVersionResource, appName, namespace string) (health string, sync string, err error) {
	health = "Unknown"
	sync = "Unknown"

	logger.Debug("Getting status for Application '%s/%s'...", namespace, appName)
	res, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		// Check if the error is just that the resource doesn't exist yet
		// Use apierrors alias here
		if apierrors.IsNotFound(err) {
			logger.Debug("Application '%s/%s' not found yet.", namespace, appName)
			return "NotFound", "NotFound", nil // Return specific statuses instead of error
		}
		logger.Warning("Error getting Application '%s/%s': %v", namespace, appName, err)
		return health, sync, err // Return the actual error for other issues
	}

	// Extract health status
	healthStatus, found, err := unstructured.NestedString(res.Object, "status", "health", "status")
	if err != nil {
		logger.Warning("Error extracting health status for '%s/%s': %v", namespace, appName, err)
	} else if found {
		health = healthStatus
	} else {
		logger.Debug("Health status field not found or not a string for '%s/%s'.", namespace, appName)
	}

	// Extract sync status
	syncStatus, found, err := unstructured.NestedString(res.Object, "status", "sync", "status")
	if err != nil {
		logger.Warning("Error extracting sync status for '%s/%s': %v", namespace, appName, err)
	} else if found {
		sync = syncStatus
	} else {
		logger.Debug("Sync status field not found or not a string for '%s/%s'.", namespace, appName)
	}

	logger.Debug("Status for '%s/%s': Health='%s', Sync='%s'", namespace, appName, health, sync)
	return health, sync, nil
}
