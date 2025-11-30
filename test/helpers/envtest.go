package helpers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// TestEnvironment wraps envtest.Environment with useful clients and helpers
type TestEnvironment struct {
	Env           *envtest.Environment
	Config        *rest.Config
	Clientset     *kubernetes.Clientset
	DynamicClient dynamic.Interface
	Namespace     string
	t             *testing.T
}

// SetupEnvTest creates a new test environment with a running Kubernetes API server
func SetupEnvTest(t *testing.T) *TestEnvironment {
	t.Helper()

	// Create envtest environment
	// BinaryAssetsDirectory is not needed if KUBEBUILDER_ASSETS env var is set
	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{},
		ErrorIfCRDPathMissing: false,
	}

	// Start the test environment
	cfg, err := env.Start()
	require.NoError(t, err, "Failed to start test environment")
	require.NotNil(t, cfg, "Expected config to be non-nil")

	// Create clientset
	clientset, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err, "Failed to create clientset")

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(cfg)
	require.NoError(t, err, "Failed to create dynamic client")

	// Create a test namespace
	// Convert test name to lowercase and replace underscores with hyphens for K8s compliance
	// Truncate to 63 chars (K8s limit) and trim trailing hyphens
	namespace := sanitizeNamespaceName("test-" + strings.ToLower(strings.ReplaceAll(t.Name(), "_", "-")))
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	_, err = clientset.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	require.NoError(t, err, "Failed to create test namespace")

	testEnv := &TestEnvironment{
		Env:           env,
		Config:        cfg,
		Clientset:     clientset,
		DynamicClient: dynamicClient,
		Namespace:     namespace,
		t:             t,
	}

	// Register cleanup
	t.Cleanup(func() {
		testEnv.Cleanup()
	})

	return testEnv
}

// Cleanup stops the test environment and cleans up resources
func (e *TestEnvironment) Cleanup() {
	if e.Env != nil {
		// Delete test namespace
		if e.Clientset != nil && e.Namespace != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := e.Clientset.CoreV1().Namespaces().Delete(ctx, e.Namespace, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				e.t.Logf("Warning: failed to delete namespace %s: %v", e.Namespace, err)
			}
		}

		// Stop environment
		if err := e.Env.Stop(); err != nil {
			e.t.Logf("Failed to stop test environment: %v", err)
		}
	}
}

// CreatePod creates a pod in the test namespace
func (e *TestEnvironment) CreatePod(pod *corev1.Pod) *corev1.Pod {
	e.t.Helper()

	if pod.Namespace == "" {
		pod.Namespace = e.Namespace
	}

	created, err := e.Clientset.CoreV1().Pods(e.Namespace).Create(
		context.Background(),
		pod,
		metav1.CreateOptions{},
	)
	require.NoError(e.t, err, "Failed to create pod")

	return created
}

// UpdatePodStatus updates a pod's status
func (e *TestEnvironment) UpdatePodStatus(pod *corev1.Pod) *corev1.Pod {
	e.t.Helper()

	updated, err := e.Clientset.CoreV1().Pods(e.Namespace).UpdateStatus(
		context.Background(),
		pod,
		metav1.UpdateOptions{},
	)
	require.NoError(e.t, err, "Failed to update pod status")

	return updated
}

// SetPodReady marks a pod as Ready
func (e *TestEnvironment) SetPodReady(podName string) *corev1.Pod {
	e.t.Helper()

	pod, err := e.Clientset.CoreV1().Pods(e.Namespace).Get(
		context.Background(),
		podName,
		metav1.GetOptions{},
	)
	require.NoError(e.t, err, "Failed to get pod")

	pod.Status.Conditions = []corev1.PodCondition{
		{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		},
	}
	pod.Status.Phase = corev1.PodRunning

	return e.UpdatePodStatus(pod)
}

// CreateEvent creates a Kubernetes event with default values
func (e *TestEnvironment) CreateEvent(event *corev1.Event) *corev1.Event {
	return e.CreateEventWithDefaults(event, "kubeasy-test", "kubeasy-test-instance")
}

// CreateEventWithDefaults creates a Kubernetes event with configurable default values
func (e *TestEnvironment) CreateEventWithDefaults(event *corev1.Event, controller, instance string) *corev1.Event {
	e.t.Helper()

	if event.Namespace == "" {
		event.Namespace = e.Namespace
	}

	// Set required fields for Kubernetes 1.30+
	if event.ReportingController == "" {
		event.ReportingController = controller
	}
	if event.ReportingInstance == "" {
		event.ReportingInstance = instance
	}
	if event.Action == "" {
		event.Action = "TestEvent"
	}

	created, err := e.Clientset.CoreV1().Events(e.Namespace).Create(
		context.Background(),
		event,
		metav1.CreateOptions{},
	)
	require.NoError(e.t, err, "Failed to create event")

	return created
}

// WaitForPod waits for a pod to exist
func (e *TestEnvironment) WaitForPod(podName string, timeout time.Duration) *corev1.Pod {
	e.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Use 50ms polling interval for faster test execution
	// This is safe for envtest (local API server) without causing flakiness
	// For real clusters, consider using watch API or longer intervals
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.t.Fatalf("Timeout waiting for pod %s", podName)
			return nil
		case <-ticker.C:
			pod, err := e.Clientset.CoreV1().Pods(e.Namespace).Get(
				context.Background(),
				podName,
				metav1.GetOptions{},
			)
			if err == nil {
				return pod
			}
		}
	}
}

// sanitizeNamespaceName ensures namespace name is valid for Kubernetes
// Limits to 63 chars and removes trailing hyphens
func sanitizeNamespaceName(name string) string {
	// Kubernetes resource names have a 63 character limit
	const maxLen = 63
	if len(name) > maxLen {
		name = name[:maxLen]
	}
	// Remove trailing hyphens (invalid in K8s names)
	name = strings.TrimRight(name, "-")
	return name
}
