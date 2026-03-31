// Package log implements the "log" validation type.
// It searches container logs for expected strings.
package log

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	corev1 "k8s.io/api/core/v1"
)

const (
	errNoMatchingPods          = "No matching pods found"
	msgFoundAllExpectedStrings = "Found all expected strings in logs"
	msgFoundAnyExpectedString  = "Found at least one expected string in logs"
)

// Execute searches container logs for expected strings.
func Execute(ctx context.Context, spec vtypes.LogSpec, deps shared.Deps) (bool, string, error) {
	logger.Debug("Executing log validation")

	pods, err := shared.GetTargetPods(ctx, deps, spec.Target)
	if err != nil {
		return false, "", err
	}
	if len(pods) == 0 {
		return false, errNoMatchingPods, nil
	}

	var sinceSecondsPtr *int64
	if spec.SinceSeconds > 0 {
		s := int64(spec.SinceSeconds)
		sinceSecondsPtr = &s
	}

	var logErrors []string

	podLogs := make(map[string]string)
	for _, pod := range pods {
		container := spec.Container
		if container == "" && len(pod.Spec.Containers) > 0 {
			container = pod.Spec.Containers[0].Name
		}

		opts := &corev1.PodLogOptions{
			Container:    container,
			SinceSeconds: sinceSecondsPtr,
			Previous:     spec.Previous,
		}

		req := deps.Clientset.CoreV1().Pods(deps.Namespace).GetLogs(pod.Name, opts)
		logs, err := req.Do(ctx).Raw()
		if err != nil {
			errMsg := fmt.Sprintf("pod %s: %v", pod.Name, err)
			logger.Debug("Failed to get logs for %s", errMsg)
			logErrors = append(logErrors, errMsg)
			continue
		}
		podLogs[pod.Name] = string(logs)
	}

	errSuffix := ""
	if len(logErrors) > 0 {
		errSuffix = fmt.Sprintf(" (errors fetching logs: %s)", strings.Join(logErrors, "; "))
	}

	if spec.MatchMode == vtypes.MatchModeAnyOf {
		for _, expected := range spec.ExpectedStrings {
			for _, logs := range podLogs {
				if strings.Contains(logs, expected) {
					return true, msgFoundAnyExpectedString, nil
				}
			}
		}
		return false, fmt.Sprintf("None of the expected strings found in logs: %v%s", spec.ExpectedStrings, errSuffix), nil
	}

	// allOf (default) — all strings must be present
	var missingStrings []string
	for _, expected := range spec.ExpectedStrings {
		found := false
		for _, logs := range podLogs {
			if strings.Contains(logs, expected) {
				found = true
				break
			}
		}
		if !found {
			missingStrings = append(missingStrings, expected)
		}
	}

	if len(missingStrings) == 0 {
		return true, msgFoundAllExpectedStrings, nil
	}
	return false, fmt.Sprintf("Missing strings in logs: %v%s", missingStrings, errSuffix), nil
}
