// Package condition implements the "condition" validation type.
// It checks .status.conditions on any Kubernetes resource (Pod, Deployment, StatefulSet, …).
package condition

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	errNoChecksSpecified = "No checks specified"
	errNoMatchingObjects = "No matching resources found"
	msgAllConditionsMet  = "All checks passed"
)

// Execute validates .status.conditions on any Kubernetes resource.
func Execute(ctx context.Context, spec vtypes.ConditionSpec, deps shared.Deps) (bool, string, error) {
	logger.Debug("Executing condition validation for %s", spec.Target.Kind)

	if len(spec.Checks) == 0 {
		return false, errNoChecksSpecified, nil
	}

	gvr, err := shared.GetGVRForKind(spec.Target.Kind)
	if err != nil {
		return false, "", err
	}

	var objs []unstructured.Unstructured

	switch {
	case spec.Target.Name != "":
		obj, err := deps.DynamicClient.Resource(gvr).Namespace(deps.Namespace).Get(ctx, spec.Target.Name, metav1.GetOptions{})
		if err != nil {
			return false, "", fmt.Errorf("failed to get %s %s: %w", spec.Target.Kind, spec.Target.Name, err)
		}
		objs = []unstructured.Unstructured{*obj}

	case len(spec.Target.LabelSelector) > 0:
		list, err := deps.DynamicClient.Resource(gvr).Namespace(deps.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(spec.Target.LabelSelector).String(),
		})
		if err != nil {
			return false, "", fmt.Errorf("failed to list %s: %w", spec.Target.Kind, err)
		}
		if len(list.Items) == 0 {
			return false, errNoMatchingObjects, nil
		}
		objs = list.Items

	default:
		return false, "No target name or labelSelector specified", nil
	}

	allPassed := true
	var messages []string

	for _, obj := range objs {
		name := obj.GetName()
		rawConditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
		if err != nil || !found {
			allPassed = false
			messages = append(messages, fmt.Sprintf("%s %s: no conditions in status", spec.Target.Kind, name))
			continue
		}

		for _, check := range spec.Checks {
			conditionFound := false
			passed := false
			for _, raw := range rawConditions {
				cond, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				condType, _ := cond["type"].(string)
				if condType != check.Type {
					continue
				}
				conditionFound = true
				condStatus, _ := cond["status"].(string)
				passed = corev1.ConditionStatus(condStatus) == check.Status
				break
			}
			if !conditionFound {
				logger.Debug("%s %s: condition %s not found", spec.Target.Kind, name, check.Type)
				allPassed = false
				messages = append(messages, fmt.Sprintf("%s %s: condition %s not found", spec.Target.Kind, name, check.Type))
			} else if !passed {
				allPassed = false
				messages = append(messages, fmt.Sprintf("%s %s: condition %s is not %s", spec.Target.Kind, name, check.Type, check.Status))
			}
		}
	}

	if allPassed {
		return true, msgAllConditionsMet, nil
	}
	return false, strings.Join(messages, "; "), nil
}
