package deployer

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// ptr returns a pointer to the given value. Used for optional API fields like GracePeriodSeconds.
func ptr[T any](v T) *T { return &v }

// CreateProbePod creates the kubeasy-probe pod in the given namespace.
// If a stale probe pod already exists it is deleted before the new pod is created.
// The pod runs curlimages/curl:VERSION with RestartPolicy:Never and minimal resource requests
// so it can be used as a connectivity probe from within the cluster.
func CreateProbePod(ctx context.Context, clientset kubernetes.Interface, namespace string) (*corev1.Pod, error) {
	// Delete any stale pod first (ignore error — it might not exist).
	_ = deleteProbePodWithCtx(ctx, clientset, namespace)

	// Wait until the stale pod is gone before creating a fresh one.
	_ = wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 10*time.Second, true,
		func(ctx context.Context) (bool, error) {
			_, err := clientset.CoreV1().Pods(namespace).Get(ctx, ProbePodName, metav1.GetOptions{})
			return apierrors.IsNotFound(err), nil
		})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ProbePodName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":        "kubeasy-probe",
				"managed-by": "kubeasy",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:            "curl",
					Image:           probePodImage(),
					Command:         []string{"sleep", "infinity"},
					ImagePullPolicy: corev1.PullIfNotPresent,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("16Mi"),
						},
					},
				},
			},
		},
	}

	return clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
}

// DeleteProbePod deletes the kubeasy-probe pod from the given namespace.
// Returns nil if the pod does not exist (idempotent).
//
// PROBE-03 contract: uses an independent context.Background()+10s timeout internally
// rather than the caller's context, to guarantee cleanup even when the caller context
// has been cancelled (e.g., during error teardown or test cleanup).
// The context.Context parameter is accepted for API consistency but deliberately ignored.
//
//nolint:revive // context is intentionally discarded — see PROBE-03 contract above
func DeleteProbePod(_ context.Context, clientset kubernetes.Interface, namespace string) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return deleteProbePodWithCtx(cleanupCtx, clientset, namespace)
}

// deleteProbePodWithCtx is the internal implementation that accepts an explicit context.
// Used by both CreateProbePod (which passes the caller's ctx for the pre-delete step)
// and DeleteProbePod (which passes an independent cleanup context).
func deleteProbePodWithCtx(ctx context.Context, clientset kubernetes.Interface, namespace string) error {
	err := clientset.CoreV1().Pods(namespace).Delete(ctx, ProbePodName, metav1.DeleteOptions{
		GracePeriodSeconds: ptr(int64(0)),
	})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// WaitForProbePodReady polls until the kubeasy-probe pod reaches Running phase or the
// context deadline is exceeded. Uses a 1s poll interval with the context controlling
// the overall timeout (callers should set an appropriate deadline on ctx).
func WaitForProbePodReady(ctx context.Context, clientset kubernetes.Interface, namespace string) error {
	return wait.PollUntilContextTimeout(ctx, 1*time.Second, 30*time.Second, true,
		func(ctx context.Context) (bool, error) {
			pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, ProbePodName, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			return pod.Status.Phase == corev1.PodRunning, nil
		})
}
