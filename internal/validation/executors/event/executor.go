// Package event implements the "event" validation type.
// It checks for forbidden Kubernetes events (OOMKilled, Evicted, etc.) and optionally
// asserts that required events (SuccessfulRescale, ScalingReplicaSet, etc.) are present.
package event

import (
	"context"
	"fmt"
	"strings"
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

// Execute checks that no forbidden events exist for the target resource and that all
// required events are present within the time window.
//
// When the target kind is "Pod" (or unset), events are matched via pod name lookup
// (supporting label selectors). For other resource kinds (e.g. HorizontalPodAutoscaler,
// Deployment), events are matched directly by InvolvedObject.Kind and Name.
func Execute(ctx context.Context, spec vtypes.EventSpec, deps shared.Deps) (bool, string, error) {
	logger.Debug("Executing event validation")

	// Determine target kind; default to Pod for backward compatibility.
	targetKind := spec.Target.Kind
	if targetKind == "" {
		targetKind = "Pod"
	}

	// matchEvent returns true when the event belongs to the target resource.
	var matchEvent func(involvedKind, involvedName string) bool

	if targetKind == "Pod" {
		// Pod targets: resolve pods (supports label selectors) and match by name.
		pods, err := shared.GetTargetPods(ctx, deps, spec.Target)
		if err != nil {
			return false, "", err
		}
		if len(pods) == 0 {
			return false, errNoMatchingPods, nil
		}
		podNames := make(map[string]bool, len(pods))
		for _, pod := range pods {
			podNames[pod.Name] = true
		}
		matchEvent = func(involvedKind, involvedName string) bool {
			return involvedKind == "Pod" && podNames[involvedName]
		}
	} else {
		// Non-pod targets (HPA, Deployment, …): match events directly by kind + name.
		targetName := spec.Target.Name
		matchEvent = func(involvedKind, involvedName string) bool {
			if involvedKind != targetKind {
				return false
			}
			return targetName == "" || involvedName == targetName
		}
	}

	events, err := deps.Clientset.CoreV1().Events(deps.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, "", fmt.Errorf("failed to list events: %w", err)
	}

	// sinceSeconds==0 means "no time filter" — check all events regardless of age.
	// The loader normalises 0 to DefaultEventSinceSeconds (300s) when loading from YAML,
	// so 0 only reaches here when EventSpec is constructed directly in code.
	var sinceTime time.Time
	if spec.SinceSeconds > 0 {
		sinceTime = time.Now().Add(-time.Duration(spec.SinceSeconds) * time.Second)
	}

	var forbiddenFound []string
	foundReasons := make(map[string]bool)

	for _, event := range events.Items {
		if !matchEvent(event.InvolvedObject.Kind, event.InvolvedObject.Name) {
			continue
		}

		if !sinceTime.IsZero() && event.LastTimestamp.Time.Before(sinceTime) && event.EventTime.Time.Before(sinceTime) {
			continue
		}

		// Track all reasons seen in the time window for required-reasons check.
		foundReasons[event.Reason] = true

		for _, forbidden := range spec.ForbiddenReasons {
			if event.Reason == forbidden {
				forbiddenFound = append(forbiddenFound, fmt.Sprintf("%s on %s", event.Reason, event.InvolvedObject.Name))
			}
		}
	}

	// Check required reasons.
	var missingReasons []string
	for _, required := range spec.RequiredReasons {
		if !foundReasons[required] {
			missingReasons = append(missingReasons, required)
		}
	}

	// Build result.
	var messages []string
	passed := true

	if len(forbiddenFound) > 0 {
		passed = false
		messages = append(messages, fmt.Sprintf("Forbidden events detected: %v", forbiddenFound))
	}
	if len(missingReasons) > 0 {
		passed = false
		messages = append(messages, fmt.Sprintf("Required events not found: %v", missingReasons))
	}

	if !passed {
		return false, strings.Join(messages, "; "), nil
	}

	if len(spec.RequiredReasons) > 0 {
		return true, fmt.Sprintf("No forbidden events found; all required events present: %v", spec.RequiredReasons), nil
	}
	return true, msgNoForbiddenEvents, nil
}
