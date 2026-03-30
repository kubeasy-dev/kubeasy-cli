// Package event implements the "event" validation type.
// It checks for forbidden Kubernetes events (OOMKilled, Evicted, etc.).
package event

import (
	"context"
	"fmt"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	errNoMatchingPods    = "No matching pods found"
	msgNoForbiddenEvents = "No forbidden events found"
)

// Execute checks that no forbidden events exist for the target pods.
func Execute(ctx context.Context, spec vtypes.EventSpec, deps shared.Deps) (bool, string, error) {
	logger.Debug("Executing event validation")

	pods, err := shared.GetTargetPods(ctx, deps, spec.Target)
	if err != nil {
		return false, "", err
	}
	if len(pods) == 0 {
		return false, errNoMatchingPods, nil
	}

	events, err := deps.Clientset.CoreV1().Events(deps.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, "", fmt.Errorf("failed to list events: %w", err)
	}

	var forbiddenFound []string
	podNames := make(map[string]bool)
	for _, pod := range pods {
		podNames[pod.Name] = true
	}

	// sinceSeconds==0 means "no time filter" — check all events regardless of age.
	// The loader normalises 0 to DefaultEventSinceSeconds (300s) when loading from YAML,
	// so 0 only reaches here when EventSpec is constructed directly in code.
	var sinceTime time.Time
	if spec.SinceSeconds > 0 {
		sinceTime = time.Now().Add(-time.Duration(spec.SinceSeconds) * time.Second)
	}

	for _, event := range events.Items {
		if event.InvolvedObject.Kind != "Pod" || !podNames[event.InvolvedObject.Name] {
			continue
		}

		if !sinceTime.IsZero() && event.LastTimestamp.Time.Before(sinceTime) && event.EventTime.Time.Before(sinceTime) {
			continue
		}

		for _, forbidden := range spec.ForbiddenReasons {
			if event.Reason == forbidden {
				forbiddenFound = append(forbiddenFound, fmt.Sprintf("%s on %s", event.Reason, event.InvolvedObject.Name))
			}
		}
	}

	if len(forbiddenFound) == 0 {
		return true, msgNoForbiddenEvents, nil
	}
	return false, fmt.Sprintf("Forbidden events detected: %v", forbiddenFound), nil
}
