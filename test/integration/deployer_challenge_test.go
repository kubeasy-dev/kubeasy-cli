//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// =============================================================================
// WaitForDeploymentsReady
// =============================================================================

func TestWaitForDeploymentsReady_AllReady(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	replicas := int32(1)
	names := []string{"web", "api"}

	for _, name := range names {
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: env.Namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": name},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "main", Image: "busybox"}},
					},
				},
			},
		}
		created, err := env.Clientset.AppsV1().Deployments(env.Namespace).Create(ctx, dep, metav1.CreateOptions{})
		require.NoError(t, err)

		// Simulate ready status (envtest doesn't run controllers)
		created.Status = appsv1.DeploymentStatus{
			Replicas:            1,
			ReadyReplicas:       1,
			UpdatedReplicas:     1,
			AvailableReplicas:   1,
			ObservedGeneration:  created.Generation,
		}
		_, err = env.Clientset.AppsV1().Deployments(env.Namespace).UpdateStatus(ctx, created, metav1.UpdateOptions{})
		require.NoError(t, err)
	}

	err := kube.WaitForDeploymentsReady(ctx, env.Clientset, env.Namespace, names)
	assert.NoError(t, err, "should return nil when all deployments are ready")
}

func TestWaitForDeploymentsReady_NotReady_Timeout(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stuck",
			Namespace: env.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "stuck"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "stuck"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "busybox"}},
				},
			},
		},
	}
	created, err := env.Clientset.AppsV1().Deployments(env.Namespace).Create(ctx, dep, metav1.CreateOptions{})
	require.NoError(t, err)

	// Set 0 ready replicas — deployment never becomes ready
	created.Status = appsv1.DeploymentStatus{
		Replicas:           1,
		ReadyReplicas:      0,
		ObservedGeneration: created.Generation,
	}
	_, err = env.Clientset.AppsV1().Deployments(env.Namespace).UpdateStatus(ctx, created, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Use a short timeout to trigger the timeout path
	shortCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = kube.WaitForDeploymentsReady(shortCtx, env.Clientset, env.Namespace, []string{"stuck"})
	assert.Error(t, err, "should timeout when deployment is not ready")
	assert.Contains(t, err.Error(), "timeout")
}

func TestWaitForDeploymentsReady_NotFound_Timeout(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Don't create any deployment — it should timeout waiting for it to appear
	shortCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := kube.WaitForDeploymentsReady(shortCtx, env.Clientset, env.Namespace, []string{"nonexistent"})
	assert.Error(t, err, "should timeout when deployment does not exist")
	assert.Contains(t, err.Error(), "timeout")
}

func TestWaitForDeploymentsReady_EmptyList(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// No deployments to wait for — should return immediately
	err := kube.WaitForDeploymentsReady(context.Background(), env.Clientset, env.Namespace, []string{})
	assert.NoError(t, err, "should return nil for empty deployment list")
}

func TestWaitForDeploymentsReady_MultipleReplicas(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	replicas := int32(3)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scaled",
			Namespace: env.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "scaled"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "scaled"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "busybox"}},
				},
			},
		},
	}
	created, err := env.Clientset.AppsV1().Deployments(env.Namespace).Create(ctx, dep, metav1.CreateOptions{})
	require.NoError(t, err)

	created.Status = appsv1.DeploymentStatus{
		Replicas:            3,
		ReadyReplicas:       3,
		UpdatedReplicas:     3,
		AvailableReplicas:   3,
		ObservedGeneration:  created.Generation,
	}
	_, err = env.Clientset.AppsV1().Deployments(env.Namespace).UpdateStatus(ctx, created, metav1.UpdateOptions{})
	require.NoError(t, err)

	err = kube.WaitForDeploymentsReady(ctx, env.Clientset, env.Namespace, []string{"scaled"})
	assert.NoError(t, err, "should succeed when all 3 replicas are ready")
}

// =============================================================================
// WaitForStatefulSetsReady
// =============================================================================

func TestWaitForStatefulSetsReady_AllReady(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	replicas := int32(1)
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db",
			Namespace: env.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: "db",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "db"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "db"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "busybox"}},
				},
			},
		},
	}
	created, err := env.Clientset.AppsV1().StatefulSets(env.Namespace).Create(ctx, sts, metav1.CreateOptions{})
	require.NoError(t, err)

	created.Status = appsv1.StatefulSetStatus{
		Replicas:           1,
		ReadyReplicas:      1,
		UpdatedReplicas:    1,
		CurrentRevision:    "db-abc",
		UpdateRevision:     "db-abc",
		ObservedGeneration: created.Generation,
	}
	_, err = env.Clientset.AppsV1().StatefulSets(env.Namespace).UpdateStatus(ctx, created, metav1.UpdateOptions{})
	require.NoError(t, err)

	err = kube.WaitForStatefulSetsReady(ctx, env.Clientset, env.Namespace, []string{"db"})
	assert.NoError(t, err, "should return nil when statefulset is ready")
}

func TestWaitForStatefulSetsReady_NotReady_Timeout(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	replicas := int32(1)
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stuck-db",
			Namespace: env.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: "stuck-db",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "stuck-db"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "stuck-db"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "busybox"}},
				},
			},
		},
	}
	created, err := env.Clientset.AppsV1().StatefulSets(env.Namespace).Create(ctx, sts, metav1.CreateOptions{})
	require.NoError(t, err)

	created.Status = appsv1.StatefulSetStatus{
		Replicas:           1,
		ReadyReplicas:      0,
		ObservedGeneration: created.Generation,
	}
	_, err = env.Clientset.AppsV1().StatefulSets(env.Namespace).UpdateStatus(ctx, created, metav1.UpdateOptions{})
	require.NoError(t, err)

	shortCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = kube.WaitForStatefulSetsReady(shortCtx, env.Clientset, env.Namespace, []string{"stuck-db"})
	assert.Error(t, err, "should timeout when statefulset is not ready")
	assert.Contains(t, err.Error(), "timeout")
}

func TestWaitForStatefulSetsReady_RevisionMismatch_Timeout(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	replicas := int32(1)
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rolling-db",
			Namespace: env.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: "rolling-db",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "rolling-db"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "rolling-db"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "busybox"}},
				},
			},
		},
	}
	created, err := env.Clientset.AppsV1().StatefulSets(env.Namespace).Create(ctx, sts, metav1.CreateOptions{})
	require.NoError(t, err)

	// Replicas ready but revision mismatch (rolling update in progress)
	created.Status = appsv1.StatefulSetStatus{
		Replicas:           1,
		ReadyReplicas:      1,
		UpdatedReplicas:    1,
		CurrentRevision:    "rolling-db-old",
		UpdateRevision:     "rolling-db-new",
		ObservedGeneration: created.Generation,
	}
	_, err = env.Clientset.AppsV1().StatefulSets(env.Namespace).UpdateStatus(ctx, created, metav1.UpdateOptions{})
	require.NoError(t, err)

	shortCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = kube.WaitForStatefulSetsReady(shortCtx, env.Clientset, env.Namespace, []string{"rolling-db"})
	assert.Error(t, err, "should timeout when revision mismatch indicates rolling update")
	assert.Contains(t, err.Error(), "timeout")
}
