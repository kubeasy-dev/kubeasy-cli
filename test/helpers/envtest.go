package helpers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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
	namespace := "test-" + strings.ToLower(strings.ReplaceAll(t.Name(), "_", "-"))
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
			_ = e.Clientset.CoreV1().Namespaces().Delete(ctx, e.Namespace, metav1.DeleteOptions{})
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

// CreateEvent creates a Kubernetes event
func (e *TestEnvironment) CreateEvent(event *corev1.Event) *corev1.Event {
	e.t.Helper()

	if event.Namespace == "" {
		event.Namespace = e.Namespace
	}

	// Set required fields for Kubernetes 1.30+
	if event.ReportingController == "" {
		event.ReportingController = "kubeasy-test"
	}
	if event.ReportingInstance == "" {
		event.ReportingInstance = "kubeasy-test-instance"
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

	ticker := time.NewTicker(100 * time.Millisecond)
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
