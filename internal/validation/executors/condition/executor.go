// Package condition implements the "condition" validation type.
// It checks Kubernetes resource conditions (Ready, Available, etc.).
package condition

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
)

const (
	errNoChecksSpecified = "No checks specified"
	errNoMatchingPods    = "No matching pods found"
	msgAllConditionsMet  = "All %d pod(s) meet the required conditions"
)

// Execute validates Kubernetes resource conditions for all matching pods.
func Execute(ctx context.Context, spec vtypes.ConditionSpec, deps shared.Deps) (bool, string, error) {
	logger.Debug("Executing condition validation for %s", spec.Target.Kind)

	if len(spec.Checks) == 0 {
		return false, errNoChecksSpecified, nil
	}

	pods, err := shared.GetTargetPods(ctx, deps, spec.Target)
	if err != nil {
		return false, "", err
	}
	if len(pods) == 0 {
		return false, errNoMatchingPods, nil
	}

	allPassed := true
	var messages []string

	for _, pod := range pods {
		for _, check := range spec.Checks {
			passed := false
			conditionFound := false
			for _, podCond := range pod.Status.Conditions {
				if string(podCond.Type) == check.Type {
					conditionFound = true
					passed = podCond.Status == check.Status
					break
				}
			}
			if !conditionFound {
				logger.Debug("Pod %s: condition type %s not found (available: %v)", pod.Name, check.Type, shared.GetPodConditionTypes(&pod))
				allPassed = false
				messages = append(messages, fmt.Sprintf("Pod %s: condition %s not found", pod.Name, check.Type))
			} else if !passed {
				allPassed = false
				messages = append(messages, fmt.Sprintf("Pod %s: condition %s is not %s", pod.Name, check.Type, check.Status))
			}
		}
	}

	if allPassed {
		return true, fmt.Sprintf(msgAllConditionsMet, len(pods)), nil
	}
	return false, strings.Join(messages, "; "), nil
}
