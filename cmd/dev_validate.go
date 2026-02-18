package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
	"github.com/spf13/cobra"
)

var devValidateDir string

var devValidateCmd = &cobra.Command{
	Use:   "validate [challenge-slug]",
	Short: "Run validations locally without submitting to API",
	Long: `Runs challenge validations against the Kind cluster and displays results.
This is the dev equivalent of 'kubeasy challenge submit' but does not send
results to the Kubeasy API. No login required.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		ui.Section(fmt.Sprintf("Validating Dev Challenge: %s", challengeSlug))

		// Validate slug format
		if err := validateChallengeSlug(challengeSlug); err != nil {
			ui.Error("Invalid challenge slug")
			return err
		}

		// Resolve local challenge directory
		challengeDir, err := devutils.ResolveLocalChallengeDir(challengeSlug, devValidateDir)
		if err != nil {
			ui.Error("Failed to find challenge directory")
			return err
		}

		// Load validations from local challenge.yaml
		challengeYAML := filepath.Join(challengeDir, "challenge.yaml")
		var config *validation.ValidationConfig
		err = ui.WaitMessage("Loading validations", func() error {
			var loadErr error
			config, loadErr = validation.LoadFromFile(challengeYAML)
			return loadErr
		})
		if err != nil {
			ui.Error("Failed to load validations")
			return fmt.Errorf("failed to load validations from %s: %w", challengeYAML, err)
		}

		if len(config.Validations) == 0 {
			ui.Warning("No validations (objectives) defined in challenge.yaml")
			ui.Info("Add objectives to your challenge.yaml to test validations")
			return nil
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

		restConfig, err := kube.GetRestConfig()
		if err != nil {
			ui.Error("Failed to get REST config")
			return fmt.Errorf("failed to get REST config: %w", err)
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

		return nil
	},
}

func init() {
	devCmd.AddCommand(devValidateCmd)
	devValidateCmd.Flags().StringVar(&devValidateDir, "dir", "", "Path to challenge directory (default: auto-detect)")
}
