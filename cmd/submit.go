package cmd

import (
	"fmt"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/kube"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/ui"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ValidationType represents a Kubeasy validation CRD type
type ValidationType struct {
	Name     string
	Resource string
	GVR      schema.GroupVersionResource
}

// All supported validation types
var validationTypes = []ValidationType{
	{
		Name:     "Log Validation",
		Resource: "logvalidations",
		GVR: schema.GroupVersionResource{
			Group:    "challenge.kubeasy.dev",
			Version:  "v1alpha1",
			Resource: "logvalidations",
		},
	},
	{
		Name:     "Status Validation",
		Resource: "statusvalidations",
		GVR: schema.GroupVersionResource{
			Group:    "challenge.kubeasy.dev",
			Version:  "v1alpha1",
			Resource: "statusvalidations",
		},
	},
	{
		Name:     "Event Validation",
		Resource: "eventvalidations",
		GVR: schema.GroupVersionResource{
			Group:    "challenge.kubeasy.dev",
			Version:  "v1alpha1",
			Resource: "eventvalidations",
		},
	},
	{
		Name:     "Metrics Validation",
		Resource: "metricsvalidations",
		GVR: schema.GroupVersionResource{
			Group:    "challenge.kubeasy.dev",
			Version:  "v1alpha1",
			Resource: "metricsvalidations",
		},
	},
	{
		Name:     "RBAC Validation",
		Resource: "rbacvalidations",
		GVR: schema.GroupVersionResource{
			Group:    "challenge.kubeasy.dev",
			Version:  "v1alpha1",
			Resource: "rbacvalidations",
		},
	},
	{
		Name:     "Connectivity Validation",
		Resource: "connectivityvalidations",
		GVR: schema.GroupVersionResource{
			Group:    "challenge.kubeasy.dev",
			Version:  "v1alpha1",
			Resource: "connectivityvalidations",
		},
	},
}

// ValidationResult holds the result of a single validation instance
type ValidationResult struct {
	Name       string
	Type       string
	AllPassed  bool
	Details    []string
	RawStatus  map[string]interface{}
}

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

		namespace := challengeSlug

		// Fetch validations for all types
		ui.Info("Reading validation results...")

		var allResults []ValidationResult
		allPassed := true
		foundValidations := false

		// Iterate through all validation types
		for _, valType := range validationTypes {
			results, err := fetchValidationsOfType(cmd.Context(), dynamicClient, namespace, valType)
			if err != nil {
				ui.Warning(fmt.Sprintf("Failed to list %s: %v", valType.Name, err))
				continue
			}

			if len(results) > 0 {
				foundValidations = true
				allResults = append(allResults, results...)

				// Display results for this type
				ui.Println()
				ui.Section(valType.Name)
				for _, result := range results {
					ui.ValidationResult(result.Name, result.AllPassed, result.Details)
					if !result.AllPassed {
						allPassed = false
					}
				}
			}
		}

		if !foundValidations {
			ui.Warning("No validations found")
			ui.Info("Cannot verify submission without validation resources")
			return nil
		}

		// Build validation results payload
		validations := buildValidationPayload(allResults)

		// Display overall result
		ui.Println()
		ui.Section("Submission Result")

		if allPassed {
			ui.Success("All validations passed!")
			ui.Info("Sending results to server...")
			err = api.SendSubmit(challengeSlug, validations)
			if err == nil {
				ui.Println()
				ui.Success(fmt.Sprintf("Congratulations! Challenge '%s' completed!", challengeSlug))
				ui.Info("You can clean up with 'kubeasy challenge clean " + challengeSlug + "'")
			}
		} else {
			ui.Error("Some validations failed")
			ui.Info("Review the results above and try again")
			err = api.SendSubmit(challengeSlug, validations)
		}

		if err != nil {
			ui.Error("Failed to submit results")
			return fmt.Errorf("failed to submit results: %w", err)
		}

		return nil
	},
}

// fetchValidationsOfType fetches all validation resources of a specific type
func fetchValidationsOfType(ctx interface{}, dynamicClient interface{}, namespace string, valType ValidationType) ([]ValidationResult, error) {
	// Type assertion for context and client
	listUnstructured, err := dynamicClient.(interface {
		Resource(resource schema.GroupVersionResource) interface {
			Namespace(namespace string) interface {
				List(ctx interface{}, opts metav1.ListOptions) (*unstructured.UnstructuredList, error)
			}
		}
	}).Resource(valType.GVR).Namespace(namespace).List(ctx, metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	var results []ValidationResult
	for _, item := range listUnstructured.Items {
		result := parseValidationResult(item, valType)
		results = append(results, result)
	}

	return results, nil
}

// parseValidationResult extracts validation result from unstructured resource
func parseValidationResult(obj unstructured.Unstructured, valType ValidationType) ValidationResult {
	result := ValidationResult{
		Name:    obj.GetName(),
		Type:    valType.Resource,
		Details: []string{},
	}

	// Extract status
	status, found, err := unstructured.NestedMap(obj.Object, "status")
	if !found || err != nil {
		result.AllPassed = false
		result.Details = append(result.Details, "Status not found or invalid")
		return result
	}

	result.RawStatus = status

	// Extract allPassed field
	allPassed, found, err := unstructured.NestedBool(status, "allPassed")
	if found && err == nil {
		result.AllPassed = allPassed
	}

	// Extract error message if present
	errorMsg, found, err := unstructured.NestedString(status, "error")
	if found && err == nil && errorMsg != "" {
		result.Details = append(result.Details, fmt.Sprintf("Error: %s", errorMsg))
	}

	// Build details from status
	result.Details = append(result.Details, buildDetailsFromStatus(status, valType)...)

	return result
}

// buildDetailsFromStatus extracts detailed information from status
func buildDetailsFromStatus(status map[string]interface{}, valType ValidationType) []string {
	var details []string

	// Extract resources array if present
	resources, found, err := unstructured.NestedSlice(status, "resources")
	if !found || err != nil {
		return details
	}

	// Limit details to avoid overwhelming output
	maxDetails := 5
	count := 0

	for _, resource := range resources {
		if count >= maxDetails {
			remaining := len(resources) - count
			details = append(details, fmt.Sprintf("... and %d more", remaining))
			break
		}

		resourceMap, ok := resource.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract message if present
		message, found, err := unstructured.NestedString(resourceMap, "message")
		if found && err == nil && message != "" {
			details = append(details, message)
			count++
		}
	}

	return details
}

// buildValidationPayload creates the payload structure for the API
func buildValidationPayload(results []ValidationResult) map[string]interface{} {
	payload := make(map[string]interface{})

	// Group results by validation type
	for _, result := range results {
		typeKey := result.Type
		if _, exists := payload[typeKey]; !exists {
			payload[typeKey] = []map[string]interface{}{}
		}

		resultMap := map[string]interface{}{
			"name":      result.Name,
			"passed":    result.AllPassed,
			"details":   result.Details,
			"rawStatus": result.RawStatus,
		}

		payload[typeKey] = append(payload[typeKey].([]map[string]interface{}), resultMap)
	}

	return payload
}

func init() {
	challengeCmd.AddCommand(submitCmd)
}
