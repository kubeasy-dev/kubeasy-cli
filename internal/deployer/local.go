package deployer

import (
	"context"
	"fmt"

	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
)

// DeployLocalChallenge applies manifests from a local challenge directory to the cluster.
// Unlike DeployChallenge, it reads from the local filesystem instead of pulling from OCI.
func DeployLocalChallenge(ctx context.Context, clientset *kubernetes.Clientset, dynamicClient dynamic.Interface, challengeDir string, namespace string) error {
	logger.Info("Deploying local challenge from '%s'...", challengeDir)

	// Build REST mapper from API discovery
	groups, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		return fmt.Errorf("failed to discover API resources: %w", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groups)

	// Find and apply YAML files from manifests/ and policies/
	if err := applyManifestDirs(ctx, challengeDir, namespace, mapper, dynamicClient); err != nil {
		return err
	}

	// Wait for Deployments and StatefulSets to be ready
	logger.Info("Waiting for challenge resources to be ready...")
	if err := WaitForChallengeReady(ctx, clientset, namespace); err != nil {
		return fmt.Errorf("challenge resources failed to become ready: %w", err)
	}

	logger.Info("Local challenge deployed successfully.")
	return nil
}
