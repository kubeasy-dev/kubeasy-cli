package condition_test

import (
	"context"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/condition"
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

func TestExecute_Success(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
	spec := vtypes.ConditionSpec{
		Target: vtypes.Target{Kind: "Pod", Name: "test-pod"},
		Checks: []vtypes.ConditionCheck{{Type: "Ready", Status: corev1.ConditionTrue}},
	}

	passed, msg, err := condition.Execute(context.Background(), spec, deps(fake.NewClientset(pod)))
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Contains(t, msg, "All 1 pod(s) meet the required conditions")
}

func TestExecute_ConditionNotMet(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionFalse},
			},
		},
	}
	spec := vtypes.ConditionSpec{
		Target: vtypes.Target{Kind: "Pod", Name: "test-pod"},
		Checks: []vtypes.ConditionCheck{{Type: "Ready", Status: corev1.ConditionTrue}},
	}

	passed, msg, err := condition.Execute(context.Background(), spec, deps(fake.NewClientset(pod)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "condition Ready is not True")
}

func TestExecute_NoMatchingPods(t *testing.T) {
	spec := vtypes.ConditionSpec{
		Target: vtypes.Target{Kind: "Pod", Name: "nonexistent-pod"},
		Checks: []vtypes.ConditionCheck{{Type: "Ready", Status: corev1.ConditionTrue}},
	}

	passed, _, err := condition.Execute(context.Background(), spec, deps(fake.NewClientset()))
	assert.Error(t, err)
	assert.False(t, passed)
}

func TestExecute_ByLabelSelector(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "test-ns", Labels: map[string]string{"app": "test"}},
		Status:     corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-2", Namespace: "test-ns", Labels: map[string]string{"app": "test"}},
		Status:     corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
	}
	spec := vtypes.ConditionSpec{
		Target: vtypes.Target{Kind: "Pod", LabelSelector: map[string]string{"app": "test"}},
		Checks: []vtypes.ConditionCheck{{Type: "Ready", Status: corev1.ConditionTrue}},
	}

	passed, msg, err := condition.Execute(context.Background(), spec, deps(fake.NewClientset(pod1, pod2)))
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Contains(t, msg, "All 2 pod(s) meet the required conditions")
}

func TestExecute_ConditionNotFound(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"},
		Status:     corev1.PodStatus{Conditions: []corev1.PodCondition{}},
	}
	spec := vtypes.ConditionSpec{
		Target: vtypes.Target{Kind: "Pod", Name: "test-pod"},
		Checks: []vtypes.ConditionCheck{{Type: "Ready", Status: corev1.ConditionTrue}},
	}

	passed, msg, err := condition.Execute(context.Background(), spec, deps(fake.NewClientset(pod)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "condition Ready not found")
}

func TestExecute_NoChecks(t *testing.T) {
	spec := vtypes.ConditionSpec{
		Target: vtypes.Target{Kind: "Pod", Name: "any"},
		Checks: nil,
	}
	passed, msg, err := condition.Execute(context.Background(), spec, deps(fake.NewClientset()))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Equal(t, "No checks specified", msg)
}
