package deployer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const testNamespace = "test-ns"

// makeProbePod creates a probe pod object for test seeding.
func makeProbePod(namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ProbePodName,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}

// TestCreateProbePod verifies that CreateProbePod creates a pod with the correct
// name, labels, RestartPolicy, and container configuration.
func TestCreateProbePod(t *testing.T) {
	clientset := fake.NewClientset()

	pod, err := CreateProbePod(context.Background(), clientset, testNamespace)
	require.NoError(t, err)
	require.NotNil(t, pod)

	assert.Equal(t, ProbePodName, pod.Name)
	assert.Equal(t, testNamespace, pod.Namespace)
	assert.Equal(t, "kubeasy-probe", pod.Labels["app"])
	assert.Equal(t, "kubeasy", pod.Labels["managed-by"])
	assert.Equal(t, corev1.RestartPolicyNever, pod.Spec.RestartPolicy)
	require.Len(t, pod.Spec.Containers, 1)
	assert.Equal(t, "curl", pod.Spec.Containers[0].Name)
}

// TestCreateProbePod_StaleExists verifies that CreateProbePod deletes a stale pod
// before creating a fresh one.
func TestCreateProbePod_StaleExists(t *testing.T) {
	stale := makeProbePod(testNamespace)
	clientset := fake.NewClientset(stale)

	pod, err := CreateProbePod(context.Background(), clientset, testNamespace)
	require.NoError(t, err)
	require.NotNil(t, pod)
	assert.Equal(t, ProbePodName, pod.Name)
}

// TestDeleteProbePod_NotFound verifies that DeleteProbePod returns nil when the
// probe pod does not exist (idempotent).
func TestDeleteProbePod_NotFound(t *testing.T) {
	clientset := fake.NewClientset()

	err := DeleteProbePod(context.Background(), clientset, testNamespace)
	assert.NoError(t, err)
}

// TestDeleteProbePod_Exists verifies that DeleteProbePod removes an existing probe pod.
func TestDeleteProbePod_Exists(t *testing.T) {
	clientset := fake.NewClientset(makeProbePod(testNamespace))

	err := DeleteProbePod(context.Background(), clientset, testNamespace)
	require.NoError(t, err)

	_, err = clientset.CoreV1().Pods(testNamespace).Get(context.Background(), ProbePodName, metav1.GetOptions{})
	assert.Error(t, err, "probe pod should be deleted")
}

// TestDeleteProbePod_CancelledContext verifies that DeleteProbePod succeeds even
// when the caller's context is already cancelled (PROBE-03 independent-context contract).
func TestDeleteProbePod_CancelledContext(t *testing.T) {
	clientset := fake.NewClientset(makeProbePod(testNamespace))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel the context

	err := DeleteProbePod(ctx, clientset, testNamespace)
	assert.NoError(t, err, "DeleteProbePod should succeed even when caller context is cancelled")
}

// TestWaitForProbePodReady_Running verifies that WaitForProbePodReady returns nil
// immediately when the probe pod is already in Running phase.
func TestWaitForProbePodReady_Running(t *testing.T) {
	pod := makeProbePod(testNamespace)
	pod.Status.Phase = corev1.PodRunning
	clientset := fake.NewClientset(pod)

	err := WaitForProbePodReady(context.Background(), clientset, testNamespace)
	assert.NoError(t, err)
}

// TestWaitForProbePodReady_Timeout verifies that WaitForProbePodReady returns an
// error when the pod does not reach Running phase within the context deadline.
func TestWaitForProbePodReady_Timeout(t *testing.T) {
	pod := makeProbePod(testNamespace)
	pod.Status.Phase = corev1.PodPending
	clientset := fake.NewClientset(pod)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := WaitForProbePodReady(ctx, clientset, testNamespace)
	assert.Error(t, err, "WaitForProbePodReady should return error when pod stays in Pending phase")
}
