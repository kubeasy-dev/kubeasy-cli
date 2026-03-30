// Package spec implements the "spec" validation type.
// It validates resource manifest fields using path-based checks.
package spec

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/fieldpath"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	errNoChecksSpecified   = "No checks specified"
	errNoMatchingResources = "No matching resources found"
	errNoTargetSpecified   = "No target name or labelSelector specified"
	msgAllChecksPassed     = "All spec checks passed" //nolint:gosec // not a credential
)

// Execute validates resource manifest fields using path-based checks.
func Execute(ctx context.Context, spec vtypes.SpecSpec, deps shared.Deps) (bool, string, error) {
	logger.Debug("Executing spec validation for %s", spec.Target.Kind)

	if len(spec.Checks) == 0 {
		return false, errNoChecksSpecified, nil
	}

	gvr, err := shared.GetGVRForKind(spec.Target.Kind)
	if err != nil {
		return false, "", err
	}

	var obj *unstructured.Unstructured

	switch {
	case spec.Target.Name != "":
		obj, err = deps.DynamicClient.Resource(gvr).Namespace(deps.Namespace).Get(ctx, spec.Target.Name, metav1.GetOptions{})
	case len(spec.Target.LabelSelector) > 0:
		list, listErr := deps.DynamicClient.Resource(gvr).Namespace(deps.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(spec.Target.LabelSelector).String(),
		})
		if listErr != nil {
			return false, "", listErr
		}
		if len(list.Items) == 0 {
			return false, errNoMatchingResources, nil
		}
		obj = &list.Items[0]
	default:
		return false, errNoTargetSpecified, nil
	}

	if err != nil {
		return false, "", fmt.Errorf("failed to get resource: %w", err)
	}

	allPassed := true
	var messages []string

	for _, check := range spec.Checks {
		actual, found, resolveErr := fieldpath.GetRaw(obj.Object, check.Path)
		if resolveErr != nil {
			allPassed = false
			messages = append(messages, fmt.Sprintf("path %q: %v", check.Path, resolveErr))
			continue
		}

		switch {
		case check.Exists != nil:
			if found != *check.Exists {
				allPassed = false
				if *check.Exists {
					messages = append(messages, fmt.Sprintf("path %q: field not found (expected to exist)", check.Path))
				} else {
					messages = append(messages, fmt.Sprintf("path %q: field exists with value %v (expected to be absent)", check.Path, actual))
				}
			}

		case check.Value != nil:
			if !found {
				allPassed = false
				messages = append(messages, fmt.Sprintf("path %q: field not found", check.Path))
				continue
			}
			if !valuesEqual(actual, check.Value) {
				allPassed = false
				messages = append(messages, fmt.Sprintf("path %q: got %v, expected %v", check.Path, actual, check.Value))
			}

		case check.Contains != nil:
			if !found {
				allPassed = false
				messages = append(messages, fmt.Sprintf("path %q: field not found", check.Path))
				continue
			}
			slice, ok := actual.([]interface{})
			if !ok {
				allPassed = false
				messages = append(messages, fmt.Sprintf("path %q: field is not a list (got %T)", check.Path, actual))
				continue
			}
			matchFound := false
			for _, elem := range slice {
				if deepContains(elem, check.Contains) {
					matchFound = true
					break
				}
			}
			if !matchFound {
				allPassed = false
				messages = append(messages, fmt.Sprintf("path %q: no element matches %v", check.Path, check.Contains))
			}
		}
	}

	if allPassed {
		return true, msgAllChecksPassed, nil
	}
	return false, strings.Join(messages, "; "), nil
}

// valuesEqual compares two values for equality, normalizing numeric types.
// Handles int/int64/float64 mismatches that arise from YAML vs JSON unmarshaling.
func valuesEqual(actual, expected interface{}) bool {
	if reflect.DeepEqual(actual, expected) {
		return true
	}
	fa, aIsNum := toFloat64(actual)
	fb, bIsNum := toFloat64(expected)
	if aIsNum && bIsNum {
		return fa == fb
	}
	return false
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

// deepContains reports whether actual contains all key-value pairs defined in expected.
// For maps: every key in expected must exist in actual with a matching value (recursive).
func deepContains(actual, expected interface{}) bool {
	expectedMap, ok := expected.(map[string]interface{})
	if !ok {
		return valuesEqual(actual, expected)
	}
	actualMap, ok := actual.(map[string]interface{})
	if !ok {
		return false
	}
	for k, ev := range expectedMap {
		av, exists := actualMap[k]
		if !exists {
			return false
		}
		if !deepContains(av, ev) {
			return false
		}
	}
	return true
}
