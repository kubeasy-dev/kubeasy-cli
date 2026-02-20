package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/deployer"
	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
	"github.com/spf13/cobra"
)

// runDevApply deploys local challenge manifests to the Kind cluster.
// If clean is true, it deletes existing resources before applying.
func runDevApply(cmd *cobra.Command, challengeSlug, challengeDir string, clean bool) error {
	// Clean existing resources if requested
	if clean {
		ui.Info("Cleaning existing resources before apply...")
		if err := deleteChallengeResources(cmd.Context(), challengeSlug); err != nil {
			ui.Warning(fmt.Sprintf("Clean failed (namespace may not exist yet): %v", err))
		}
	}

	// Get Kubernetes clients
	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		ui.Error("Failed to get Kubernetes client. Is the cluster running? Try 'kubeasy setup'")
		return fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	dynamicClient, err := kube.GetDynamicClient()
	if err != nil {
		ui.Error("Failed to get dynamic client")
		return fmt.Errorf("failed to get dynamic client: %w", err)
	}

	// Build and load Docker image if image/ directory exists
	if deployer.HasImageDir(challengeDir) {
		imageDir := filepath.Join(challengeDir, "image")
		imageTag := challengeSlug + ":latest"
		ui.Info(fmt.Sprintf("Detected image/ directory, building '%s'...", imageTag))

		err = ui.TimedSpinner("Building and loading Docker image", func() error {
			return deployer.BuildAndLoadImage(cmd.Context(), imageDir, imageTag, constants.KubeasyClusterName)
		})
		if err != nil {
			ui.Error("Failed to build/load Docker image")
			return fmt.Errorf("failed to build/load Docker image: %w", err)
		}
	}

	// Create namespace
	err = ui.WaitMessage("Creating namespace", func() error {
		return kube.CreateNamespace(cmd.Context(), clientset, challengeSlug)
	})
	if err != nil {
		ui.Error("Failed to create namespace")
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	// Deploy local manifests
	err = ui.TimedSpinner("Deploying challenge manifests", func() error {
		return deployer.DeployLocalChallenge(cmd.Context(), clientset, dynamicClient, challengeDir, challengeSlug)
	})
	if err != nil {
		ui.Error("Failed to deploy challenge")
		return fmt.Errorf("failed to deploy challenge: %w", err)
	}

	// Set kubectl context
	if err := kube.SetNamespaceForContext(constants.KubeasyClusterContext, challengeSlug); err != nil {
		ui.Warning(fmt.Sprintf("Failed to set kubectl context namespace: %v", err))
	}

	return nil
}

// runDevValidate runs validations against the cluster and displays results.
// Returns true if all validations passed.
func runDevValidate(cmd *cobra.Command, challengeSlug, challengeDir string) (bool, error) {
	// Load validations from local challenge.yaml
	challengeYAML := filepath.Join(challengeDir, "challenge.yaml")
	var config *validation.ValidationConfig
	err := ui.WaitMessage("Loading validations", func() error {
		var loadErr error
		config, loadErr = validation.LoadFromFile(challengeYAML)
		return loadErr
	})
	if err != nil {
		ui.Error("Failed to load validations")
		return false, fmt.Errorf("failed to load validations from %s: %w", challengeYAML, err)
	}

	if len(config.Validations) == 0 {
		ui.Warning("No validations (objectives) defined in challenge.yaml")
		ui.Info("Add objectives to your challenge.yaml to test validations")
		return true, nil
	}

	// Get Kubernetes clients
	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		ui.Error("Failed to get Kubernetes client. Is the cluster running? Try 'kubeasy setup'")
		return false, fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	dynamicClient, err := kube.GetDynamicClient()
	if err != nil {
		ui.Error("Failed to get dynamic client")
		return false, fmt.Errorf("failed to get dynamic client: %w", err)
	}

	restConfig, err := kube.GetRestConfig()
	if err != nil {
		ui.Error("Failed to get REST config")
		return false, fmt.Errorf("failed to get REST config: %w", err)
	}

	namespace := challengeSlug

	// Create executor and run validations
	executor := validation.NewExecutor(clientset, dynamicClient, restConfig, namespace)

	ui.Info("Running validations...")
	ui.Println()

	results := executor.ExecuteAll(cmd.Context(), config.Validations)

	// Display results
	allPassed := devutils.DisplayValidationResults(config.Validations, results)

	// Display overall result
	ui.Section("Validation Result")
	if allPassed {
		ui.Success("All validations passed!")
	} else {
		ui.Error("Some validations failed")
		ui.Info("Review the results above and try again")
	}

	return allPassed, nil
}
