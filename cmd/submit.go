package cmd

import (
	"fmt"

	operator "github.com/kubeasy-dev/challenge-operator/api/v1alpha1"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/kube"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/ui"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var submitCmd = &cobra.Command{
	Use:   "submit [challenge-slug]",
	Short: "Submit a challenge solution",
	Long: `Submit a challenge solution to Kubeasy. This command will package your solution
and send it to the Kubeasy API for evaluation. Make sure you have completed the challenge before submitting.`,
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

		// Get Kubernetes client
		dynamicClient, err := kube.GetDynamicClient()
		if err != nil {
			ui.Error("Failed to get Kubernetes client")
			return fmt.Errorf("failed to get Kubernetes client: %w", err)
		}

		svGVR := schema.GroupVersionResource{
			Group:    "challenge.kubeasy.dev",
			Version:  "v1alpha1",
			Resource: "staticvalidations",
		}

		dvGVR := schema.GroupVersionResource{
			Group:    "challenge.kubeasy.dev",
			Version:  "v1alpha1",
			Resource: "dynamicvalidations",
		}

		namespace := challengeSlug

		// Fetch validations
		ui.Info("Reading validation results...")

		svListUnstructured, err := dynamicClient.Resource(svGVR).Namespace(namespace).List(cmd.Context(), metav1.ListOptions{})
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to list StaticValidations in namespace %s", namespace))
			return fmt.Errorf("failed to list StaticValidations in namespace %s: %w", namespace, err)
		}

		if len(svListUnstructured.Items) == 0 {
			ui.Warning("No StaticValidations found")
			ui.Info("Cannot verify submission without validation resources")
			return nil
		}

		allStaticSucceeded := true
		allDynamicSucceeded := true
		detailedStatuses := map[string]interface{}{
			"staticValidations":  map[string][]operator.StaticValidationStatus{},
			"dynamicValidations": map[string][]operator.DynamicValidationStatus{},
		}

		ui.Println()
		ui.Section("Static Validations")

		// Process static validations
		for _, svUnstructured := range svListUnstructured.Items {
			var sv operator.StaticValidation
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(svUnstructured.Object, &sv)
			if err != nil {
				ui.Error("Failed to convert StaticValidation")
				return fmt.Errorf("failed to convert StaticValidation: %w", err)
			}

			staticStatuses := detailedStatuses["staticValidations"].(map[string][]operator.StaticValidationStatus)
			staticStatuses[svUnstructured.GetName()] = []operator.StaticValidationStatus{sv.Status}

			// Display validation result
			validationDetails := []string{}
			for _, resource := range sv.Status.Resources {
				for _, ruleResult := range resource.RuleResults {
					detail := fmt.Sprintf("%s: %s", ruleResult.Rule, ruleResult.Status)
					if ruleResult.Message != "" {
						detail += fmt.Sprintf(" - %s", ruleResult.Message)
					}
					validationDetails = append(validationDetails, detail)
				}
			}

			ui.ValidationResult(svUnstructured.GetName(), sv.Status.AllPassed, validationDetails)

			if !sv.Status.AllPassed {
				allStaticSucceeded = false
			}
		}

		ui.Println()
		ui.Section("Dynamic Validations")

		// Process dynamic validations
		dvListUnstructured, err := dynamicClient.Resource(dvGVR).Namespace(namespace).List(cmd.Context(), metav1.ListOptions{})
		if err != nil {
			ui.Warning("Failed to list DynamicValidations")
			allDynamicSucceeded = false
		} else {
			if len(dvListUnstructured.Items) > 0 {
				for _, dvUnstructured := range dvListUnstructured.Items {
					var dv operator.DynamicValidation
					err = runtime.DefaultUnstructuredConverter.FromUnstructured(dvUnstructured.Object, &dv)
					if err != nil {
						ui.Warning("Failed to convert DynamicValidation")
						allDynamicSucceeded = false
						continue
					}

					dynamicStatuses := detailedStatuses["dynamicValidations"].(map[string][]operator.DynamicValidationStatus)
					dynamicStatuses[dvUnstructured.GetName()] = []operator.DynamicValidationStatus{dv.Status}

					// Display validation result
					validationDetails := []string{}
					for _, resource := range dv.Status.Resources {
						for _, checkResult := range resource.CheckResults {
							detail := fmt.Sprintf("%s: %s", checkResult.Kind, checkResult.Status)
							if checkResult.Message != "" {
								detail += fmt.Sprintf(" - %s", checkResult.Message)
							}
							validationDetails = append(validationDetails, detail)
						}
					}

					ui.ValidationResult(dvUnstructured.GetName(), dv.Status.AllPassed, validationDetails)

					if !dv.Status.AllPassed {
						allDynamicSucceeded = false
					}
				}
			} else {
				ui.Info("No dynamic validations defined for this challenge")
			}
		}

		// Display overall result
		ui.Println()
		ui.Section("Submission Result")

		if allStaticSucceeded && allDynamicSucceeded {
			ui.Success("All validations passed!")
			ui.Info("Sending results to server...")
			err = api.SendSubmit(challengeSlug, true, true, detailedStatuses)
			if err == nil {
				ui.Println()
				ui.Success(fmt.Sprintf("Congratulations! Challenge '%s' completed!", challengeSlug))
				ui.Info("You can clean up with 'kubeasy challenge clean " + challengeSlug + "'")
			}
		} else if allStaticSucceeded && !allDynamicSucceeded {
			ui.Success("Static validations passed")
			ui.Error("Some dynamic validations failed")
			err = api.SendSubmit(challengeSlug, true, false, detailedStatuses)
		} else if !allStaticSucceeded && allDynamicSucceeded {
			ui.Error("Some static validations failed")
			ui.Success("Dynamic validations passed")
			err = api.SendSubmit(challengeSlug, false, true, detailedStatuses)
		} else {
			ui.Error("Some validations failed")
			ui.Info("Review the results above and try again")
			err = api.SendSubmit(challengeSlug, false, false, detailedStatuses)
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
