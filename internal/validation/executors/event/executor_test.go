package event_test

import (
	"context"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/event"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func deps(clientset *fake.Clientset) shared.Deps {
	return shared.Deps{Clientset: clientset, Namespace: "test-ns"}
}

func TestExecute_NoForbiddenEvents(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"},
	}
	spec := vtypes.EventSpec{
		Target:           vtypes.Target{Kind: "Pod", Name: "test-pod"},
		ForbiddenReasons: []string{"OOMKilled", "Evicted"},
		SinceSeconds:     300,
	}

	passed, msg, err := event.Execute(context.Background(), spec, deps(fake.NewClientset(pod)))
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, "No forbidden events found", msg)
}

func TestExecute_ForbiddenEventDetected(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns", UID: "test-uid"},
	}
	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "oom-event", Namespace: "test-ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "test-pod", UID: "test-uid"},
		Reason:         "OOMKilled",
		LastTimestamp:  metav1.Now(),
		EventTime:      metav1.NowMicro(),
	}
	spec := vtypes.EventSpec{
		Target:           vtypes.Target{Kind: "Pod", Name: "test-pod"},
		ForbiddenReasons: []string{"OOMKilled", "Evicted"},
		SinceSeconds:     300,
	}

	passed, msg, err := event.Execute(context.Background(), spec, deps(fake.NewClientset(pod, ev)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Forbidden events detected")
	assert.Contains(t, msg, "OOMKilled")
}

func TestExecute_NoMatchingPods(t *testing.T) {
	spec := vtypes.EventSpec{
		Target:           vtypes.Target{Kind: "Pod", Name: "nonexistent"},
		ForbiddenReasons: []string{"OOMKilled"},
		SinceSeconds:     300,
	}
	passed, _, err := event.Execute(context.Background(), spec, deps(fake.NewClientset()))
	assert.Error(t, err)
	assert.False(t, passed)
}

func TestExecute_OldEventsIgnored(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns", UID: "uid"},
	}
	// Event is 1 hour old, but sinceSeconds is 300 (5 min)
	oldTime := metav1.NewTime(metav1.Now().Add(-3600 * 1e9))
	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "old-event", Namespace: "test-ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "test-pod"},
		Reason:         "OOMKilled",
		LastTimestamp:  oldTime,
	}
	spec := vtypes.EventSpec{
		Target:           vtypes.Target{Kind: "Pod", Name: "test-pod"},
		ForbiddenReasons: []string{"OOMKilled"},
		SinceSeconds:     300,
	}

	passed, msg, err := event.Execute(context.Background(), spec, deps(fake.NewClientset(pod, ev)))
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, "No forbidden events found", msg)
}

func TestExecute_RequiredReasonPresent(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns", UID: "uid"},
	}
	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "rescale-event", Namespace: "test-ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "test-pod"},
		Reason:         "SuccessfulRescale",
		LastTimestamp:  metav1.Now(),
		EventTime:      metav1.NowMicro(),
	}
	spec := vtypes.EventSpec{
		Target:           vtypes.Target{Kind: "Pod", Name: "test-pod"},
		ForbiddenReasons: []string{"FailedGetScale"},
		RequiredReasons:  []string{"SuccessfulRescale"},
		SinceSeconds:     300,
	}

	passed, msg, err := event.Execute(context.Background(), spec, deps(fake.NewClientset(pod, ev)))
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Contains(t, msg, "No forbidden events found")
	assert.Contains(t, msg, "SuccessfulRescale")
}

