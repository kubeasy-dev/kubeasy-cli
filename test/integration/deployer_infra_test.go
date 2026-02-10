//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/deployer"
	"github.com/kubeasy-dev/kubeasy-cli/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsInfrastructureReady_NoNamespace(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// No kyverno or local-path-storage namespaces exist
	ready, err := deployer.IsInfrastructureReadyWithClient(context.Background(), env.Clientset)
	require.NoError(t, err)
	assert.False(t, ready, "should be false when no infrastructure namespaces exist")
}

func TestIsInfrastructureReady_NamespaceOnly(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	// Create kyverno namespace but no deployments
	kyvernoNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kyverno"},
	}
	_, err := env.Clientset.CoreV1().Namespaces().Create(ctx, kyvernoNS, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = env.Clientset.CoreV1().Namespaces().Delete(ctx, "kyverno", metav1.DeleteOptions{})
	})

	ready, err := deployer.IsInfrastructureReadyWithClient(ctx, env.Clientset)
	require.NoError(t, err)
	assert.False(t, ready, "should be false when kyverno deployments are missing")
}

func TestIsInfrastructureReady_DeploymentsNotReady(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	// Create namespaces
	for _, nsName := range []string{"kyverno", "local-path-storage"} {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: nsName},
		}
		_, err := env.Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = env.Clientset.CoreV1().Namespaces().Delete(ctx, nsName, metav1.DeleteOptions{})
		})
	}

	// Create kyverno deployments with 0 ready replicas
	replicas := int32(1)
	kyvernoDeployments := []string{
		"kyverno-admission-controller",
		"kyverno-background-controller",
		"kyverno-cleanup-controller",
		"kyverno-reports-controller",
	}

	for _, name := range kyvernoDeployments {
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "kyverno",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": name},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": name},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "main", Image: "busybox"},
						},
					},
				},
			},
			Status: appsv1.DeploymentStatus{
				Replicas:      1,
				ReadyReplicas: 0, // Not ready
			},
		}
		_, err := env.Clientset.AppsV1().Deployments("kyverno").Create(ctx, dep, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	ready, err := deployer.IsInfrastructureReadyWithClient(ctx, env.Clientset)
	require.NoError(t, err)
	assert.False(t, ready, "should be false when deployments have 0 ready replicas")
}

func TestIsInfrastructureReady_FullyReady(t *testing.T) {
	env := helpers.SetupEnvTest(t)
	ctx := context.Background()

	// Create namespaces
	for _, nsName := range []string{"kyverno", "local-path-storage"} {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: nsName},
		}
		_, err := env.Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = env.Clientset.CoreV1().Namespaces().Delete(ctx, nsName, metav1.DeleteOptions{})
		})
	}

	// Create all deployments with ready replicas
	replicas := int32(1)
	deployments := map[string][]string{
		"kyverno": {
			"kyverno-admission-controller",
			"kyverno-background-controller",
			"kyverno-cleanup-controller",
			"kyverno-reports-controller",
		},
		"local-path-storage": {
			"local-path-provisioner",
		},
	}

	for namespace, names := range deployments {
		for _, name := range names {
			dep := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": name},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": name},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "main", Image: "busybox"},
							},
						},
					},
				},
				Status: appsv1.DeploymentStatus{
					Replicas:      1,
					ReadyReplicas: 1, // Ready
				},
			}
			created, err := env.Clientset.AppsV1().Deployments(namespace).Create(ctx, dep, metav1.CreateOptions{})
			require.NoError(t, err)

			// Update status (envtest might not apply status on create)
			created.Status.Replicas = 1
			created.Status.ReadyReplicas = 1
			_, err = env.Clientset.AppsV1().Deployments(namespace).UpdateStatus(ctx, created, metav1.UpdateOptions{})
			require.NoError(t, err)
		}
	}

	ready, err := deployer.IsInfrastructureReadyWithClient(ctx, env.Clientset)
	require.NoError(t, err)
	assert.True(t, ready, "should be true when all components are ready")
}
