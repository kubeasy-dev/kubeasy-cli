package argocd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	argoAppGVR = schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}
)

// CreateOrUpdateChallengeApplication ensures an Argo CD Application exists for the given challenge.
func CreateOrUpdateChallengeApplication(ctx context.Context, dynamicClient dynamic.Interface, challengeSlug string) error {
	logger.Debug("Constructing Argo CD Application manifest for challenge '%s'", challengeSlug)

	argoApp := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":      challengeSlug,
				"namespace": ArgoCDNamespace,
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					"repoURL":        constants.ExercisesRepoURL,
					"path":           challengeSlug,
					"targetRevision": constants.ExercicesRepoBranch,
					"directory": map[string]interface{}{
						"recurse": true,
						"include": "{manifests/**.yaml,policies/**.yaml,validations/**.yaml}",
					},
				},
				"destination": map[string]interface{}{
					"server":    "https://kubernetes.default.svc",
					"namespace": challengeSlug,
				},
				"syncPolicy": map[string]interface{}{
					"retry": map[string]interface{}{
						"limit": 5,
						"backoff": map[string]interface{}{
							"duration":    "5s",
							"maxDuration": "30s",
						},
					},
				},
			},
		},
	}
	logger.Debug("Argo CD Application manifest constructed: %v", argoApp.Object)

	logger.Info("Applying Argo CD Application '%s' in namespace '%s'...", challengeSlug, ArgoCDNamespace)
	_, err := dynamicClient.Resource(argoAppGVR).Namespace(ArgoCDNamespace).Create(ctx, argoApp, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			logger.Warning("Argo CD Application '%s' already exists in namespace '%s'. Attempting update...", challengeSlug, ArgoCDNamespace)
			existingApp, getErr := dynamicClient.Resource(argoAppGVR).Namespace(ArgoCDNamespace).Get(ctx, challengeSlug, metav1.GetOptions{})
			if getErr != nil {
				logger.Error("Failed to get existing Argo CD Application '%s' for update: %v", challengeSlug, getErr)
				return fmt.Errorf("failed to get existing Argo CD Application '%s' for update: %w", challengeSlug, getErr)
			}
			argoApp.SetResourceVersion(existingApp.GetResourceVersion())
			_, updateErr := dynamicClient.Resource(argoAppGVR).Namespace(ArgoCDNamespace).Update(ctx, argoApp, metav1.UpdateOptions{})
			if updateErr != nil {
				logger.Error("Failed to update Argo CD Application '%s': %v", challengeSlug, updateErr)
				return fmt.Errorf("failed to update Argo CD Application '%s': %w", challengeSlug, updateErr)
			}
			logger.Info("Argo CD Application '%s' updated successfully in namespace '%s'.", challengeSlug, ArgoCDNamespace)
		} else {
			logger.Error("Failed to create Argo CD Application '%s' in namespace '%s': %v", challengeSlug, ArgoCDNamespace, err)
			return fmt.Errorf("failed to create Argo CD Application '%s' in namespace '%s': %w", challengeSlug, ArgoCDNamespace, err)
		}
	} else {
		logger.Info("Argo CD Application '%s' created successfully in namespace '%s'.", challengeSlug, ArgoCDNamespace)
	}

	// Patch to trigger a manual sync (patch 'operation' at root, not in spec)
	syncPatch := map[string]interface{}{
		"operation": map[string]interface{}{
			"sync": map[string]interface{}{
				"syncStrategy": map[string]interface{}{
					"hook": map[string]interface{}{},
				},
			},
			"initiatedBy": map[string]interface{}{
				"username": "kubeasy-cli",
			},
		},
	}
	patchBytes, err := json.Marshal(syncPatch)
	if err != nil {
		logger.Warning("Failed to marshal sync patch for Argo CD Application '%s': %v", challengeSlug, err)
		return nil // Not fatal
	}
	_, err = dynamicClient.Resource(argoAppGVR).Namespace(ArgoCDNamespace).Patch(
		ctx,
		challengeSlug,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		logger.Warning("Failed to patch Argo CD Application '%s' to trigger sync: %v", challengeSlug, err)
	} else {
		logger.Info("Triggered manual sync for Argo CD Application '%s' (ArgoCD operation field)", challengeSlug)
		// Wait for sync to complete using the generalized method
		// ArgoCD handles retry/backoff internally, so we just wait for completion
		return WaitForApplicationStatus(ctx, dynamicClient, challengeSlug, ArgoCDNamespace, WaitOptions{
			CheckSync:   true,
			CheckHealth: false,
		})
	}
	return nil
}

