package spec_test

import (
	"context"
	"testing"

	executorspec "github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/spec"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func boolPtr(b bool) *bool { return &b }

func deployment(name string, specFields map[string]interface{}) *unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": name, "namespace": "test-ns"},
	}
	if len(specFields) > 0 {
		obj["spec"] = specFields
	}
	return &unstructured.Unstructured{Object: obj}
}

func deps(d *dynamicfake.FakeDynamicClient) shared.Deps {
	return shared.Deps{DynamicClient: d, Namespace: "test-ns"}
}

func TestExecute_ExistsTrue(t *testing.T) {
	d := deployment("test", map[string]interface{}{"replicas": int64(3)})
	spec := vtypes.SpecSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "test"},
		Checks: []vtypes.SpecCheck{{Path: "spec.replicas", Exists: boolPtr(true)}},
	}

	passed, _, err := executorspec.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.True(t, passed)
}

func TestExecute_ExistsFalse_FieldAbsent(t *testing.T) {
	d := deployment("test", nil)
	spec := vtypes.SpecSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "test"},
		Checks: []vtypes.SpecCheck{{Path: "spec.replicas", Exists: boolPtr(false)}},
	}

	passed, _, err := executorspec.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.True(t, passed)
}

func TestExecute_ValueMatch(t *testing.T) {
	d := deployment("test", map[string]interface{}{"replicas": int64(3)})
	spec := vtypes.SpecSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "test"},
		Checks: []vtypes.SpecCheck{{Path: "spec.replicas", Value: int64(3)}},
	}

	passed, _, err := executorspec.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.True(t, passed)
}

func TestExecute_ValueMismatch(t *testing.T) {
	d := deployment("test", map[string]interface{}{"replicas": int64(1)})
	spec := vtypes.SpecSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "test"},
		Checks: []vtypes.SpecCheck{{Path: "spec.replicas", Value: int64(3)}},
	}

	passed, msg, err := executorspec.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "got 1, expected 3")
}

func TestExecute_ContainsFound(t *testing.T) {
	d := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]interface{}{"name": "test", "namespace": "test-ns"},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "app", "image": "nginx:latest"},
					},
				},
			},
		},
	}}
	spec := vtypes.SpecSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "test"},
		Checks: []vtypes.SpecCheck{{
			Path:     "spec.template.spec.containers",
			Contains: map[string]interface{}{"name": "app"},
		}},
	}

	passed, _, err := executorspec.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.True(t, passed)
}

func TestExecute_ContainsNotFound(t *testing.T) {
	d := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]interface{}{"name": "test", "namespace": "test-ns"},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "other"},
					},
				},
			},
		},
	}}
	spec := vtypes.SpecSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "test"},
		Checks: []vtypes.SpecCheck{{
			Path:     "spec.template.spec.containers",
			Contains: map[string]interface{}{"name": "app"},
		}},
	}

	passed, msg, err := executorspec.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), d)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "no element matches")
}

func TestExecute_NoChecks(t *testing.T) {
	spec := vtypes.SpecSpec{
		Target: vtypes.Target{Kind: "Deployment", Name: "any"},
		Checks: nil,
	}
	passed, msg, err := executorspec.Execute(context.Background(), spec, deps(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Equal(t, "No checks specified", msg)
}
