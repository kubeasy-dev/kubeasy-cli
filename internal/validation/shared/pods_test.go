package shared_test

import (
	"context"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetTargetPods_ByName(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"},
	}
	deps := shared.Deps{
		Clientset: fake.NewClientset(pod),
		Namespace: "test-ns",
	}

	pods, err := shared.GetTargetPods(context.Background(), deps, vtypes.Target{Kind: "Pod", Name: "test-pod"})
	require.NoError(t, err)
	require.Len(t, pods, 1)
	assert.Equal(t, "test-pod", pods[0].Name)
}

func TestGetTargetPods_ByLabelSelector(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "test-ns", Labels: map[string]string{"app": "test"}},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-2", Namespace: "test-ns", Labels: map[string]string{"app": "test"}},
	}
	deps := shared.Deps{
		Clientset: fake.NewClientset(pod1, pod2),
		Namespace: "test-ns",
	}

	pods, err := shared.GetTargetPods(context.Background(), deps, vtypes.Target{Kind: "Pod", LabelSelector: map[string]string{"app": "test"}})
	require.NoError(t, err)
	assert.Len(t, pods, 2)
}

func TestGetPodsForResource_Deployment(t *testing.T) {
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-ns",
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{"app": "test"},
				},
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns", Labels: map[string]string{"app": "test"}},
	}
	deps := shared.Deps{
		Clientset:     fake.NewClientset(pod),
		DynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), deployment),
		Namespace:     "test-ns",
	}

	pods, err := shared.GetPodsForResource(context.Background(), deps, vtypes.Target{Kind: "Deployment", Name: "test-deployment"})
	require.NoError(t, err)
	require.Len(t, pods, 1)
	assert.Equal(t, "test-pod", pods[0].Name)
}