func DeleteChallengeApplication(ctx context.Context, dynamicClient dynamic.Interface, appName, namespace string) error {
	logger.Info("Ensuring ArgoCD Application '%s' in namespace '%s' has the resources-finalizer before deletion...", appName, namespace)

	// Step 1: Get the current Application
	app, err := dynamicClient.Resource(argoAppGVR).Namespace(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("ArgoCD Application '%s' not found in namespace '%s'.", appName, namespace)
			return nil
		}
		return fmt.Errorf("failed to get ArgoCD Application '%s' before patching finalizer: %w", appName, err)
	}

	// Step 2: Merge the finalizer if not already present
	finalizers, found, err := unstructured.NestedStringSlice(app.Object, "metadata", "finalizers")
	if err != nil {
		logger.Warning("Error extracting finalizers: %v", err)
	}
	needPatch := true
	for _, f := range finalizers {
		if f == "resources-finalizer.argocd.argoproj.io/" {
			needPatch = false
			break
		}
	}

	if !found {
		finalizers = []string{"resources-finalizer.argocd.argoproj.io/"}
		needPatch = true
	} else if needPatch {
		finalizers = append(finalizers, "resources-finalizer.argocd.argoproj.io/")
	}

	if needPatch {
		patchObj := map[string]interface{}{
			"metadata": map[string]interface{}{
				"finalizers": finalizers,
			},
		}
		patchBytes, err := json.Marshal(patchObj)
		if err != nil {
			logger.Warning("Error marshaling patch object: %v", err)
			return err
		}
		logger.Debug("Patching finalizers for '%s': %s", appName, string(patchBytes))
		_, err = dynamicClient.Resource(argoAppGVR).Namespace(namespace).Patch(
			ctx,
			appName,
			types.MergePatchType,
			patchBytes,
			metav1.PatchOptions{},
		)
		if err != nil {
			logger.Warning("Failed to patch ArgoCD Application '%s' with finalizer: %v (continuing anyway)", appName, err)
		} else {
			logger.Info("Patched ArgoCD Application '%s' with resources-finalizer.", appName)
		}
		// Wait for patch to be visible
		for i := 0; i < 10; i++ {
			appCheck, err := dynamicClient.Resource(argoAppGVR).Namespace(namespace).Get(ctx, appName, metav1.GetOptions{})
			if err != nil {
				logger.Warning("Error getting ArgoCD Application '%s': %v", appName, err)
				continue
			}
			fList, _, err := unstructured.NestedStringSlice(appCheck.Object, "metadata", "finalizers")
			if err != nil {
				logger.Warning("Error extracting finalizers from appCheck: %v", err)
				continue
			}
			for _, f := range fList {
				if f == "resources-finalizer.argocd.argoproj.io" {
					goto PATCH_DONE
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
	PATCH_DONE:
	}

	logger.Info("Deleting ArgoCD Application '%s' in namespace '%s'...", appName, namespace)
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	err = dynamicClient.Resource(argoAppGVR).Namespace(namespace).Delete(ctx, appName, deleteOptions)
	if err != nil {
		return fmt.Errorf("failed to delete ArgoCD Application '%s': %w", appName, err)
	}

	// Wait for the Application to be fully deleted
	for i := 0; i < 30; i++ {
		_, err := dynamicClient.Resource(argoAppGVR).Namespace(namespace).Get(ctx, appName, metav1.GetOptions{})
		if err != nil {
			logger.Info("ArgoCD Application '%s' deleted.", appName)
			return nil
		}
		logger.Debug("Waiting for ArgoCD Application '%s' to be deleted...", appName)
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timed out waiting for ArgoCD Application '%s' deletion", appName)
}

// WaitOptions defines what conditions to wait for in an ArgoCD Application
type WaitOptions struct {
	CheckHealth bool          // Wait for health status to be "Healthy"
	CheckSync   bool          // Wait for sync status to be "Synced"
	Timeout     time.Duration // Timeout for the wait operation (optional, no timeout if 0)
}

// DefaultWaitOptions returns sensible defaults for waiting
func DefaultWaitOptions() WaitOptions {
	return WaitOptions{
		CheckHealth: true,
		CheckSync:   true,
		Timeout:     0, // No timeout by default, let ArgoCD handle retry/backoff
	}
}

// WaitForApplicationStatus waits for an ArgoCD Application to meet the specified conditions
// This is a unified method that can handle different waiting scenarios
func WaitForApplicationStatus(ctx context.Context, dynamicClient dynamic.Interface, appName, namespace string, options WaitOptions) error {
	// Build description of what we're waiting for
	conditions := []string{}
	if options.CheckHealth {
		conditions = append(conditions, "Healthy")
	}
	if options.CheckSync {
		conditions = append(conditions, "Synced")
	}

	logger.Info("Waiting for ArgoCD Application '%s' to be: %s", appName, strings.Join(conditions, " + "))

	// Setup context with timeout if specified
	waitCtx := ctx
	var cancel context.CancelFunc
	if options.Timeout > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
		logger.Debug("Using timeout: %s", options.Timeout)
	} else {
		logger.Debug("No timeout specified - relying on ArgoCD retry/backoff")
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Log every 10 seconds in detail
	detailedLogTicker := time.NewTicker(10 * time.Second)
	defer detailedLogTicker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			// Get final status for better error reporting
			health, sync, phase, conditions, err := getApplicationFullStatusWithConditions(context.Background(), dynamicClient, appName, namespace)
			if err != nil {
				logger.Error("Timeout waiting for ArgoCD Application '%s'. Failed to get final status: %v", appName, err)
				return fmt.Errorf("timeout waiting for ArgoCD Application '%s'", appName)
			}
			logger.Error("Timeout waiting for ArgoCD Application '%s'. Final status: health=%s, sync=%s, phase=%s, conditions=%s", appName, health, sync, phase, conditions)
			return fmt.Errorf("timeout waiting for ArgoCD Application '%s' (final status: health=%s, sync=%s, phase=%s, conditions=%s)", appName, health, sync, phase, conditions)

		case <-detailedLogTicker.C:
			// Detailed logging every 10 seconds
			health, sync, phase, conditions, err := getApplicationFullStatusWithConditions(waitCtx, dynamicClient, appName, namespace)
			if err != nil {
				logger.Warning("Failed to get ArgoCD Application '%s' detailed status: %v", appName, err)
			} else {
				logger.Info("ArgoCD Application '%s' detailed status: health=%s, sync=%s, phase=%s, conditions=%s", appName, health, sync, phase, conditions)
			}

		case <-ticker.C:
			health, sync, phase, err := getApplicationFullStatus(waitCtx, dynamicClient, appName, namespace)
			if err != nil {
				logger.Warning("Failed to get ArgoCD Application '%s' status: %v (retrying...)", appName, err)
				continue
			}

			logger.Debug("ArgoCD Application '%s' status: health=%s, sync=%s, phase=%s", appName, health, sync, phase)

			// Check all required conditions
			ready := true
			if options.CheckHealth && health != "Healthy" {
				ready = false
			}
			if options.CheckSync && sync != syncedStatus {
				ready = false
			}

			if ready {
				logger.Info("ArgoCD Application '%s' meets all required conditions: %s", appName, strings.Join(conditions, " + "))
				return nil
			}

			// Log current status for visibility
			logger.Debug("Waiting for ArgoCD Application '%s'... (health=%s, sync=%s, phase=%s)", appName, health, sync, phase)
		}
	}
}

// getApplicationFullStatus retrieves the health, sync status and operation phase of an ArgoCD Application
func getApplicationFullStatus(ctx context.Context, dynamicClient dynamic.Interface, appName, namespace string) (health string, sync string, phase string, err error) {
	app, err := dynamicClient.Resource(argoAppGVR).Namespace(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get ArgoCD Application '%s': %w", appName, err)
	}

	// Extract health status
	health, _, err = unstructured.NestedString(app.Object, "status", "health", "status")
	if err != nil {
		return "", "", "", fmt.Errorf("error extracting health status: %w", err)
	}

	// Extract sync status
	sync, _, err = unstructured.NestedString(app.Object, "status", "sync", "status")
	if err != nil {
		return health, "", "", fmt.Errorf("error extracting sync status: %w", err)
	}

	// Extract operation phase (may not exist if no operation is running)
	phase, _, _ = unstructured.NestedString(app.Object, "status", "operationState", "phase")
	// Don't return error for phase as it may not exist

	return health, sync, phase, nil
}

// getApplicationFullStatusWithConditions retrieves the full status including error conditions
func getApplicationFullStatusWithConditions(ctx context.Context, dynamicClient dynamic.Interface, appName, namespace string) (health string, sync string, phase string, conditions string, err error) {
	app, err := dynamicClient.Resource(argoAppGVR).Namespace(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to get ArgoCD Application '%s': %w", appName, err)
	}

	// Extract health status
	health, _, _ = unstructured.NestedString(app.Object, "status", "health", "status")

	// Extract sync status
	sync, _, _ = unstructured.NestedString(app.Object, "status", "sync", "status")

	// Extract operation phase (may not exist if no operation is running)
	phase, _, _ = unstructured.NestedString(app.Object, "status", "operationState", "phase")

	// Extract conditions (error messages)
	conditionsList, found, _ := unstructured.NestedSlice(app.Object, "status", "conditions")
	if found && len(conditionsList) > 0 {
		var conditionsStrings []string
		for _, cond := range conditionsList {
			condMap, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _, _ := unstructured.NestedString(condMap, "type")
			message, _, _ := unstructured.NestedString(condMap, "message")
			if condType != "" && message != "" {
				conditionsStrings = append(conditionsStrings, fmt.Sprintf("%s: %s", condType, message))
			}
		}
		conditions = strings.Join(conditionsStrings, "; ")
	} else {
		conditions = "none"
	}

	return health, sync, phase, conditions, nil
}
