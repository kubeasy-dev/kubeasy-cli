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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func deps(dc *dynamicfake.FakeDynamicClient) shared.Deps {
	return shared.Deps{DynamicClient: dc, Namespace: "test-ns"}
}

func resource(kind, apiVersion, name string, conditions []map[string]interface{}) *unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]interface{}{"name": name, "namespace": "test-ns"},
	}
	if len(conditions) > 0 {
		rawConds := make([]interface{}, len(conditions))
		for i, c := range conditions {
			rawConds[i] = c
		}
		obj["status"] = map[string]interface{}{"conditions": rawConds}
	}
	return &unstructured.Unstructured{Object: obj}
}

func TestExecute_Pod_Ready(t *testing.T) {
	pod := resource("Pod", "v1", "test-pod", []map[string]interface{}{
		{"type": "Ready", "status": "True"},
	})
	spec := vtypes.ConditionSpec{
		Target: vtypes.Target{Kind: "Pod", Name: "test-pod"},
		Checks: []vtypes.ConditionCheck{{Type: "Ready", Status: corev1.ConditionTrue}},
	}

	passed, msg, err := condition.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), pod)))
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, "All checks passed", msg)
}

func TestExecute_Pod_ConditionFalse(t *testing.T) {
	pod := resource("Pod", "v1", "test-pod", []map[string]interface{}{
		{"type": "Ready", "status": "False"},
	})
	spec := vtypes.ConditionSpec{
		Target: vtypes.Target{Kind: "Pod", Name: "test-pod"},
		Checks: []vtypes.ConditionCheck{{Type: "Ready", Status: corev1.ConditionTrue}},
	}

	passed, msg, err := condition.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), pod)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "condition Ready is not True")
}

func TestExecute_Pod_ConditionNotFound(t *testing.T) {
	pod := resource("Pod", "v1", "test-pod", []map[string]interface{}{
		{"type": "Initialized", "status": "True"},
	})
	spec := vtypes.ConditionSpec{
		Target: vtypes.Target{Kind: "Pod", Name: "test-pod"},
		Checks: []vtypes.ConditionCheck{{Type: "Ready", Status: corev1.ConditionTrue}},
	}

	passed, msg, err := condition.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), pod)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "condition Ready not found")
}

func TestExecute_Deployment_Available(t *testing.T) {
	d := resource("Deployment", "apps/v1", "my-deploy", []map[string]interface{}{
		{"type": "Available", "status": "True"},
		{"type": "Progressing", "status": "True"},
	})
	spec := vtypes.ConditionSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "my-deploy"},
		Checks: []vtypes.ConditionCheck{{Type: "Available", Status: corev1.ConditionTrue}},
	}

	passed, msg, err := condition.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, "All checks passed", msg)
}

func TestExecute_Deployment_ConditionFalse(t *testing.T) {
	d := resource("Deployment", "apps/v1", "my-deploy", []map[string]interface{}{
		{"type": "Available", "status": "False"},
	})
	spec := vtypes.ConditionSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "my-deploy"},
		Checks: []vtypes.ConditionCheck{{Type: "Available", Status: corev1.ConditionTrue}},
	}

	passed, msg, err := condition.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "condition Available is not True")
}

func TestExecute_NoStatus(t *testing.T) {
	d := resource("Deployment", "apps/v1", "my-deploy", nil)
	spec := vtypes.ConditionSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "my-deploy"},
		Checks: []vtypes.ConditionCheck{{Type: "Available", Status: corev1.ConditionTrue}},
	}

	passed, msg, err := condition.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "no conditions in status")
}

func TestExecute_NoMatchingResources(t *testing.T) {
	spec := vtypes.ConditionSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "nonexistent"},
		Checks: []vtypes.ConditionCheck{{Type: "Available", Status: corev1.ConditionTrue}},
	}

	passed, _, err := condition.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())))
	assert.Error(t, err)
	assert.False(t, passed)
}

func TestExecute_NoChecks(t *testing.T) {
	spec := vtypes.ConditionSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "any"},
		Checks: nil,
	}
	passed, msg, err := condition.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Equal(t, "No checks specified", msg)
}
