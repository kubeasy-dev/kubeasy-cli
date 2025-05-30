package argocd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/constants"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/logger"
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
					"repoURL":        constants.ExercisesRepoUrl,
					"path":           challengeSlug,
					"targetRevision": "HEAD",
					"directory": map[string]interface{}{
						"recurse": true,
						"include": "{manifests/**.yaml,dynamic/**.yaml,static/**.yaml,policies/**.yaml}",
					},
				},
				"destination": map[string]interface{}{
					"server":    "https://kubernetes.default.svc",
					"namespace": challengeSlug,
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"prune":    false,
						"selfHeal": false,
					},
					"syncOptions": []string{
						"CreateNamespace=true",
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
	return nil
}

func DeleteChallengeApplication(ctx context.Context, dynamicClient dynamic.Interface, appName, namespace string) error {
	logger.Info("Ensuring ArgoCD Application '%s' in namespace '%s' has the resources-finalizer before deletion...", appName, namespace)

	// Step 1: Get the current Application
	app, err := dynamicClient.Resource(argoAppGVR).Namespace(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ArgoCD Application '%s' before patching finalizer: %w", appName, err)
	}

	// Step 2: Merge the finalizer if not already present
	finalizers, found, _ := unstructured.NestedStringSlice(app.Object, "metadata", "finalizers")
	needPatch := true
	for _, f := range finalizers {
		if f == "resources-finalizer.argocd.argoproj.io" {
			needPatch = false
			break
		}
	}

	if !found {
		finalizers = []string{"resources-finalizer.argocd.argoproj.io"}
		needPatch = true
	} else if needPatch {
		finalizers = append(finalizers, "resources-finalizer.argocd.argoproj.io")
	}

	if needPatch {
		patchObj := map[string]interface{}{
			"metadata": map[string]interface{}{
				"finalizers": finalizers,
			},
		}
		patchBytes, _ := json.Marshal(patchObj)
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
			appCheck, _ := dynamicClient.Resource(argoAppGVR).Namespace(namespace).Get(ctx, appName, metav1.GetOptions{})
			fList, _, _ := unstructured.NestedStringSlice(appCheck.Object, "metadata", "finalizers")
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
