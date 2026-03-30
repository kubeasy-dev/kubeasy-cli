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
