//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
	"github.com/kubeasy-dev/kubeasy-cli/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStatusValidation_DeploymentReplicas_Success(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a deployment
	replicas := int32(3)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: env.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test-app",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
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
			},
		},
	}

	created, err := env.Clientset.AppsV1().Deployments(env.Namespace).Create(
		context.Background(),
		deployment,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	// Update deployment status
	created.Status.Replicas = 3
	created.Status.ReadyReplicas = 3
	_, err = env.Clientset.AppsV1().Deployments(env.Namespace).UpdateStatus(
		context.Background(),
		created,
		metav1.UpdateOptions{},
	)
	require.NoError(t, err)

	// Create validation spec - field paths are relative to status (no prefix needed)
	spec := validation.StatusSpec{
		Target: validation.Target{
			Kind: "Deployment",
			Name: "test-deployment",
		},
		Checks: []validation.StatusCheck{
			{
				Field:    "replicas",
				Operator: "==",
				Value:    int64(3),
			},
			{
				Field:    "readyReplicas",
				Operator: ">=",
				Value:    int64(2),
			},
		},
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-deployment-replicas",
		Type: validation.TypeStatus,
		Spec: spec,
	})

	// Assert results
	assert.True(t, result.Passed, "Expected validation to pass")
	assert.Equal(t, "All status checks passed", result.Message)
}

func TestStatusValidation_DeploymentReplicas_Failure(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a deployment
	replicas := int32(3)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: env.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test-app",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
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
			},
		},
	}

	created, err := env.Clientset.AppsV1().Deployments(env.Namespace).Create(
		context.Background(),
		deployment,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	// Update deployment status - only 1 ready replica instead of expected 3
	created.Status.Replicas = 3
	created.Status.ReadyReplicas = 1
	_, err = env.Clientset.AppsV1().Deployments(env.Namespace).UpdateStatus(
		context.Background(),
		created,
		metav1.UpdateOptions{},
	)
	require.NoError(t, err)

	// Create validation spec expecting at least 2 ready replicas
	spec := validation.StatusSpec{
		Target: validation.Target{
			Kind: "Deployment",
			Name: "test-deployment",
		},
		Checks: []validation.StatusCheck{
			{
				Field:    "readyReplicas",
				Operator: ">=",
				Value:    int64(2),
			},
		},
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-deployment-replicas-fail",
		Type: validation.TypeStatus,
		Spec: spec,
	})

	// Assert results
	assert.False(t, result.Passed, "Expected validation to fail")
	assert.Contains(t, result.Message, "readyReplicas")
	assert.Contains(t, result.Message, "got 1")
	assert.Contains(t, result.Message, "expected >= 2")
}

func TestStatusValidation_AllOperators(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	tests := []struct {
		name     string
		operator string
		value    int64
		actual   int32
		want     bool
	}{
		{"equal-pass", "==", 3, 3, true},
		{"equal-fail", "==", 5, 3, false},
		{"not-equal-pass", "!=", 5, 3, true},
		{"not-equal-fail", "!=", 3, 3, false},
		{"greater-than-pass", ">", 2, 3, true},
		{"greater-than-fail", ">", 3, 3, false},
		{"less-than-pass", "<", 5, 3, true},
		{"less-than-fail", "<", 2, 3, false},
		{"greater-equal-pass", ">=", 3, 3, true},
		{"greater-equal-fail", ">=", 4, 3, false},
		{"less-equal-pass", "<=", 3, 3, true},
		{"less-equal-fail", "<=", 2, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a deployment
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-" + tt.name,
					Namespace: env.Namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &tt.actual,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-app-" + tt.name,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-app-" + tt.name,
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

			// Update status
			created.Status.Replicas = tt.actual
			_, err = env.Clientset.AppsV1().Deployments(env.Namespace).UpdateStatus(
				context.Background(),
				created,
				metav1.UpdateOptions{},
			)
			require.NoError(t, err)

			// Create validation spec - field paths are relative to status
			spec := validation.StatusSpec{
				Target: validation.Target{
					Kind: "Deployment",
					Name: "test-deployment-" + tt.name,
				},
				Checks: []validation.StatusCheck{
					{
						Field:    "replicas",
						Operator: tt.operator,
						Value:    tt.value,
					},
				},
			}

			// Execute validation
			executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result := executor.Execute(ctx, validation.Validation{
				Key:  "test-operator-" + tt.name,
				Type: validation.TypeStatus,
				Spec: spec,
			})

			// Assert results
			assert.Equal(t, tt.want, result.Passed, "Operator %s with actual=%d value=%d should be %v", tt.operator, tt.actual, tt.value, tt.want)
		})
	}
}

