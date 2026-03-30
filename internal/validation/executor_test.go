package validation_test

import (
	"context"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func newTestExecutor(objs ...runtime.Object) *validation.Executor {
	return validation.NewExecutor(
		fake.NewClientset(objs...),
		dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		&rest.Config{},
		"test-ns",
	)
}

func TestNewExecutor(t *testing.T) {
	e := newTestExecutor()
	require.NotNil(t, e)
}

func TestNewExecutor_NilValues(t *testing.T) {
	e := validation.NewExecutor(nil, nil, nil, "")
	require.NotNil(t, e)
}

func TestExecute_UnknownType(t *testing.T) {
	e := newTestExecutor()

	result := e.Execute(context.Background(), validation.Validation{
		Key:  "test-key",
		Type: "invalid-type",
		Spec: validation.StatusSpec{},
	})

	assert.False(t, result.Passed)
	assert.Contains(t, result.Message, "Unknown validation type")
	assert.Equal(t, "test-key", result.Key)
}

func TestExecute_WrongSpecType(t *testing.T) {
	e := newTestExecutor()

	// Pass a StatusSpec where a ConditionSpec is expected
	result := e.Execute(context.Background(), validation.Validation{
		Key:  "bad-spec",
		Type: validation.TypeCondition,
		Spec: validation.StatusSpec{}, // wrong type
	})

	assert.False(t, result.Passed)
	assert.Contains(t, result.Message, "internal error")
}

func TestExecuteAll(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
	e := newTestExecutor(pod)

	validations := []validation.Validation{
		{
			Key:  "pod-ready",
			Type: validation.TypeCondition,
			Spec: validation.ConditionSpec{
				Target: validation.Target{Kind: "Pod", Name: "test-pod"},
				Checks: []validation.ConditionCheck{
					{Type: "Ready", Status: corev1.ConditionTrue},
				},
			},
		},
		{
			Key:  "unknown-type",
			Type: "invalid",
			Spec: validation.StatusSpec{},
		},
	}

	results := e.ExecuteAll(context.Background(), validations)

	require.Len(t, results, 2)
	// Results are in input order despite parallel execution
	assert.Equal(t, "pod-ready", results[0].Key)
	assert.True(t, results[0].Passed)
	assert.Equal(t, "unknown-type", results[1].Key)
	assert.False(t, results[1].Passed)
}

func TestExecuteSequential(t *testing.T) {
	e := newTestExecutor()

	validations := []validation.Validation{
		{Key: "a", Type: "invalid", Spec: validation.StatusSpec{}},
		{Key: "b", Type: "invalid", Spec: validation.StatusSpec{}},
	}

	results := e.ExecuteSequential(context.Background(), validations, false)
	require.Len(t, results, 2)
}

func TestExecuteSequential_FailFast(t *testing.T) {
	e := newTestExecutor()

	validations := []validation.Validation{
		{Key: "a", Type: "invalid", Spec: validation.StatusSpec{}},
		{Key: "b", Type: "invalid", Spec: validation.StatusSpec{}},
	}

	results := e.ExecuteSequential(context.Background(), validations, true)
	// Should stop after first failure
	require.Len(t, results, 1)
	assert.Equal(t, "a", results[0].Key)
}

func TestExecute_ResultHasDuration(t *testing.T) {
	e := newTestExecutor()

	result := e.Execute(context.Background(), validation.Validation{
		Key:  "k",
		Type: "invalid",
		Spec: validation.StatusSpec{},
	})

	assert.Greater(t, result.Duration.Nanoseconds(), int64(0))
}
