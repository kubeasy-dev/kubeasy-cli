package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/deployer"
	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
	"github.com/spf13/cobra"
)

// DevValidateOpts holds options for dev validate runs.
type DevValidateOpts struct {
	FailFast   bool
	JSONOutput bool
}

// runDevApply deploys challenge manifests to the Kind cluster.
// When challengeDir is empty it fetches manifests from registryURL (registry mode).
// When challengeDir is set it reads from the local filesystem (--dir override).
// Returns the manifest content hash (only meaningful in registry mode; empty in filesystem mode).
func runDevApply(cmd *cobra.Command, challengeSlug, challengeDir, registryURL string, clean bool) (string, error) {
	if clean {
		ui.Info("Cleaning existing resources before apply...")
		if err := deleteChallengeResources(cmd.Context(), challengeSlug); err != nil {
			ui.Warning(fmt.Sprintf("Clean failed (namespace may not exist yet): %v", err))
		}
	}

	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		ui.Error("Failed to get Kubernetes client. Is the cluster running? Try 'kubeasy setup'")
		return "", fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	dynamicClient, err := kube.GetDynamicClient()
	if err != nil {
		ui.Error("Failed to get dynamic client")
		return "", fmt.Errorf("failed to get dynamic client: %w", err)
	}

	err = ui.WaitMessage("Creating namespace", func() error {
		return kube.CreateNamespace(cmd.Context(), clientset, challengeSlug)
	})
	if err != nil {
		ui.Error("Failed to create namespace")
		return "", fmt.Errorf("failed to create namespace: %w", err)
	}

	if challengeDir != "" {
		// Filesystem mode: build custom image if present, then deploy local manifests.
		if deployer.HasImageDir(challengeDir) {
			imageDir := filepath.Join(challengeDir, "image")
			imageTag := challengeSlug + ":latest"
			ui.Info(fmt.Sprintf("Detected image/ directory, building '%s'...", imageTag))
			err = ui.TimedSpinner("Building and loading Docker image", func() error {
				return deployer.BuildAndLoadImage(cmd.Context(), imageDir, imageTag, constants.KubeasyClusterName)
			})
			if err != nil {
				ui.Error("Failed to build/load Docker image")
				return "", fmt.Errorf("failed to build/load Docker image: %w", err)
			}
		}

		err = ui.TimedSpinner("Deploying challenge manifests", func() error {
			return deployer.DeployLocalChallenge(cmd.Context(), clientset, dynamicClient, challengeDir, challengeSlug)
		})
		if err != nil {
			ui.Error("Failed to deploy challenge")
			return "", fmt.Errorf("failed to deploy challenge: %w", err)
		}

		if err := kube.SetNamespaceForContext(constants.KubeasyClusterContext, challengeSlug); err != nil {
			ui.Warning(fmt.Sprintf("Failed to set kubectl context namespace: %v", err))
		}
		return "", nil
	}

	// Registry mode: fetch tar.gz from local registry and apply.
	var hash string
	err = ui.TimedSpinner("Deploying challenge manifests", func() error {
		var deployErr error
		hash, deployErr = deployer.DeployChallengeFromRegistry(cmd.Context(), clientset, dynamicClient, registryURL, challengeSlug)
		return deployErr
	})
	if err != nil {
		ui.Error("Failed to deploy challenge from registry")
		return "", fmt.Errorf("failed to deploy challenge from registry: %w", err)
	}

	if err := kube.SetNamespaceForContext(constants.KubeasyClusterContext, challengeSlug); err != nil {
		ui.Warning(fmt.Sprintf("Failed to set kubectl context namespace: %v", err))
	}

	return hash, nil
}

// runDevValidate runs validations against the cluster and displays results.
// When challengeDir is empty it loads the challenge YAML from registryURL (registry mode).
// When challengeDir is set it reads from the local filesystem (--dir override).
// Returns true if all validations passed.
func runDevValidate(cmd *cobra.Command, challengeSlug, challengeDir, registryURL string, opts DevValidateOpts) (bool, error) {
	var config *validation.ValidationConfig

	loadConfig := func() error {
		if challengeDir != "" {
			var err error
			config, err = validation.LoadFromFile(filepath.Join(challengeDir, "challenge.yaml"))
			return err
		}
		var err error
		config, err = validation.LoadForChallengeFromRegistryURL(registryURL, challengeSlug)
		return err
	}

	if !opts.JSONOutput {
		err := ui.WaitMessage("Loading validations", loadConfig)
		if err != nil {
			ui.Error("Failed to load validations")
			return false, fmt.Errorf("failed to load validations: %w", err)
		}
	} else {
		if err := loadConfig(); err != nil {
			return false, fmt.Errorf("failed to load validations: %w", err)
		}
	}

	if len(config.Validations) == 0 {
		if opts.JSONOutput {
			out := devutils.FormatValidationJSON(challengeSlug, config.Validations, nil, 0)
			data, err := json.Marshal(out)
			if err != nil {
				return false, fmt.Errorf("failed to serialize JSON output: %w", err)
			}
			fmt.Println(string(data))
			return true, nil
		}
		ui.Warning("No validations (objectives) defined in challenge.yaml")
		ui.Info("Add objectives to your challenge.yaml to test validations")
		return true, nil
	}

	// Get Kubernetes clients
	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		if !opts.JSONOutput {
			ui.Error("Failed to get Kubernetes client. Is the cluster running? Try 'kubeasy setup'")
		}
		return false, fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	dynamicClient, err := kube.GetDynamicClient()
	if err != nil {
		if !opts.JSONOutput {
			ui.Error("Failed to get dynamic client")
		}
		return false, fmt.Errorf("failed to get dynamic client: %w", err)
	}

	restConfig, err := kube.GetRestConfig()
	if err != nil {
		if !opts.JSONOutput {
			ui.Error("Failed to get REST config")
		}
		return false, fmt.Errorf("failed to get REST config: %w", err)
	}

	namespace := challengeSlug

	// Create executor and run validations
	executor := validation.NewExecutor(clientset, dynamicClient, restConfig, namespace)

	if !opts.JSONOutput {
		ui.Info("Running validations...")
		ui.Println()
	}

	totalStart := time.Now()
	var results []validation.Result
	if opts.FailFast {
		results = executor.ExecuteSequential(cmd.Context(), config.Validations, true)
	} else {
		results = executor.ExecuteAll(cmd.Context(), config.Validations)
	}
	totalDuration := time.Since(totalStart)

	if opts.JSONOutput {
		out := devutils.FormatValidationJSON(challengeSlug, config.Validations, results, totalDuration)
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return false, fmt.Errorf("failed to serialize JSON output: %w", err)
		}
		fmt.Println(string(data))
		allPassed := true
		for _, r := range results {
			if !r.Passed {
				allPassed = false
				break
			}
		}
		return allPassed, nil
	}

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
