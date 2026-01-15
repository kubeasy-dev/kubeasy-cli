package cmd

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/api"
	"github.com/kubeasy-dev/kubeasy-cli/internal/argocd"
	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
)

// validateChallengeSlug validates that a challenge slug has the correct format
func validateChallengeSlug(slug string) error {
	// Challenge slugs should be lowercase alphanumeric with hyphens
	// Example: "basic-pod", "deployment-rollout", "config-map-101"
	if !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(slug) {
		return fmt.Errorf("invalid challenge slug format: '%s' (must be lowercase alphanumeric with hyphens)", slug)
	}
	if len(slug) < 3 || len(slug) > 63 {
		return fmt.Errorf("invalid challenge slug length: '%s' (must be between 3 and 63 characters)", slug)
	}
	return nil
}

// getChallenge tries to get a challenge and returns an error if it fails
func getChallenge(slug string) (*api.ChallengeEntity, error) {
	if err := validateChallengeSlug(slug); err != nil {
		return nil, err
	}

	challenge, err := api.GetChallenge(slug)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch challenge: %w", err)
	}
	if challenge == nil {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}
	return challenge, nil
}

// deleteChallengeResources deletes ArgoCD Application and all subresources for a challenge
func deleteChallengeResources(ctx context.Context, challengeSlug string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	ui.Println()

	// Step 1: Delete ArgoCD Application
	dynamicClient, err := kube.GetDynamicClient()
	if err != nil {
		ui.Error("Failed to get Kubernetes dynamic client")
		return fmt.Errorf("failed to get Kubernetes dynamic client: %w", err)
	}

	err = ui.TimedSpinner("Deleting ArgoCD application", func() error {
		return argocd.DeleteChallengeApplication(ctx, dynamicClient, challengeSlug, argocd.ArgoCDNamespace)
	})
	if err != nil {
		ui.Error("Failed to delete ArgoCD application")
		return fmt.Errorf("failed to delete ArgoCD application: %w", err)
	}

	// Step 2: Delete namespace
	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		ui.Error("Failed to get Kubernetes clientset")
		return fmt.Errorf("failed to get Kubernetes clientset: %w", err)
	}

	err = ui.TimedSpinner("Deleting namespace", func() error {
		return kube.DeleteNamespace(ctx, clientset, challengeSlug)
	})
	if err != nil {
		ui.Error("Failed to delete namespace")
		return fmt.Errorf("failed to delete namespace: %w", err)
	}

	// Step 3: Switch to default namespace
	if err := kube.SetNamespaceForContext(constants.KubeasyClusterContext, "default"); err != nil {
		ui.Error("Failed to switch to default namespace")
		return fmt.Errorf("failed to switch to default namespace: %w", err)
	}
	ui.Success("Switched to default namespace")

	return nil
}
