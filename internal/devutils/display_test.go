package devutils

import (
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
	"github.com/stretchr/testify/assert"
)

func TestDisplayValidationResults(t *testing.T) {
	t.Run("all passed returns true", func(t *testing.T) {
		validations := []validation.Validation{
			{Key: "pod-ready", Type: validation.TypeCondition},
			{Key: "deploy-ready", Type: validation.TypeCondition},
		}
		results := []validation.Result{
			{Key: "pod-ready", Passed: true, Message: "Pod is ready"},
			{Key: "deploy-ready", Passed: true, Message: "Deployment is ready"},
		}

		allPassed := DisplayValidationResults(validations, results)
		assert.True(t, allPassed)
	})

	t.Run("one failure returns false", func(t *testing.T) {
		validations := []validation.Validation{
			{Key: "pod-ready", Type: validation.TypeCondition},
			{Key: "deploy-ready", Type: validation.TypeCondition},
		}
		results := []validation.Result{
			{Key: "pod-ready", Passed: true, Message: "Pod is ready"},
			{Key: "deploy-ready", Passed: false, Message: "Deployment not available"},
		}

		allPassed := DisplayValidationResults(validations, results)
		assert.False(t, allPassed)
	})

	t.Run("all failed returns false", func(t *testing.T) {
		validations := []validation.Validation{
			{Key: "pod-ready", Type: validation.TypeCondition},
			{Key: "log-check", Type: validation.TypeLog},
		}
		results := []validation.Result{
			{Key: "pod-ready", Passed: false, Message: "Pod not found"},
			{Key: "log-check", Passed: false, Message: "Expected string not found"},
		}

		allPassed := DisplayValidationResults(validations, results)
		assert.False(t, allPassed)
	})

	t.Run("mixed types grouped correctly", func(t *testing.T) {
		validations := []validation.Validation{
			{Key: "pod-ready", Type: validation.TypeCondition},
			{Key: "no-crashes", Type: validation.TypeEvent},
			{Key: "deploy-ok", Type: validation.TypeCondition},
			{Key: "log-check", Type: validation.TypeLog},
		}
		results := []validation.Result{
			{Key: "pod-ready", Passed: true, Message: "OK"},
			{Key: "no-crashes", Passed: true, Message: "No forbidden events"},
			{Key: "deploy-ok", Passed: true, Message: "OK"},
			{Key: "log-check", Passed: true, Message: "Found expected strings"},
		}

		allPassed := DisplayValidationResults(validations, results)
		assert.True(t, allPassed)
	})

	t.Run("empty validations returns true", func(t *testing.T) {
		allPassed := DisplayValidationResults(nil, nil)
		assert.True(t, allPassed)
	})
}
