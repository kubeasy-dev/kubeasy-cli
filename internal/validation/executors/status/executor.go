// Package status implements the "status" validation type.
// It checks arbitrary resource status fields using comparison operators.
package status

import (
	"context"
	"fmt"
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
	msgAllChecksPassed     = "All status checks passed"
)

// Execute validates arbitrary status fields of a Kubernetes resource.
func Execute(ctx context.Context, spec vtypes.StatusSpec, deps shared.Deps) (bool, string, error) {
	logger.Debug("Executing status validation for %s", spec.Target.Kind)

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
		value, found, err := fieldpath.Get(obj.Object, check.Field)
		if err != nil {
			allPassed = false
			messages = append(messages, fmt.Sprintf("Field %s: %v", check.Field, err))
			continue
		}
		if !found {
			allPassed = false
			messages = append(messages, fmt.Sprintf("Field %s not found", check.Field))
			continue
		}

		passed, compErr := shared.CompareTypedValues(value, check.Operator, check.Value)
		if compErr != nil {
			allPassed = false
			messages = append(messages, fmt.Sprintf("Field %s: %v", check.Field, compErr))
			continue
		}

		if !passed {
			allPassed = false
			messages = append(messages, fmt.Sprintf("%s: got %v, expected %s %v", check.Field, value, check.Operator, check.Value))
		}
	}

	if allPassed {
		return true, msgAllChecksPassed, nil
	}
	return false, strings.Join(messages, "; "), nil
}
