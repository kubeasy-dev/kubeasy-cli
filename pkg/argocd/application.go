package argocd

import (
	"context"
	"fmt"

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
