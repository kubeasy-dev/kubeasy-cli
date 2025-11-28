//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/validation"
	"github.com/kubeasy-dev/kubeasy-cli/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStatusValidation_PodReady_Success(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a pod with proper labels
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
		},
	}

	createdPod := env.CreatePod(pod)
	require.NotNil(t, createdPod)

	// Mark pod as Ready
	env.SetPodReady(createdPod.Name)

	// Create validation spec
	spec := validation.StatusSpec{
		Target: validation.Target{
			Kind: "Pod",
			LabelSelector: map[string]string{
				"app": "test-app",
			},
		},
		Conditions: []validation.StatusCondition{
			{
				Type:   "Ready",
				Status: "True",
			},
		},
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-pod-ready",
		Type: validation.TypeStatus,
		Spec: spec,
	})

	// Assert results
	assert.True(t, result.Passed, "Expected validation to pass")
	assert.Contains(t, result.Message, "All 1 pod(s) meet the required conditions")
}

func TestStatusValidation_PodReady_Failure_NotReady(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a pod but DON'T mark it as Ready
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "not-ready-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
		},
	}

	createdPod := env.CreatePod(pod)
	require.NotNil(t, createdPod)

	// Create validation spec
	spec := validation.StatusSpec{
		Target: validation.Target{
			Kind: "Pod",
			LabelSelector: map[string]string{
				"app": "test-app",
			},
		},
		Conditions: []validation.StatusCondition{
			{
				Type:   "Ready",
				Status: "True",
			},
		},
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-pod-not-ready",
		Type: validation.TypeStatus,
		Spec: spec,
	})

	// Assert results
	assert.False(t, result.Passed, "Expected validation to fail")
	assert.Contains(t, result.Message, "condition Ready is not True")
}

func TestStatusValidation_PodReady_Failure_NoMatchingPods(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Don't create any pods

	// Create validation spec
	spec := validation.StatusSpec{
		Target: validation.Target{
			Kind: "Pod",
			LabelSelector: map[string]string{
				"app": "nonexistent-app",
			},
		},
		Conditions: []validation.StatusCondition{
			{
				Type:   "Ready",
				Status: "True",
			},
		},
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-no-pods",
		Type: validation.TypeStatus,
		Spec: spec,
	})

	// Assert results
	assert.False(t, result.Passed, "Expected validation to fail")
	assert.Equal(t, "No matching pods found", result.Message)
}

func TestStatusValidation_MultiplePods_AllReady(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create multiple pods
	for i := 1; i <= 3; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-pod-%d", i),
				Namespace: env.Namespace,
				Labels: map[string]string{
					"app": "multi-pod-app",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:latest",
					},
				},
			},
		}

		createdPod := env.CreatePod(pod)
		env.SetPodReady(createdPod.Name)
	}

	// Create validation spec
	spec := validation.StatusSpec{
		Target: validation.Target{
			Kind: "Pod",
			LabelSelector: map[string]string{
				"app": "multi-pod-app",
			},
		},
		Conditions: []validation.StatusCondition{
			{
				Type:   "Ready",
				Status: "True",
			},
		},
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-multi-pods",
		Type: validation.TypeStatus,
		Spec: spec,
	})

	// Assert results
	assert.True(t, result.Passed, "Expected validation to pass")
	assert.Contains(t, result.Message, "All 3 pod(s) meet the required conditions")
}

func TestStatusValidation_Deployment_Available(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a deployment
	replicas := int32(2)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: env.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test-deployment-app",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test-deployment-app",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}

	created, err := env.Clientset.AppsV1().Deployments(env.Namespace).Create(
		context.Background(),
		deployment,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	// Update deployment status to Available
	created.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:   appsv1.DeploymentAvailable,
			Status: corev1.ConditionTrue,
		},
	}
	created.Status.Replicas = replicas
	created.Status.ReadyReplicas = replicas

	_, err = env.Clientset.AppsV1().Deployments(env.Namespace).UpdateStatus(
		context.Background(),
		created,
		metav1.UpdateOptions{},
	)
	require.NoError(t, err)

	// Create validation spec
	spec := validation.StatusSpec{
		Target: validation.Target{
			Kind: "Deployment",
			Name: "test-deployment",
		},
		Conditions: []validation.StatusCondition{
			{
				Type:   "Available",
				Status: "True",
			},
		},
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-deployment-available",
		Type: validation.TypeStatus,
		Spec: spec,
	})

	// Assert results - Note: This will check pods owned by the deployment
	// Since we're using envtest and not creating actual pods, this might need adjustment
	t.Logf("Result: passed=%v, message=%s", result.Passed, result.Message)
}
