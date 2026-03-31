package status_test

import (
	"context"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/status"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func deps(dynamicClient *dynamicfake.FakeDynamicClient) shared.Deps {
	return shared.Deps{DynamicClient: dynamicClient, Namespace: "test-ns"}
}

func deployment(name, namespace string, statusFields map[string]interface{}) *unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
	}
	if len(statusFields) > 0 {
		obj["status"] = statusFields
	}
	return &unstructured.Unstructured{Object: obj}
}

func TestExecute_Success(t *testing.T) {
	d := deployment("test-deployment", "test-ns", map[string]interface{}{"readyReplicas": int64(3)})
	spec := vtypes.StatusSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "test-deployment"},
		Checks: []vtypes.StatusCheck{{Field: "readyReplicas", Operator: "==", Value: int64(3)}},
	}

	passed, msg, err := status.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, "All status checks passed", msg)
}

func TestExecute_CheckFailed(t *testing.T) {
	d := deployment("test-deployment", "test-ns", map[string]interface{}{"readyReplicas": int64(1)})
	spec := vtypes.StatusSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "test-deployment"},
		Checks: []vtypes.StatusCheck{{Field: "readyReplicas", Operator: ">=", Value: int64(3)}},
	}

	passed, msg, err := status.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "got 1, expected >= 3")
}

func TestExecute_NoMatchingResources(t *testing.T) {
	spec := vtypes.StatusSpec{
		Target: vtypes.Target{Kind: "Pod", LabelSelector: map[string]string{"app": "nonexistent"}},
		Checks: []vtypes.StatusCheck{{Field: "readyReplicas", Operator: "==", Value: int64(0)}},
	}

	sc := runtime.NewScheme()
	_ = corev1.AddToScheme(sc)
	passed, msg, err := status.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(sc)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Equal(t, "No matching resources found", msg)
}

func TestExecute_NoTargetSpecified(t *testing.T) {
	spec := vtypes.StatusSpec{
		Target: vtypes.Target{Kind: "Deployment"},
		Checks: []vtypes.StatusCheck{{Field: "readyReplicas", Operator: "==", Value: int64(3)}},
	}

	passed, msg, err := status.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Equal(t, "No target name or labelSelector specified", msg)
}

func TestExecute_NoChecks(t *testing.T) {
	spec := vtypes.StatusSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "any"},
		Checks: nil,
	}

	passed, msg, err := status.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Equal(t, "No checks specified", msg)
}

func TestExecute_FieldNotFound(t *testing.T) {
	d := deployment("test-deployment", "test-ns", map[string]interface{}{})
	spec := vtypes.StatusSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "test-deployment"},
		Checks: []vtypes.StatusCheck{{Field: "nonexistentField", Operator: "==", Value: int64(0)}},
	}

	passed, msg, err := status.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "not found")
}

func TestExecute_InOperator_Passes(t *testing.T) {
	d := deployment("test-deployment", "test-ns", map[string]interface{}{"phase": "Succeeded"})
	spec := vtypes.StatusSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "test-deployment"},
		Checks: []vtypes.StatusCheck{{Field: "phase", Operator: "in", Value: []interface{}{"Running", "Succeeded"}}},
	}

	passed, msg, err := status.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, "All status checks passed", msg)
}

func TestExecute_InOperator_Fails(t *testing.T) {
	d := deployment("test-deployment", "test-ns", map[string]interface{}{"phase": "Failed"})
	spec := vtypes.StatusSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "test-deployment"},
		Checks: []vtypes.StatusCheck{{Field: "phase", Operator: "in", Value: []interface{}{"Running", "Succeeded"}}},
	}

	passed, msg, err := status.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "phase")
}

func TestExecute_ContainsOperator_Passes(t *testing.T) {
	d := deployment("test-deployment", "test-ns", map[string]interface{}{"message": "deployment completed successfully"})
	spec := vtypes.StatusSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "test-deployment"},
		Checks: []vtypes.StatusCheck{{Field: "message", Operator: "contains", Value: "successfully"}},
	}

	passed, msg, err := status.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, "All status checks passed", msg)
}

func TestExecute_ContainsOperator_Fails(t *testing.T) {
	d := deployment("test-deployment", "test-ns", map[string]interface{}{"message": "deployment is progressing"})
	spec := vtypes.StatusSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "test-deployment"},
		Checks: []vtypes.StatusCheck{{Field: "message", Operator: "contains", Value: "successfully"}},
	}

	passed, msg, err := status.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "message")
}