func TestExecute_RequiredReasonMissing(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns", UID: "uid"},
	}
	spec := vtypes.EventSpec{
		Target:          vtypes.Target{Kind: "Pod", Name: "test-pod"},
		RequiredReasons: []string{"SuccessfulRescale"},
		SinceSeconds:    300,
	}

	passed, msg, err := event.Execute(context.Background(), spec, deps(fake.NewClientset(pod)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Required events not found")
	assert.Contains(t, msg, "SuccessfulRescale")
}

func TestExecute_RequiredReasonOldEventIgnored(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns", UID: "uid"},
	}
	// Event is 1 hour old — outside the 5-minute window
	oldTime := metav1.NewTime(metav1.Now().Add(-3600 * 1e9))
	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "old-rescale", Namespace: "test-ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "test-pod"},
		Reason:         "SuccessfulRescale",
		LastTimestamp:  oldTime,
	}
	spec := vtypes.EventSpec{
		Target:          vtypes.Target{Kind: "Pod", Name: "test-pod"},
		RequiredReasons: []string{"SuccessfulRescale"},
		SinceSeconds:    300,
	}

	passed, msg, err := event.Execute(context.Background(), spec, deps(fake.NewClientset(pod, ev)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Required events not found")
	assert.Contains(t, msg, "SuccessfulRescale")
}

func TestExecute_BothForbiddenAndRequiredFail(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns", UID: "uid"},
	}
	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "oom-event", Namespace: "test-ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "test-pod"},
		Reason:         "OOMKilled",
		LastTimestamp:  metav1.Now(),
		EventTime:      metav1.NowMicro(),
	}
	spec := vtypes.EventSpec{
		Target:           vtypes.Target{Kind: "Pod", Name: "test-pod"},
		ForbiddenReasons: []string{"OOMKilled"},
		RequiredReasons:  []string{"SuccessfulRescale"},
		SinceSeconds:     300,
	}

	passed, msg, err := event.Execute(context.Background(), spec, deps(fake.NewClientset(pod, ev)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Forbidden events detected")
	assert.Contains(t, msg, "OOMKilled")
	assert.Contains(t, msg, "Required events not found")
	assert.Contains(t, msg, "SuccessfulRescale")
}

func TestExecute_NonPodTarget_RequiredReasonPresent(t *testing.T) {
	// SuccessfulRescale events have InvolvedObject.Kind = "HorizontalPodAutoscaler", not "Pod".
	// This test ensures non-pod targets are matched by kind+name, not via pod lookup.
	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "rescale-event", Namespace: "test-ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "HorizontalPodAutoscaler", Name: "my-hpa"},
		Reason:         "SuccessfulRescale",
		LastTimestamp:  metav1.Now(),
		EventTime:      metav1.NowMicro(),
	}
	spec := vtypes.EventSpec{
		Target:          vtypes.Target{Kind: "HorizontalPodAutoscaler", Name: "my-hpa"},
		RequiredReasons: []string{"SuccessfulRescale"},
		SinceSeconds:    300,
	}

	passed, msg, err := event.Execute(context.Background(), spec, deps(fake.NewClientset(ev)))
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Contains(t, msg, "SuccessfulRescale")
}

func TestExecute_NonPodTarget_RequiredReasonMissing(t *testing.T) {
	// No SuccessfulRescale event exists for the HPA — should fail.
	spec := vtypes.EventSpec{
		Target:          vtypes.Target{Kind: "HorizontalPodAutoscaler", Name: "my-hpa"},
		RequiredReasons: []string{"SuccessfulRescale"},
		SinceSeconds:    300,
	}

	passed, msg, err := event.Execute(context.Background(), spec, deps(fake.NewClientset()))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Required events not found")
	assert.Contains(t, msg, "SuccessfulRescale")
}

func TestExecute_NonPodTarget_ForbiddenAndRequiredReasons(t *testing.T) {
	// HPA has a FailedGetScale event (forbidden) but no SuccessfulRescale (required).
	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "fail-event", Namespace: "test-ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "HorizontalPodAutoscaler", Name: "my-hpa"},
		Reason:         "FailedGetScale",
		LastTimestamp:  metav1.Now(),
		EventTime:      metav1.NowMicro(),
	}
	spec := vtypes.EventSpec{
		Target:           vtypes.Target{Kind: "HorizontalPodAutoscaler", Name: "my-hpa"},
		ForbiddenReasons: []string{"FailedGetScale"},
		RequiredReasons:  []string{"SuccessfulRescale"},
		SinceSeconds:     300,
	}

	passed, msg, err := event.Execute(context.Background(), spec, deps(fake.NewClientset(ev)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Forbidden events detected")
	assert.Contains(t, msg, "FailedGetScale")
	assert.Contains(t, msg, "Required events not found")
	assert.Contains(t, msg, "SuccessfulRescale")
}
