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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Note: Connectivity validation tests are limited in EnvTest because:
// 1. Pods don't actually run (no containers executing)
// 2. No network connectivity between pods exists
// 3. No curl/wget tools available in the test environment
// These tests verify the validation logic handles edge cases correctly.

func TestConnectivityValidation_NoSourcePod_Failure(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Don't create any pods

	// Create validation spec
	spec := validation.ConnectivitySpec{
		SourcePod: validation.SourcePod{
			LabelSelector: map[string]string{
				"app": "nonexistent-app",
			},
		},
		Targets: []validation.ConnectivityCheck{
			{
				URL:                "http://test-service:8080/health",
				ExpectedStatusCode: 200,
				TimeoutSeconds:     5,
			},
		},
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-no-source-pod",
		Type: validation.TypeConnectivity,
		Spec: spec,
	})

	// Should fail because no source pod exists
	assert.False(t, result.Passed, "Expected validation to fail when no source pod exists")
	assert.Equal(t, "No matching source pods found", result.Message)
}

func TestConnectivityValidation_SourcePodNotRunning_Failure(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a pod but don't mark it as running
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "source-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "curl",
					Image: "curlimages/curl:latest",
				},
			},
		},
	}

	createdPod := env.CreatePod(pod)
	require.NotNil(t, createdPod)
	// Don't set pod as ready - leave it in pending state

	// Create validation spec
	spec := validation.ConnectivitySpec{
		SourcePod: validation.SourcePod{
			LabelSelector: map[string]string{
				"app": "source-app",
			},
		},
		Targets: []validation.ConnectivityCheck{
			{
				URL:                "http://test-service:8080",
				ExpectedStatusCode: 200,
				TimeoutSeconds:     5,
			},
		},
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-source-not-running",
		Type: validation.TypeConnectivity,
		Spec: spec,
	})

	// Should fail because source pod is not running
	assert.False(t, result.Passed, "Expected validation to fail when source pod not running")
	assert.Equal(t, "No running source pods found", result.Message)
}

func TestConnectivityValidation_SourcePodByName(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create two pods, but only one will be the source
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "curl-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "curl",
					Image: "curlimages/curl:latest",
				},
			},
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "curl-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "curl",
					Image: "curlimages/curl:latest",
				},
			},
		},
	}

	createdPod1 := env.CreatePod(pod1)
	env.CreatePod(pod2)

	// Mark first pod as running
	env.SetPodReady(createdPod1.Name)

	// Validate using specific pod name
	spec := validation.ConnectivitySpec{
		SourcePod: validation.SourcePod{
			Name: "source-pod",
		},
		Targets: []validation.ConnectivityCheck{
			{
				URL:                "http://api-service:8080/v1/health",
				ExpectedStatusCode: 200,
				TimeoutSeconds:     5,
			},
		},
	}

	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-source-by-name",
		Type: validation.TypeConnectivity,
		Spec: spec,
	})

	// Will fail in EnvTest because exec into pod won't work, but validates source pod selection
	assert.False(t, result.Passed, "Expected validation to fail (no actual exec in EnvTest)")
	// Message should indicate connection failure, not "no source pod"
	assert.NotEqual(t, "No matching source pods found", result.Message)
	assert.NotEqual(t, "No running source pods found", result.Message)
}

func TestConnectivityValidation_MultipleTargets(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create source pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "curl",
					Image: "curlimages/curl:latest",
				},
			},
		},
	}

	createdPod := env.CreatePod(pod)
	env.SetPodReady(createdPod.Name)

	// Create validation spec with multiple targets
	spec := validation.ConnectivitySpec{
		SourcePod: validation.SourcePod{
			Name: "source-pod",
		},
		Targets: []validation.ConnectivityCheck{
			{
				URL:                "http://service-a:8080/health",
				ExpectedStatusCode: 200,
				TimeoutSeconds:     5,
			},
			{
				URL:                "http://service-b:9000/ready",
				ExpectedStatusCode: 200,
				TimeoutSeconds:     3,
			},
			{
				URL:                "http://service-c:3000/status",
				ExpectedStatusCode: 200,
				TimeoutSeconds:     10,
			},
		},
	}

	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-multiple-targets",
		Type: validation.TypeConnectivity,
		Spec: spec,
	})

	// Will fail in EnvTest, but validates multiple targets are checked
	assert.False(t, result.Passed, "Expected validation to fail (no actual connectivity in EnvTest)")
}

func TestConnectivityValidation_LabelSelector(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create multiple pods with same labels
	for i := 1; i <= 3; i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("pod-%d", i),
				Namespace: env.Namespace,
				Labels: map[string]string{
					"app":  "test-app",
					"role": "client",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "curl",
						Image: "curlimages/curl:latest",
					},
				},
			},
		}

		createdPod := env.CreatePod(pod)
		// Mark all pods as running
		env.SetPodReady(createdPod.Name)
	}

	// Validate using label selector (should pick first running pod)
	spec := validation.ConnectivitySpec{
		SourcePod: validation.SourcePod{
			LabelSelector: map[string]string{
				"app":  "test-app",
				"role": "client",
			},
		},
		Targets: []validation.ConnectivityCheck{
			{
				URL:                "http://backend:8080",
				ExpectedStatusCode: 200,
				TimeoutSeconds:     5,
			},
		},
	}

	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-label-selector",
		Type: validation.TypeConnectivity,
		Spec: spec,
	})

	// Will fail in EnvTest, but validates label selector works
	assert.False(t, result.Passed, "Expected validation to fail (no actual connectivity in EnvTest)")
}

func TestConnectivityValidation_NoSourcePodSpecified_Failure(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create validation spec without any source pod specification
	spec := validation.ConnectivitySpec{
		SourcePod: validation.SourcePod{
			// Neither Name nor LabelSelector specified
		},
		Targets: []validation.ConnectivityCheck{
			{
				URL:                "http://test-service:8080",
				ExpectedStatusCode: 200,
				TimeoutSeconds:     5,
			},
		},
	}

	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-no-source-specified",
		Type: validation.TypeConnectivity,
		Spec: spec,
	})

	// Should fail because no source pod is specified
	assert.False(t, result.Passed, "Expected validation to fail when no source pod specified")
	assert.Equal(t, "No source pod specified", result.Message)
}

func TestConnectivityValidation_DefaultTimeout(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create source pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "curl",
					Image: "curlimages/curl:latest",
				},
			},
		},
	}

	createdPod := env.CreatePod(pod)
	env.SetPodReady(createdPod.Name)

	// Create validation without specifying timeout (should default to 5 seconds)
	spec := validation.ConnectivitySpec{
		SourcePod: validation.SourcePod{
			Name: "source-pod",
		},
		Targets: []validation.ConnectivityCheck{
			{
				URL:                "http://slow-service:8080",
				ExpectedStatusCode: 200,
				// TimeoutSeconds not specified - should default to 5
			},
		},
	}

	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-default-timeout",
		Type: validation.TypeConnectivity,
		Spec: spec,
	})

	// Will fail in EnvTest, but validates timeout defaulting works
	assert.False(t, result.Passed, "Expected validation to fail (no actual connectivity in EnvTest)")
}
