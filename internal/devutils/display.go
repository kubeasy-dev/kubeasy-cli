package devutils

import (
	"fmt"
	"slices"

	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
)

// formatDuration formats a duration for display (e.g. "245ms", "1.2s")
func formatDuration(d interface{ Milliseconds() int64 }) string {
	ms := d.Milliseconds()
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

// DisplayValidationResults renders validation results grouped by type and returns whether all passed.
func DisplayValidationResults(validations []validation.Validation, results []validation.Result) bool {
	allPassed := true

	// Group validations by type for display
	typeResults := make(map[validation.ValidationType][]validation.Result)
	for i, v := range validations {
	for i, v := range validations {
		if i >= len(results) {
			break
		}
		typeResults[v.Type] = append(typeResults[v.Type], results[i])
	}
	}

	typeOrder := []validation.ValidationType{
		validation.TypeCondition,
		validation.TypeStatus,
		validation.TypeLog,
		validation.TypeEvent,
		validation.TypeConnectivity,
	}

	typeLabels := map[validation.ValidationType]string{
		validation.TypeStatus:       "Status Validation",
		validation.TypeCondition:    "Condition Validation",
		validation.TypeLog:          "Log Validation",
		validation.TypeEvent:        "Event Validation",
		validation.TypeConnectivity: "Connectivity Validation",
	}

	for _, valType := range typeOrder {
		typeRes, ok := typeResults[valType]
		if !ok {
			continue
		}
		ui.Section(typeLabels[valType])
		for _, r := range typeRes {
			detail := r.Message
			if r.Duration > 0 {
				detail = fmt.Sprintf("%s (%s)", r.Message, formatDuration(r.Duration))
			}
			ui.ValidationResult(r.Key, r.Passed, []string{detail})
			if !r.Passed {
				allPassed = false
			}
		}
		ui.Println()
	}

	// Handle any unknown types not in typeOrder
	for valType, typeRes := range typeResults {
		if slices.Contains(typeOrder, valType) {
			continue
		}
		ui.Section(string(valType))
		for _, r := range typeRes {
			detail := r.Message
			if r.Duration > 0 {
				detail = fmt.Sprintf("%s (%s)", r.Message, formatDuration(r.Duration))
			}
			ui.ValidationResult(r.Key, r.Passed, []string{detail})
			if !r.Passed {
				allPassed = false
			}
		}
		ui.Println()
	}

	return allPassed
}
