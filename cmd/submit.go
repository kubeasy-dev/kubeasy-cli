package cmd

import (
	"fmt"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/kube"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/ui"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/validation"
	"github.com/spf13/cobra"
)

var submitCmd = &cobra.Command{
	Use:   "submit [challenge-slug]",
	Short: "Submit a challenge solution",
	Long: `Submit a challenge solution to Kubeasy. This command will run validations
against your cluster and send the results to the Kubeasy API for evaluation.
Make sure you have completed the challenge before submitting.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		ui.Section(fmt.Sprintf("Submitting Challenge: %s", challengeSlug))

		// Verify challenge exists
		err := ui.WaitMessage("Verifying challenge", func() error {
			_, err := api.GetChallenge(challengeSlug)
			return err
		})
		if err != nil {
			ui.Error("Failed to fetch challenge")
			return fmt.Errorf("failed to fetch challenge: %w", err)
		}

		// Check progress
		var progress *api.ChallengeStatusResponse
		err = ui.WaitMessage("Checking progress", func() error {
			var err error
			progress, err = api.GetChallengeProgress(challengeSlug)
			return err
		})
		if err != nil {
			ui.Error("Failed to fetch challenge progress")
			return fmt.Errorf("failed to fetch challenge progress: %w", err)
		}

		if progress == nil {
			ui.Error("Challenge not started")
			ui.Info("Please start the challenge first with 'kubeasy challenge start " + challengeSlug + "'")
			return nil
		}

		if progress.Status == "completed" {
			ui.Warning("Challenge already completed")
			ui.Info("You can reset the challenge with 'kubeasy challenge reset " + challengeSlug + "'")
			return nil
		}

		// Load validations from challenges repo
		var config *validation.ValidationConfig
		err = ui.WaitMessage("Loading validations", func() error {
			var loadErr error
			config, loadErr = validation.LoadForChallenge(challengeSlug)
			return loadErr
		})
		if err != nil {
			ui.Error("Failed to load validations")
			return fmt.Errorf("failed to load validations: %w", err)
		}

		if len(config.Validations) == 0 {
			ui.Warning("No validations found for this challenge")
			return nil
		}

		// Get Kubernetes clients
		clientset, err := kube.GetKubernetesClient()
		if err != nil {
			ui.Error("Failed to get Kubernetes client")
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

		// Display results grouped by type
		allPassed := true
		var apiResults []api.ObjectiveResult

		// Group validations by type for display
		typeResults := make(map[validation.ValidationType][]validation.Result)
		for i, v := range config.Validations {
			typeResults[v.Type] = append(typeResults[v.Type], results[i])
		}

		typeLabels := map[validation.ValidationType]string{
			validation.TypeStatus:       "Status Validation",
			validation.TypeCondition:    "Condition Validation",
			validation.TypeLog:          "Log Validation",
			validation.TypeEvent:        "Event Validation",
			validation.TypeConnectivity: "Connectivity Validation",
		}

		for valType, typeRes := range typeResults {
			ui.Section(typeLabels[valType])
			for _, r := range typeRes {
				ui.ValidationResult(r.Key, r.Passed, []string{r.Message})
				if !r.Passed {
					allPassed = false
				}

				// Convert to API result
				msg := r.Message
				apiResults = append(apiResults, api.ObjectiveResult{
					ObjectiveKey: r.Key,
					Passed:       r.Passed,
					Message:      &msg,
				})
			}
			ui.Println()
		}

		// Display overall result
		ui.Section("Submission Result")

		if allPassed {
			ui.Success("All validations passed!")
			ui.Info("Sending results to server...")
			err = api.SendSubmit(challengeSlug, apiResults)
			if err == nil {
				ui.Println()
				ui.Success(fmt.Sprintf("Congratulations! Challenge '%s' completed!", challengeSlug))
				ui.Info("You can clean up with 'kubeasy challenge clean " + challengeSlug + "'")
			}
		} else {
			ui.Error("Some validations failed")
			ui.Info("Review the results above and try again")
			err = api.SendSubmit(challengeSlug, apiResults)
		}

		if err != nil {
			ui.Error("Failed to submit results")
			return fmt.Errorf("failed to submit results: %w", err)
		}

		return nil
	},
}

func init() {
	challengeCmd.AddCommand(submitCmd)
}
