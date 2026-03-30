package log_test

import (
	"context"
	"testing"

	executorlog "github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/log"
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

func TestExecute_NoMatchingPods(t *testing.T) {
	spec := vtypes.LogSpec{
		Target:          vtypes.Target{Kind: "Pod", Name: "nonexistent"},
		ExpectedStrings: []string{"hello"},
	}
	passed, _, err := executorlog.Execute(context.Background(), spec, deps(fake.NewClientset()))
	assert.Error(t, err)
	assert.False(t, passed)
}

func TestExecute_MissingString(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	}
	spec := vtypes.LogSpec{
		Target:          vtypes.Target{Kind: "Pod", Name: "test-pod"},
		ExpectedStrings: []string{"expected-string-not-in-logs"},
		SinceSeconds:    300,
	}

	// The fake clientset returns empty logs, so the expected string won't be found
	passed, msg, err := executorlog.Execute(context.Background(), spec, deps(fake.NewClientset(pod)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Missing strings in logs")
}

func TestExecute_ByLabelSelector(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod", Namespace: "test-ns",
			Labels: map[string]string{"app": "test"},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	}
	spec := vtypes.LogSpec{
		Target:          vtypes.Target{Kind: "Pod", LabelSelector: map[string]string{"app": "test"}},
		ExpectedStrings: []string{"some-string"},
		SinceSeconds:    300,
	}

	passed, msg, err := executorlog.Execute(context.Background(), spec, deps(fake.NewClientset(pod)))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "Missing strings in logs")
}
