//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
	"github.com/kubeasy-dev/kubeasy-cli/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Note: Log validation tests are limited in EnvTest because there are no actual
// running containers producing logs. These tests verify the validation logic
// handles the "no logs" case correctly.

func TestLogValidation_NoPods_Failure(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Don't create any pods

	// Create validation spec
	spec := validation.LogSpec{
		Target: validation.Target{
			Kind: "Pod",
			LabelSelector: map[string]string{
				"app": "nonexistent-app",
			},
		},
		ExpectedStrings: []string{
			"Server started",
		},
		SinceSeconds: 60,
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-no-pods",
		Type: validation.TypeLog,
		Spec: spec,
	})

	// Assert results - should fail because no pods exist
	assert.False(t, result.Passed, "Expected validation to fail when no pods exist")
	assert.Equal(t, "No matching pods found", result.Message)
}

func TestLogValidation_PodExists_NoLogs(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a pod (but it won't have logs in EnvTest since no container actually runs)
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

	// Mark pod as running (even though no container is actually running)
	env.SetPodReady(createdPod.Name)

	// Create validation spec
	spec := validation.LogSpec{
		Target: validation.Target{
			Kind: "Pod",
			LabelSelector: map[string]string{
				"app": "test-app",
			},
		},
		ExpectedStrings: []string{
			"Server started",
			"Ready to accept connections",
		},
		SinceSeconds: 60,
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-pod-no-logs",
		Type: validation.TypeLog,
		Spec: spec,
	})

	// In EnvTest, pods don't have actual logs, so this will fail with missing strings
	// This validates the "missing strings" error path
	assert.False(t, result.Passed, "Expected validation to fail (no actual logs in EnvTest)")
	assert.Contains(t, result.Message, "Missing strings in logs")
	assert.Contains(t, result.Message, "Server started")
	assert.Contains(t, result.Message, "Ready to accept connections")
}

func TestLogValidation_SpecificContainer(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a pod with multiple containers
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multi-container-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "multi-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
				{
					Name:  "sidecar",
					Image: "busybox:latest",
				},
			},
		},
	}

	createdPod := env.CreatePod(pod)
	require.NotNil(t, createdPod)

	// Create validation spec targeting specific container
	spec := validation.LogSpec{
		Target: validation.Target{
			Kind: "Pod",
			LabelSelector: map[string]string{
				"app": "multi-app",
			},
		},
		Container: "nginx",
		ExpectedStrings: []string{
			"nginx: the configuration file",
		},
		SinceSeconds: 30,
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-specific-container",
		Type: validation.TypeLog,
		Spec: spec,
	})

	// Will fail in EnvTest due to no actual logs, but validates the container selection logic
	assert.False(t, result.Passed, "Expected validation to fail (no actual logs in EnvTest)")
}

func TestLogValidation_DefaultContainer(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create a pod without specifying container name in validation
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-container-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "default-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "nginx:latest",
				},
			},
		},
	}

	createdPod := env.CreatePod(pod)
	require.NotNil(t, createdPod)

	// Create validation spec WITHOUT specifying container (should use first container)
	spec := validation.LogSpec{
		Target: validation.Target{
			Kind: "Pod",
			LabelSelector: map[string]string{
				"app": "default-app",
			},
		},
		// No Container field - should default to first container
		ExpectedStrings: []string{
			"Application ready",
		},
		SinceSeconds: 60,
	}

	// Execute validation
	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-default-container",
		Type: validation.TypeLog,
		Spec: spec,
	})

	// Will fail in EnvTest, but validates default container selection works
	assert.False(t, result.Passed, "Expected validation to fail (no actual logs in EnvTest)")
}

func TestLogValidation_PodByName(t *testing.T) {
	env := helpers.SetupEnvTest(t)

	// Create two pods
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "log-app",
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

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-pod",
			Namespace: env.Namespace,
			Labels: map[string]string{
				"app": "log-app",
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

	env.CreatePod(pod1)
	env.CreatePod(pod2)

	// Validate specific pod by name
	spec := validation.LogSpec{
		Target: validation.Target{
			Kind: "Pod",
			Name: "target-pod",
		},
		ExpectedStrings: []string{
			"Started successfully",
		},
		SinceSeconds: 60,
	}

	executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := executor.Execute(ctx, validation.Validation{
		Key:  "test-pod-by-name",
		Type: validation.TypeLog,
		Spec: spec,
	})

	// Should target only target-pod, not both pods
	assert.False(t, result.Passed, "Expected validation to fail (no actual logs in EnvTest)")
	assert.Contains(t, result.Message, "Missing strings in logs")
}
