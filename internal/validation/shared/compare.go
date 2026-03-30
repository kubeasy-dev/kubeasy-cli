package shared

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// GetNestedInt64 extracts an int64 value from a nested map.
func GetNestedInt64(obj map[string]interface{}, fields ...string) (int64, bool, error) {
	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if !found || err != nil {
		return 0, found, err
	}

	switch v := val.(type) {
	case int64:
		return v, true, nil
	case int32:
		return int64(v), true, nil
	case int:
		return int64(v), true, nil
	case float64:
		return int64(v), true, nil
	default:
		return 0, true, fmt.Errorf("unexpected type %T", val)
	}
}

// CompareValues compares two int64 values using the specified operator.
func CompareValues(actual int64, operator string, expected int64) bool {
	switch operator {
	case "==", "=":
		return actual == expected
	case "!=":
		return actual != expected
	case ">":
		return actual > expected
	case "<":
		return actual < expected
	case ">=":
		return actual >= expected
	case "<=":
		return actual <= expected
	default:
		return false
	}
}

// GetPodConditionTypes returns a list of condition types present on a pod (for debug logging).
func GetPodConditionTypes(pod *corev1.Pod) []string {
	types := make([]string, len(pod.Status.Conditions))
	for i, cond := range pod.Status.Conditions {
		types[i] = string(cond.Type)
	}
	return types
}

// CompareTypedValues compares two values using the specified operator.
// Supports string, int64, float64, and bool types.
func CompareTypedValues(actual interface{}, operator string, expected interface{}) (bool, error) {
	if actual == nil {
		return false, fmt.Errorf("actual value is nil")
	}

	switch actualVal := actual.(type) {
	case string:
		expectedStr, ok := expected.(string)
		if !ok {
			return false, fmt.Errorf("type mismatch: actual is string, expected is %T", expected)
		}
		return compareStrings(actualVal, operator, expectedStr)

	case bool:
		expectedBool, ok := expected.(bool)
		if !ok {
			return false, fmt.Errorf("type mismatch: actual is bool, expected is %T", expected)
		}
		return compareBools(actualVal, operator, expectedBool)

	case int64:
		return compareNumeric(float64(actualVal), operator, expected)
	case int32:
		return compareNumeric(float64(actualVal), operator, expected)
	case int:
		return compareNumeric(float64(actualVal), operator, expected)
	case float64:
		return compareNumeric(actualVal, operator, expected)

	default:
		return false, fmt.Errorf("unsupported type: %T", actual)
	}
}

func compareStrings(actual, operator, expected string) (bool, error) {
	switch operator {
	case "==", "=":
		return actual == expected, nil
	case "!=":
		return actual != expected, nil
	default:
		return false, fmt.Errorf("operator %s not supported for strings (use == or !=)", operator)
	}
}

func compareBools(actual bool, operator string, expected bool) (bool, error) {
	switch operator {
	case "==", "=":
		return actual == expected, nil
	case "!=":
		return actual != expected, nil
	default:
		return false, fmt.Errorf("operator %s not supported for booleans (use == or !=)", operator)
	}
}

// compareNumeric compares numeric values using the specified operator.
// Handles int/float coercion by converting all numeric types to float64.
//
// Note: Converting large int64 values to float64 may lose precision for values
// greater than 2^53 (9,007,199,254,740,992). For typical Kubernetes use cases
// like replica counts, restart counts, and resource metrics, this is not an issue.
func compareNumeric(actual float64, operator string, expected interface{}) (bool, error) {
	var expectedFloat float64

	switch v := expected.(type) {
	case int:
		expectedFloat = float64(v)
	case int32:
		expectedFloat = float64(v)
	case int64:
		expectedFloat = float64(v)
	case float64:
		expectedFloat = v
	default:
		return false, fmt.Errorf("expected value must be numeric, got %T", expected)
	}

	switch operator {
	case "==", "=":
		return actual == expectedFloat, nil
	case "!=":
		return actual != expectedFloat, nil
	case ">":
		return actual > expectedFloat, nil
	case "<":
		return actual < expectedFloat, nil
	case ">=":
		return actual >= expectedFloat, nil
	case "<=":
		return actual <= expectedFloat, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", operator)
	}
}