func TestStatusValidation_LabelSelector(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create multiple deployments, but only one matches our labels
	for i := 1; i <= 3; i++ {
		replicas := int32(i)
		labels := map[string]string{
			"app": "other-app",
		}
		if i == 2 {
			labels = map[string]string{
				"app":  "target-app",
				"tier": "backend",
			}
		}

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("deployment-%d", i),
				Namespace: env.Namespace,
				Labels:    labels,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: labels,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
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

		// Update status
		created.Status.Replicas = replicas
		_, err = env.Clientset.AppsV1().Deployments(env.Namespace).UpdateStatus(
			context.Background(),
			created,
			metav1.UpdateOptions{},
		)
		require.NoError(t, err)
	}

	// Validate using label selector (should match deployment-2 with replicas=2)
	spec := validation.StatusSpec{
		Target: validation.Target{
			Kind: "Deployment",
			LabelSelector: map[string]string{
				"app":  "target-app",
				"tier": "backend",
			},
		},
		Checks: []validation.StatusCheck{
			{
				Field:    "replicas",
				Operator: "==",
				Value:    int64(2),
			},
		},
	}

	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-label-selector",
		Type: validation.TypeStatus,
		Spec: spec,
	})

	// Should pass - found deployment-2 with 2 replicas
	assert.True(t, result.Passed, "Expected validation to pass")
}

func TestStatusValidation_NoMatchingResources(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Don't create any deployments

	// Try to validate a non-existent deployment
	spec := validation.StatusSpec{
		Target: validation.Target{
			Kind: "Deployment",
			Name: "nonexistent-deployment",
		},
		Checks: []validation.StatusCheck{
			{
				Field:    "replicas",
				Operator: "==",
				Value:    int64(3),
			},
		},
	}

	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-no-resources",
		Type: validation.TypeStatus,
		Spec: spec,
	})

	// Should fail - no matching resources
	assert.False(t, result.Passed, "Expected validation to fail")
	// The message will contain an error about the resource not being found
}

func TestStatusValidation_InvalidField(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a deployment
	replicas := int32(3)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: env.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test-app",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
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
			},
		},
	}

	created, err := env.Clientset.AppsV1().Deployments(env.Namespace).Create(
		context.Background(),
		deployment,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	created.Status.Replicas = 3
	_, err = env.Clientset.AppsV1().Deployments(env.Namespace).UpdateStatus(
		context.Background(),
		created,
		metav1.UpdateOptions{},
	)
	require.NoError(t, err)

	// Try to validate a field that doesn't exist
	spec := validation.StatusSpec{
		Target: validation.Target{
			Kind: "Deployment",
			Name: "test-deployment",
		},
		Checks: []validation.StatusCheck{
			{
				Field:    "nonExistentField",
				Operator: "==",
				Value:    int64(1),
			},
		},
	}

	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-invalid-field",
		Type: validation.TypeStatus,
		Spec: spec,
	})

	// Should fail
	assert.False(t, result.Passed, "Expected validation to fail for invalid field")
	assert.Contains(t, result.Message, "nonExistentField")
	assert.Contains(t, result.Message, "not found")
}

func TestStatusValidation_ArrayAccess(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a deployment with conditions
	replicas := int32(3)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: env.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test-app",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
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
			},
		},
	}

	created, err := env.Clientset.AppsV1().Deployments(env.Namespace).Create(
		context.Background(),
		deployment,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	// Update deployment status with conditions
	created.Status.Replicas = 3
	created.Status.ReadyReplicas = 3
	created.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:   appsv1.DeploymentAvailable,
			Status: corev1.ConditionTrue,
		},
		{
			Type:   appsv1.DeploymentProgressing,
			Status: corev1.ConditionTrue,
		},
	}
	_, err = env.Clientset.AppsV1().Deployments(env.Namespace).UpdateStatus(
		context.Background(),
		created,
		metav1.UpdateOptions{},
	)
	require.NoError(t, err)

	// Test array filter access: conditions[type=Available].status
	spec := validation.StatusSpec{
		Target: validation.Target{
			Kind: "Deployment",
			Name: "test-deployment",
		},
		Checks: []validation.StatusCheck{
			{
				Field:    "conditions[type=Available].status",
				Operator: "==",
				Value:    "True",
			},
		},
	}

	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-array-access",
		Type: validation.TypeStatus,
		Spec: spec,
	})

	assert.True(t, result.Passed, "Expected validation to pass for array filter access")
}
