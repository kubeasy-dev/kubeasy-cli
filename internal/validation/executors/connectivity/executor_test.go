package connectivity_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/connectivity"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func depsWithPod(pod *corev1.Pod) shared.Deps {
	return shared.Deps{
		Clientset:     fake.NewClientset(pod),
		DynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		RestConfig:    &rest.Config{},
		Namespace:     "test-ns",
	}
}

func TestExecute_NoSourcePods(t *testing.T) {
	spec := vtypes.ConnectivitySpec{
		SourcePod: vtypes.SourcePod{Name: "nonexistent"},
		Targets:   []vtypes.ConnectivityCheck{{URL: "http://svc:80", ExpectedStatusCode: 200}},
	}
	deps := shared.Deps{
		Clientset:  fake.NewClientset(),
		RestConfig: &rest.Config{},
		Namespace:  "test-ns",
	}

	passed, _, err := connectivity.Execute(context.Background(), spec, deps)
	assert.Error(t, err)
	assert.False(t, passed)
}

func TestExecute_NoRunningSoucePods(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "source-pod", Namespace: "test-ns", Labels: map[string]string{"app": "src"}},
		Status:     corev1.PodStatus{Phase: corev1.PodPending}, // not running
	}
	spec := vtypes.ConnectivitySpec{
		SourcePod: vtypes.SourcePod{LabelSelector: map[string]string{"app": "src"}},
		Targets:   []vtypes.ConnectivityCheck{{URL: "http://svc:80"}},
	}

	passed, msg, err := connectivity.Execute(context.Background(), spec, depsWithPod(pod))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Equal(t, "No running source pods found", msg)
}

func TestExecute_InternalMode_TestEnv(t *testing.T) {
	// When restConfig.Host is empty (test env), exec returns deterministic error
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "source-pod", Namespace: "test-ns"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	spec := vtypes.ConnectivitySpec{
		SourcePod: vtypes.SourcePod{Name: "source-pod"},
		Targets:   []vtypes.ConnectivityCheck{{URL: "http://svc:80", ExpectedStatusCode: 200}},
	}

	passed, msg, err := connectivity.Execute(context.Background(), spec, depsWithPod(pod))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "exec not available in test environment")
}

func TestExecute_InternalMode_BlockedExpected(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "source-pod", Namespace: "test-ns"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	spec := vtypes.ConnectivitySpec{
		SourcePod: vtypes.SourcePod{Name: "source-pod"},
		Targets:   []vtypes.ConnectivityCheck{{URL: "http://svc:80", ExpectedStatusCode: 0}},
	}

	passed, msg, err := connectivity.Execute(context.Background(), spec, depsWithPod(pod))
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Contains(t, msg, "All connectivity checks passed")
}

func TestExecute_ExternalMode_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	spec := vtypes.ConnectivitySpec{
		Mode: vtypes.ConnectivityModeExternal,
		Targets: []vtypes.ConnectivityCheck{
			{URL: srv.URL, ExpectedStatusCode: 200},
		},
	}
	deps := shared.Deps{Clientset: fake.NewClientset(), Namespace: "test-ns"}

	passed, msg, err := connectivity.Execute(context.Background(), spec, deps)
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, "All connectivity checks passed", msg)
}

func TestExecute_ExternalMode_WrongStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	spec := vtypes.ConnectivitySpec{
		Mode: vtypes.ConnectivityModeExternal,
		Targets: []vtypes.ConnectivityCheck{
			{URL: srv.URL, ExpectedStatusCode: 200},
		},
	}
	deps := shared.Deps{Clientset: fake.NewClientset(), Namespace: "test-ns"}

	passed, msg, err := connectivity.Execute(context.Background(), spec, deps)
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "got status 404, expected 200")
}

func TestExecute_ExternalMode_Blocked(t *testing.T) {
	// ExpectedStatusCode: 0 means "connection should fail"
	spec := vtypes.ConnectivitySpec{
		Mode: vtypes.ConnectivityModeExternal,
		Targets: []vtypes.ConnectivityCheck{
			{URL: "http://127.0.0.1:1", ExpectedStatusCode: 0},
		},
	}
	deps := shared.Deps{Clientset: fake.NewClientset(), Namespace: "test-ns"}

	passed, msg, err := connectivity.Execute(context.Background(), spec, deps)
	require.NoError(t, err)
	assert.True(t, passed)
	// When all targets pass (including blocked-as-expected), the summary message is returned
	assert.Equal(t, "All connectivity checks passed", msg)
}

func TestExecute_ExternalMode_HostHeader(t *testing.T) {
	var capturedHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHost = r.Host
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	spec := vtypes.ConnectivitySpec{
		Mode: vtypes.ConnectivityModeExternal,
		Targets: []vtypes.ConnectivityCheck{
			{URL: srv.URL, ExpectedStatusCode: 200, HostHeader: "my-virtual-host.example.com"},
		},
	}
	deps := shared.Deps{Clientset: fake.NewClientset(), Namespace: "test-ns"}

	passed, _, err := connectivity.Execute(context.Background(), spec, deps)
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, "my-virtual-host.example.com", capturedHost)
}
