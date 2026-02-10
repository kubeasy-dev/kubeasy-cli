package deployer

import (
	"context"
	"fmt"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"k8s.io/client-go/kubernetes"
)

// CleanupChallenge deletes the challenge namespace and restores the kubectl context.
func CleanupChallenge(ctx context.Context, clientset kubernetes.Interface, slug string) error {
	logger.Info("Cleaning up challenge '%s'...", slug)

	// Delete the namespace (cascades to all namespaced resources)
	if err := kube.DeleteNamespace(ctx, clientset, slug); err != nil {
		return fmt.Errorf("failed to delete namespace '%s': %w", slug, err)
	}

	// Restore kubectl context to default namespace
	if err := kube.SetNamespaceForContext(constants.KubeasyClusterContext, "default"); err != nil {
		return fmt.Errorf("failed to switch to default namespace: %w", err)
	}

	logger.Info("Challenge '%s' cleaned up successfully.", slug)
	return nil
}
