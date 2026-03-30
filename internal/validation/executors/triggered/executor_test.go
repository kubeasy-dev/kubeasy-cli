package triggered

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	ktesting "k8s.io/client-go/testing"
)

func testDeps(objs ...runtime.Object) shared.Deps {
	return shared.Deps{
		Clientset:     fake.NewClientset(objs...),
		DynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		RestConfig:    &rest.Config{},
		Namespace:     "test-ns",
	}
}

func passingExecFn(_ context.Context, v vtypes.Validation) vtypes.Result {
	return vtypes.Result{Key: v.Key, Passed: true, Message: "ok"}
}

func failingExecFn(_ context.Context, v vtypes.Validation) vtypes.Result {
	return vtypes.Result{Key: v.Key, Passed: false, Message: "failed"}
}

// --- Execute orchestration tests ---

func TestExecute_WaitTrigger_ThenPass(t *testing.T) {
	spec := vtypes.TriggeredSpec{
		Trigger:          vtypes.TriggerConfig{Type: vtypes.TriggerTypeWait, WaitSeconds: 0},
		WaitAfterSeconds: 0,
		Then: []vtypes.Validation{
			{Key: "check-1", Type: vtypes.TypeCondition, Spec: vtypes.ConditionSpec{}},
		},
	}

	passed, msg, err := Execute(context.Background(), spec, testDeps(), passingExecFn)
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Contains(t, msg, "all 1 then validator(s) passed")
}

func TestExecute_WaitTrigger_ThenFail(t *testing.T) {
	spec := vtypes.TriggeredSpec{
		Trigger: vtypes.TriggerConfig{Type: vtypes.TriggerTypeWait, WaitSeconds: 0},
		Then: []vtypes.Validation{
			{Key: "check-1", Type: vtypes.TypeCondition, Spec: vtypes.ConditionSpec{}},
		},
	}

	passed, msg, err := Execute(context.Background(), spec, testDeps(), failingExecFn)
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Then validators failed")
}

func TestExecute_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	spec := vtypes.TriggeredSpec{
		Trigger:          vtypes.TriggerConfig{Type: vtypes.TriggerTypeWait, WaitSeconds: 0},
		WaitAfterSeconds: 10, // would block if context not cancelled
		Then:             []vtypes.Validation{{Key: "k", Type: vtypes.TypeCondition}},
	}

	_, _, err := Execute(ctx, spec, testDeps(), passingExecFn)
	assert.Error(t, err)
}

func TestExecute_TriggerFailurePropagates(t *testing.T) {
	spec := vtypes.TriggeredSpec{
		Trigger: vtypes.TriggerConfig{
			Type:   vtypes.TriggerTypeDelete,
			Target: &vtypes.Target{Kind: "UnknownKind", Name: "some-resource"},
		},
		Then: []vtypes.Validation{{Key: "k", Type: vtypes.TypeCondition}},
	}

	passed, msg, err := Execute(context.Background(), spec, testDeps(), passingExecFn)
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Trigger failed")
}

// --- executeTriggerDelete tests ---

func TestExecuteTriggerDelete_ByName(t *testing.T) {
	sc := runtime.NewScheme()
	pod := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]interface{}{"name": "target-pod", "namespace": "test-ns"},
	}}
	deps := shared.Deps{
		Clientset:     fake.NewClientset(),
		DynamicClient: dynamicfake.NewSimpleDynamicClient(sc, pod),
		Namespace:     "test-ns",
	}
	trigger := vtypes.TriggerConfig{
		Type:   vtypes.TriggerTypeDelete,
		Target: &vtypes.Target{Kind: "Pod", Name: "target-pod"},
	}

	err := executeTriggerDelete(context.Background(), trigger, deps)
	require.NoError(t, err)
}

