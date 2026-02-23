package devutils

import (
	"testing"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
	"github.com/stretchr/testify/assert"
)

func TestFormatValidationJSON_AllPassed(t *testing.T) {
	validations := []validation.Validation{
		{Key: "pod-ready", Title: "Pod Ready", Type: validation.TypeCondition},
		{Key: "no-crashes", Title: "Stable Operation", Type: validation.TypeEvent},
	}
	results := []validation.Result{
		{Key: "pod-ready", Passed: true, Message: "All conditions met", Duration: 100 * time.Millisecond},
		{Key: "no-crashes", Passed: true, Message: "No forbidden events", Duration: 50 * time.Millisecond},
	}

	out := FormatValidationJSON("test-challenge", validations, results, 150*time.Millisecond)

	assert.True(t, out.AllPassed)
	assert.Equal(t, 2, out.Total)
	assert.Equal(t, 2, out.Passed)
	assert.Equal(t, 0, out.Failed)
	assert.Equal(t, "test-challenge", out.Slug)
	assert.Len(t, out.Results, 2)
	assert.Equal(t, "condition", out.Results[0].Type)
	assert.Equal(t, "Pod Ready", out.Results[0].Title)
}

func TestFormatValidationJSON_SomeFailed(t *testing.T) {
	validations := []validation.Validation{
		{Key: "pod-ready", Title: "Pod Ready", Type: validation.TypeCondition},
		{Key: "no-crashes", Title: "Stable", Type: validation.TypeEvent},
	}
	results := []validation.Result{
		{Key: "pod-ready", Passed: false, Message: "Pod not ready", Duration: 200 * time.Millisecond},
		{Key: "no-crashes", Passed: true, Message: "OK", Duration: 30 * time.Millisecond},
	}

	out := FormatValidationJSON("test", validations, results, 230*time.Millisecond)

	assert.False(t, out.AllPassed)
	assert.Equal(t, 1, out.Passed)
	assert.Equal(t, 1, out.Failed)
}

func TestFormatValidationJSON_FailFastPartialResults(t *testing.T) {
	validations := []validation.Validation{
		{Key: "a", Title: "A", Type: validation.TypeCondition},
		{Key: "b", Title: "B", Type: validation.TypeEvent},
		{Key: "c", Title: "C", Type: validation.TypeLog},
	}
	// fail-fast stopped after first failure
	results := []validation.Result{
		{Key: "a", Passed: false, Message: "Failed", Duration: 100 * time.Millisecond},
	}

	out := FormatValidationJSON("test", validations, results, 100*time.Millisecond)

	assert.False(t, out.AllPassed)
	assert.Equal(t, 3, out.Total)
	assert.Equal(t, 0, out.Passed)
	assert.Equal(t, 3, out.Failed) // 1 actual + 2 remaining
	assert.Len(t, out.Results, 1)
}

func TestFormatValidationJSON_Empty(t *testing.T) {
	out := FormatValidationJSON("empty", nil, nil, 0)

	assert.True(t, out.AllPassed)
	assert.Equal(t, 0, out.Total)
	assert.Empty(t, out.Results)
}
