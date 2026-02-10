package cmd

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/api"
	"github.com/kubeasy-dev/kubeasy-cli/internal/deployer"
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

// deleteChallengeResources deletes all resources for a challenge
func deleteChallengeResources(ctx context.Context, challengeSlug string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	ui.Println()

	// Get Kubernetes clientset
	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		ui.Error("Failed to get Kubernetes clientset")
		return fmt.Errorf("failed to get Kubernetes clientset: %w", err)
	}

	// Delete namespace and restore context
	err = ui.TimedSpinner("Deleting challenge resources", func() error {
		return deployer.CleanupChallenge(ctx, clientset, challengeSlug)
	})
	if err != nil {
		ui.Error("Failed to delete challenge resources")
		return fmt.Errorf("failed to delete challenge resources: %w", err)
	}

	ui.Success("Challenge resources deleted")
	return nil
}
