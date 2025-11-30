//go:build integration
// +build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/validation"
	"github.com/kubeasy-dev/kubeasy-cli/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEventValidation_NoForbiddenEvents_Success(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
		},
	}

	createdPod := env.CreatePod(pod)
	require.NotNil(t, createdPod)

	// Create a normal event (not forbidden)
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-event-normal",
			Namespace: env.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      createdPod.Name,
			Namespace: env.Namespace,
		},
		Reason:  "Started",
		Message: "Container started",
		Type:    corev1.EventTypeNormal,
		LastTimestamp: metav1.Time{
			Time: time.Now(),
		},
		EventTime: metav1.MicroTime{
			Time: time.Now(),
		},
	}

	env.CreateEvent(event)

	// Create validation spec
	spec := validation.EventSpec{
		Target: validation.Target{
			Kind: "Pod",
			LabelSelector: map[string]string{
				"app": "test-app",
			},
		},
		ForbiddenReasons: []string{
			"OOMKilled",
			"Evicted",
			"BackOff",
		},
		SinceSeconds: 300,
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-no-forbidden-events",
		Type: validation.TypeEvent,
		Spec: spec,
	})

	// Assert results
	assert.True(t, result.Passed, "Expected validation to pass")
	assert.Equal(t, "No forbidden events found", result.Message)
}

func TestEventValidation_OOMKilled_Failure(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oom-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
		},
	}

	createdPod := env.CreatePod(pod)
	require.NotNil(t, createdPod)

	// Create an OOMKilled event
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-event-oom",
			Namespace: env.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      createdPod.Name,
			Namespace: env.Namespace,
		},
		Reason:  "OOMKilled",
		Message: "Container was OOM killed",
		Type:    corev1.EventTypeWarning,
		LastTimestamp: metav1.Time{
			Time: time.Now(),
		},
		EventTime: metav1.MicroTime{
			Time: time.Now(),
		},
	}

	env.CreateEvent(event)

	// Create validation spec
	spec := validation.EventSpec{
		Target: validation.Target{
			Kind: "Pod",
			LabelSelector: map[string]string{
				"app": "test-app",
			},
		},
		ForbiddenReasons: []string{
			"OOMKilled",
			"Evicted",
		},
		SinceSeconds: 300,
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-oom-event",
		Type: validation.TypeEvent,
		Spec: spec,
	})

	// Assert results
	assert.False(t, result.Passed, "Expected validation to fail")
	assert.Contains(t, result.Message, "Forbidden events detected")
	assert.Contains(t, result.Message, "OOMKilled")
}

func TestEventValidation_MultipleForbiddenEvents_Failure(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "crash-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "crash-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
		},
	}

	createdPod := env.CreatePod(pod)
	require.NotNil(t, createdPod)

	// Create multiple forbidden events
	events := []struct {
		name   string
		reason string
	}{
		{"event-oom", "OOMKilled"},
		{"event-backoff", "BackOff"},
		{"event-failed", "Failed"},
	}

	for _, e := range events {
		event := &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      e.name,
				Namespace: env.Namespace,
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Pod",
				Name:      createdPod.Name,
				Namespace: env.Namespace,
			},
			Reason:  e.reason,
			Message: "Something went wrong",
			Type:    corev1.EventTypeWarning,
			LastTimestamp: metav1.Time{
				Time: time.Now(),
			},
			EventTime: metav1.MicroTime{
				Time: time.Now(),
			},
		}
		env.CreateEvent(event)
	}

	// Create validation spec
	spec := validation.EventSpec{
		Target: validation.Target{
			Kind: "Pod",
			LabelSelector: map[string]string{
				"app": "crash-app",
			},
		},
		ForbiddenReasons: []string{
			"OOMKilled",
			"BackOff",
			"Failed",
		},
		SinceSeconds: 300,
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-multiple-forbidden",
		Type: validation.TypeEvent,
		Spec: spec,
	})

	// Assert results
	assert.False(t, result.Passed, "Expected validation to fail")
	assert.Contains(t, result.Message, "Forbidden events detected")
	// Should contain at least one of the forbidden events
	hasForbidden := false
	for _, e := range events {
		if strings.Contains(result.Message, e.reason) {
			hasForbidden = true
			break
		}
	}
	assert.True(t, hasForbidden, "Message should contain forbidden event reasons")
}

func TestEventValidation_TimeWindow_OldEventsIgnored(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "old-event-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
		},
	}

	createdPod := env.CreatePod(pod)
	require.NotNil(t, createdPod)

	// Create an old OOMKilled event (outside time window)
	oldEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "old-oom-event",
			Namespace: env.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      createdPod.Name,
			Namespace: env.Namespace,
		},
		Reason:  "OOMKilled",
		Message: "Old OOM event",
		Type:    corev1.EventTypeWarning,
		LastTimestamp: metav1.Time{
			Time: time.Now().Add(-10 * time.Minute), // 10 minutes ago
		},
		EventTime: metav1.MicroTime{
			Time: time.Now().Add(-10 * time.Minute),
		},
	}

	env.CreateEvent(oldEvent)

	// Create validation spec with 5-minute window
	spec := validation.EventSpec{
		Target: validation.Target{
			Kind: "Pod",
			LabelSelector: map[string]string{
				"app": "test-app",
			},
		},
		ForbiddenReasons: []string{
			"OOMKilled",
		},
		SinceSeconds: 300, // 5 minutes
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-old-events-ignored",
		Type: validation.TypeEvent,
		Spec: spec,
	})

	// Assert results - should pass because old event is outside time window
	assert.True(t, result.Passed, "Expected validation to pass (old events should be ignored)")
	assert.Equal(t, "No forbidden events found", result.Message)
}

func TestEventValidation_SpecificPodByName(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create two pods
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
		},
	}

	createdPod1 := env.CreatePod(pod1)
	createdPod2 := env.CreatePod(pod2)
	require.NotNil(t, createdPod1)
	require.NotNil(t, createdPod2)

	// Create OOMKilled event for pod2 only
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oom-event-pod2",
			Namespace: env.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      createdPod2.Name,
			Namespace: env.Namespace,
		},
		Reason:  "OOMKilled",
		Message: "OOM killed",
		Type:    corev1.EventTypeWarning,
		LastTimestamp: metav1.Time{
			Time: time.Now(),
		},
		EventTime: metav1.MicroTime{
			Time: time.Now(),
		},
	}

	env.CreateEvent(event)

	// Validate pod1 by name (should pass - no events)
	spec := validation.EventSpec{
		Target: validation.Target{
			Kind: "Pod",
			Name: "target-pod",
		},
		ForbiddenReasons: []string{
			"OOMKilled",
		},
		SinceSeconds: 300,
	}

	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-specific-pod",
		Type: validation.TypeEvent,
		Spec: spec,
	})

	// Should pass because target-pod has no OOMKilled events
	assert.True(t, result.Passed, "Expected validation to pass for target-pod")
	assert.Equal(t, "No forbidden events found", result.Message)
}