func TestExecuteTriggerDelete_ByLabelSelector(t *testing.T) {
	sc := runtime.NewScheme()
	pod := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]interface{}{
			"name": "target-pod", "namespace": "test-ns",
			"labels": map[string]interface{}{"app": "target"},
		},
	}}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(sc, pod)

	var deletedBySelector string
	dynamicClient.PrependReactor("delete-collection", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		deletedBySelector = action.(ktesting.DeleteCollectionAction).GetListRestrictions().Labels.String()
		return true, nil, nil
	})
	dynamicClient.PrependReactor("list", "pods", func(_ ktesting.Action) (bool, runtime.Object, error) {
		return true, &unstructured.UnstructuredList{Items: []unstructured.Unstructured{*pod}}, nil
	})

	deps := shared.Deps{
		Clientset:     fake.NewClientset(),
		DynamicClient: dynamicClient,
		Namespace:     "test-ns",
	}
	trigger := vtypes.TriggerConfig{
		Type:   vtypes.TriggerTypeDelete,
		Target: &vtypes.Target{Kind: "Pod", LabelSelector: map[string]string{"app": "target"}},
	}

	err := executeTriggerDelete(context.Background(), trigger, deps)
	require.NoError(t, err)
	assert.Contains(t, deletedBySelector, "app=target")
}

func TestExecuteTriggerDelete_ByLabelSelector_NoMatch(t *testing.T) {
	sc := runtime.NewScheme()
	_ = corev1.AddToScheme(sc)
	deps := shared.Deps{
		Clientset:     fake.NewClientset(),
		DynamicClient: dynamicfake.NewSimpleDynamicClient(sc),
		Namespace:     "test-ns",
	}
	trigger := vtypes.TriggerConfig{
		Type:   vtypes.TriggerTypeDelete,
		Target: &vtypes.Target{Kind: "Pod", LabelSelector: map[string]string{"app": "typo"}},
	}

	err := executeTriggerDelete(context.Background(), trigger, deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no Pod resources found")
}

// --- executeTriggerWait tests ---

func TestExecuteTriggerWait_ZeroSeconds(t *testing.T) {
	trigger := vtypes.TriggerConfig{Type: vtypes.TriggerTypeWait, WaitSeconds: 0}
	err := executeTriggerWait(context.Background(), trigger)
	require.NoError(t, err)
}

func TestExecuteTriggerWait_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	trigger := vtypes.TriggerConfig{Type: vtypes.TriggerTypeWait, WaitSeconds: 60}
	err := executeTriggerWait(ctx, trigger)
	assert.Error(t, err)
}

// --- executeTriggerScale tests ---

func TestExecuteTriggerScale(t *testing.T) {
	sc := runtime.NewScheme()
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]interface{}{"name": "my-deploy", "namespace": "test-ns"},
		"spec":     map[string]interface{}{"replicas": int64(1)},
	}}
	deps := shared.Deps{
		Clientset:     fake.NewClientset(),
		DynamicClient: dynamicfake.NewSimpleDynamicClient(sc, obj),
		Namespace:     "test-ns",
	}
	replicas := int32(3)
	trigger := vtypes.TriggerConfig{
		Type:     vtypes.TriggerTypeScale,
		Target:   &vtypes.Target{Kind: "Deployment", Name: "my-deploy"},
		Replicas: &replicas,
	}

	err := executeTriggerScale(context.Background(), trigger, deps)
	require.NoError(t, err)
}

// --- executeTriggerLoad host-based test ---

func TestExecuteTriggerLoad_HostBased(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	trigger := vtypes.TriggerConfig{
		Type:              vtypes.TriggerTypeLoad,
		URL:               srv.URL,
		RequestsPerSecond: 5,
		DurationSeconds:   1,
	}

	err := executeTriggerLoad(context.Background(), trigger, testDeps())
	require.NoError(t, err)
	mu.Lock()
	count := requestCount
	mu.Unlock()
	// At 5 rps for 1 second, expect ~5 requests; allow ±2 for timing jitter
	assert.GreaterOrEqual(t, count, 3, "expected at least 3 requests")
}

// --- firstContainerName tests ---

func TestFirstContainerName(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "my-container"},
					},
				},
			},
		},
	}}
	name, err := firstContainerName(obj)
	require.NoError(t, err)
	assert.Equal(t, "my-container", name)
}

func TestFirstContainerName_Empty(t *testing.T) {
	_, err := firstContainerName(&unstructured.Unstructured{Object: map[string]interface{}{}})
	assert.Error(t, err)
}
