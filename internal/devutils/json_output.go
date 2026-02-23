package devutils

import (
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
)

// JSONValidationOutput is the structured JSON output for validation results.
type JSONValidationOutput struct {
	Slug      string            `json:"slug"`
	AllPassed bool              `json:"allPassed"`
	Total     int               `json:"total"`
	Passed    int               `json:"passed"`
	Failed    int               `json:"failed"`
	Duration  string            `json:"duration"`
	Results   []JSONResultEntry `json:"results"`
}

// JSONResultEntry is a single validation result in JSON output.
type JSONResultEntry struct {
	Key      string `json:"key"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
	Duration string `json:"duration"`
}

// FormatValidationJSON builds a JSONValidationOutput from validations and results.
func FormatValidationJSON(slug string, validations []validation.Validation, results []validation.Result, totalDuration time.Duration) JSONValidationOutput {
	out := JSONValidationOutput{
		Slug:      slug,
		AllPassed: true,
		Total:     len(validations),
		Duration:  totalDuration.Round(time.Millisecond).String(),
		Results:   make([]JSONResultEntry, 0, len(results)),
	}

	for i, r := range results {
		entry := JSONResultEntry{
			Key:      r.Key,
			Passed:   r.Passed,
			Message:  r.Message,
			Duration: r.Duration.Round(time.Millisecond).String(),
		}
		if i < len(validations) {
			entry.Type = string(validations[i].Type)
			entry.Title = validations[i].Title
		}
		if r.Passed {
			out.Passed++
		} else {
			out.Failed++
			out.AllPassed = false
		}
		out.Results = append(out.Results, entry)
	}

	// If fail-fast stopped early, count remaining as failed
	if len(results) < len(validations) {
		out.Failed += len(validations) - len(results)
		out.AllPassed = false
	}

	return out
}
