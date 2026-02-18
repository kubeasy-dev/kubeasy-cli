package devutils

import (
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
)

// DisplayValidationResults renders validation results grouped by type and returns whether all passed.
func DisplayValidationResults(validations []validation.Validation, results []validation.Result) bool {
	allPassed := true

	// Group validations by type for display
	typeResults := make(map[validation.ValidationType][]validation.Result)
	for i, v := range validations {
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
		}
		ui.Println()
	}

	return allPassed
}
