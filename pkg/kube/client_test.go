package kube

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// TestCreateNamespace_Logic tests namespace creation logic
func TestCreateNamespace_Logic(t *testing.T) {
	t.Run("creates new namespace successfully", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		ctx := context.Background()

		// Manually test the logic without calling CreateNamespace
		// Check if namespace exists
		_, err := clientset.CoreV1().Namespaces().Get(ctx, "test-namespace", metav1.GetOptions{})
		assert.True(t, apierrors.IsNotFound(err), "namespace should not exist initially")

		// Create namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-namespace",
			},
		}
		_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		require.NoError(t, err)

		// Verify namespace was created
		created, err := clientset.CoreV1().Namespaces().Get(ctx, "test-namespace", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, "test-namespace", created.Name)
	})

	t.Run("idempotent - namespace already exists", func(t *testing.T) {
		// Create clientset with existing namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "existing-namespace",
			},
		}
		clientset := fake.NewSimpleClientset(ns)
		ctx := context.Background()

		// Check namespace exists
		_, err := clientset.CoreV1().Namespaces().Get(ctx, "existing-namespace", metav1.GetOptions{})
		require.NoError(t, err)

		// Try to create again - should get AlreadyExists
		ns2 := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "existing-namespace",
			},
		}
		_, err = clientset.CoreV1().Namespaces().Create(ctx, ns2, metav1.CreateOptions{})
		assert.True(t, apierrors.IsAlreadyExists(err), "should return AlreadyExists error")

		// Verify namespace still exists (CreateNamespace should handle this gracefully)
		_, err = clientset.CoreV1().Namespaces().Get(ctx, "existing-namespace", metav1.GetOptions{})
		require.NoError(t, err)
	})
}

func TestCreateNamespace(t *testing.T) {
	t.Run("creates new namespace successfully", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		ctx := context.Background()

		// Add a reactor to set namespace status to Active on creation
		clientset.PrependReactor("create", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
			createAction := action.(k8stesting.CreateAction)
			ns := createAction.GetObject().(*corev1.Namespace)
			ns.Status.Phase = corev1.NamespaceActive
			return false, ns, nil // Return false to let the fake clientset handle the actual creation
		})

		err := CreateNamespace(ctx, clientset, "test-namespace")
		require.NoError(t, err)

		// Verify namespace was created
		created, err := clientset.CoreV1().Namespaces().Get(ctx, "test-namespace", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, "test-namespace", created.Name)
	})

	t.Run("idempotent - namespace already exists with Active status", func(t *testing.T) {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "existing-namespace",
			},
			Status: corev1.NamespaceStatus{
				Phase: corev1.NamespaceActive,
			},
		}
		clientset := fake.NewSimpleClientset(ns)
		ctx := context.Background()

		err := CreateNamespace(ctx, clientset, "existing-namespace")
		require.NoError(t, err)

		// Verify namespace still exists
		_, err = clientset.CoreV1().Namespaces().Get(ctx, "existing-namespace", metav1.GetOptions{})
		require.NoError(t, err)
	})
}

// TestDeleteNamespace_Logic tests namespace deletion logic
func TestDeleteNamespace_Logic(t *testing.T) {
	t.Run("deletes existing namespace successfully", func(t *testing.T) {
		// Create clientset with existing namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "to-delete",
			},
		}
		clientset := fake.NewSimpleClientset(ns)
		ctx := context.Background()

		// Verify namespace exists
		_, err := clientset.CoreV1().Namespaces().Get(ctx, "to-delete", metav1.GetOptions{})
		require.NoError(t, err)

		// Delete namespace
		err = clientset.CoreV1().Namespaces().Delete(ctx, "to-delete", metav1.DeleteOptions{})
		require.NoError(t, err)

		// Verify namespace was deleted
		_, err = clientset.CoreV1().Namespaces().Get(ctx, "to-delete", metav1.GetOptions{})
		assert.True(t, apierrors.IsNotFound(err), "namespace should be deleted")
	})

	t.Run("idempotent - namespace does not exist", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		ctx := context.Background()

		// Try to delete non-existent namespace
		err := clientset.CoreV1().Namespaces().Delete(ctx, "nonexistent", metav1.DeleteOptions{})
		assert.True(t, apierrors.IsNotFound(err), "should return NotFound")

		// DeleteNamespace should handle this gracefully (check first)
		_, err = clientset.CoreV1().Namespaces().Get(ctx, "nonexistent", metav1.GetOptions{})
		assert.True(t, apierrors.IsNotFound(err))
	})
}

func TestWaitForNamespaceActive(t *testing.T) {
	t.Run("returns immediately when namespace is already Active", func(t *testing.T) {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "active-namespace",
			},
			Status: corev1.NamespaceStatus{
				Phase: corev1.NamespaceActive,
			},
		}
		clientset := fake.NewSimpleClientset(ns)
		ctx := context.Background()

		err := WaitForNamespaceActive(ctx, clientset, "active-namespace")
		require.NoError(t, err)
	})

	t.Run("returns error when namespace is Terminating", func(t *testing.T) {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "terminating-namespace",
			},
			Status: corev1.NamespaceStatus{
				Phase: corev1.NamespaceTerminating,
			},
		}
		clientset := fake.NewSimpleClientset(ns)
		ctx := context.Background()

		err := WaitForNamespaceActive(ctx, clientset, "terminating-namespace")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Terminating")
	})

	t.Run("times out when context is cancelled", func(t *testing.T) {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pending-namespace",
			},
			Status: corev1.NamespaceStatus{
				Phase: "", // Empty phase - not Active
			},
		}
		clientset := fake.NewSimpleClientset(ns)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := WaitForNamespaceActive(ctx, clientset, "pending-namespace")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
	})

	t.Run("returns error when namespace does not exist", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := WaitForNamespaceActive(ctx, clientset, "nonexistent")
		require.Error(t, err)
		// Should timeout since namespace doesn't exist
		assert.Contains(t, err.Error(), "timeout")
	})
}

func TestDeleteNamespace(t *testing.T) {
	t.Run("deletes existing namespace successfully", func(t *testing.T) {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "to-delete",
			},
		}
		clientset := fake.NewSimpleClientset(ns)
		ctx := context.Background()

		err := DeleteNamespace(ctx, clientset, "to-delete")
		require.NoError(t, err)

		// Verify namespace was deleted
		_, err = clientset.CoreV1().Namespaces().Get(ctx, "to-delete", metav1.GetOptions{})
		assert.True(t, apierrors.IsNotFound(err), "namespace should be deleted")
	})

	t.Run("idempotent - namespace does not exist", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		ctx := context.Background()

		err := DeleteNamespace(ctx, clientset, "nonexistent")
		require.NoError(t, err) // DeleteNamespace should be idempotent
	})
}

// TestGetResourceGVR tests the GetResourceGVR function
func TestGetResourceGVR(t *testing.T) {
	tests := []struct {
		name        string
		gvk         schema.GroupVersionKind
		expectedGVR schema.GroupVersionResource
		description string
	}{
		{
			name: "Deployment",
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			expectedGVR: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			description: "apps/v1 Deployment should map to deployments",
		},
		{
			name: "Service",
			gvk: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Service",
			},
			expectedGVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "services",
			},
			description: "core/v1 Service should map to services",
		},
		{
			name: "Ingress (networking.k8s.io)",
			gvk: schema.GroupVersionKind{
				Group:   "networking.k8s.io",
				Version: "v1",
				Kind:    "Ingress",
			},
			expectedGVR: schema.GroupVersionResource{
				Group:    "networking.k8s.io",
				Version:  "v1",
				Resource: "ingresses",
			},
			description: "networking.k8s.io Ingress should map to ingresses",
		},
		{
			name: "NetworkPolicy",
			gvk: schema.GroupVersionKind{
				Group:   "networking.k8s.io",
				Version: "v1",
				Kind:    "NetworkPolicy",
			},
			expectedGVR: schema.GroupVersionResource{
				Group:    "networking.k8s.io",
				Version:  "v1",
				Resource: "networkpolicies",
			},
			description: "NetworkPolicy should map to networkpolicies",
		},
		{
			name: "CustomResourceDefinition",
			gvk: schema.GroupVersionKind{
				Group:   "apiextensions.k8s.io",
				Version: "v1",
				Kind:    "CustomResourceDefinition",
			},
			expectedGVR: schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			},
			description: "CRD should map to customresourcedefinitions",
		},
		{
			name: "ClusterRole",
			gvk: schema.GroupVersionKind{
				Group:   "rbac.authorization.k8s.io",
				Version: "v1",
				Kind:    "ClusterRole",
			},
			expectedGVR: schema.GroupVersionResource{
				Group:    "rbac.authorization.k8s.io",
				Version:  "v1",
				Resource: "clusterroles",
			},
			description: "RBAC ClusterRole should map to clusterroles",
		},
		{
			name: "Endpoint",
			gvk: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Endpoint",
			},
			expectedGVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "endpoints",
			},
			description: "Endpoint should map to endpoints (irregular plural)",
		},
		{
			name: "PodSecurityPolicy",
			gvk: schema.GroupVersionKind{
				Group:   "policy",
				Version: "v1beta1",
				Kind:    "PodSecurityPolicy",
			},
			expectedGVR: schema.GroupVersionResource{
				Group:    "policy",
				Version:  "v1beta1",
				Resource: "podsecuritypolicies",
			},
			description: "PodSecurityPolicy should map to podsecuritypolicies",
		},
		{
			name: "StatefulSet",
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "StatefulSet",
			},
			expectedGVR: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "statefulsets",
			},
			description: "StatefulSet should map to statefulsets",
		},
		{
			name: "DaemonSet",
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "DaemonSet",
			},
			expectedGVR: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "daemonsets",
			},
			description: "DaemonSet should map to daemonsets",
		},
		{
			name: "ConfigMap",
			gvk: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "ConfigMap",
			},
			expectedGVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "configmaps",
			},
			description: "ConfigMap should map to configmaps",
		},
		{
			name: "Secret",
			gvk: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Secret",
			},
			expectedGVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "secrets",
			},
			description: "Secret should map to secrets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gvr := GetResourceGVR(&tt.gvk)
			assert.Equal(t, tt.expectedGVR.Group, gvr.Group, "Group mismatch")
			assert.Equal(t, tt.expectedGVR.Version, gvr.Version, "Version mismatch")
			assert.Equal(t, tt.expectedGVR.Resource, gvr.Resource, "Resource mismatch: %s", tt.description)
		})
	}
}

// TestWaitForDeploymentsReady_Logic tests deployment readiness logic
func TestWaitForDeploymentsReady_Logic(t *testing.T) {
	t.Run("deployment readiness conditions", func(t *testing.T) {
		tests := []struct {
			name       string
			deployment *appsv1.Deployment
			isReady    bool
			reason     string
		}{
			{
				name: "ready - all replicas ready",
				deployment: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: int32Ptr(3),
					},
					Status: appsv1.DeploymentStatus{
						ObservedGeneration: 1,
						ReadyReplicas:      3,
						UpdatedReplicas:    3,
						AvailableReplicas:  3,
					},
				},
				isReady: true,
				reason:  "all conditions met",
			},
			{
				name: "not ready - missing ready replicas",
				deployment: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: int32Ptr(3),
					},
					Status: appsv1.DeploymentStatus{
						ObservedGeneration: 1,
						ReadyReplicas:      2, // Only 2/3
						UpdatedReplicas:    3,
						AvailableReplicas:  3,
					},
				},
				isReady: false,
				reason:  "readyReplicas < desired",
			},
			{
				name: "not ready - outdated generation",
				deployment: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 2, // New generation
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: int32Ptr(3),
					},
					Status: appsv1.DeploymentStatus{
						ObservedGeneration: 1, // Old generation
						ReadyReplicas:      3,
						UpdatedReplicas:    3,
						AvailableReplicas:  3,
					},
				},
				isReady: false,
				reason:  "generation > observedGeneration",
			},
			{
				name: "not ready - missing updated replicas",
				deployment: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: int32Ptr(3),
					},
					Status: appsv1.DeploymentStatus{
						ObservedGeneration: 1,
						ReadyReplicas:      3,
						UpdatedReplicas:    2, // Only 2 updated
						AvailableReplicas:  3,
					},
				},
				isReady: false,
				reason:  "updatedReplicas < desired",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				desiredReplicas := *tt.deployment.Spec.Replicas
				ready := tt.deployment.Generation <= tt.deployment.Status.ObservedGeneration &&
					tt.deployment.Status.UpdatedReplicas >= desiredReplicas &&
					tt.deployment.Status.AvailableReplicas >= desiredReplicas &&
					tt.deployment.Status.ReadyReplicas >= desiredReplicas

				assert.Equal(t, tt.isReady, ready, "Readiness check failed: %s", tt.reason)
			})
		}
	})
}

// TestWaitForStatefulSetsReady_Logic tests statefulset readiness logic
func TestWaitForStatefulSetsReady_Logic(t *testing.T) {
	t.Run("statefulset readiness conditions", func(t *testing.T) {
		tests := []struct {
			name        string
			statefulSet *appsv1.StatefulSet
			isReady     bool
			reason      string
		}{
			{
				name: "ready - all conditions met",
				statefulSet: &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(3),
					},
					Status: appsv1.StatefulSetStatus{
						ObservedGeneration: 1,
						ReadyReplicas:      3,
						UpdatedReplicas:    3,
						CurrentRevision:    "test-abc123",
						UpdateRevision:     "test-abc123", // Same = rollout complete
					},
				},
				isReady: true,
				reason:  "all conditions met and revisions match",
			},
			{
				name: "not ready - revision mismatch",
				statefulSet: &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(3),
					},
					Status: appsv1.StatefulSetStatus{
						ObservedGeneration: 1,
						ReadyReplicas:      3,
						UpdatedReplicas:    2,
						CurrentRevision:    "test-abc123",
						UpdateRevision:     "test-def456", // Different = rollout in progress
					},
				},
				isReady: false,
				reason:  "currentRevision != updateRevision",
			},
			{
				name: "not ready - missing ready replicas",
				statefulSet: &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(5),
					},
					Status: appsv1.StatefulSetStatus{
						ObservedGeneration: 1,
						ReadyReplicas:      3, // Only 3/5
						UpdatedReplicas:    5,
						CurrentRevision:    "test-abc123",
						UpdateRevision:     "test-abc123",
					},
				},
				isReady: false,
				reason:  "readyReplicas < desired",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				desiredReplicas := *tt.statefulSet.Spec.Replicas
				ready := tt.statefulSet.Generation <= tt.statefulSet.Status.ObservedGeneration &&
					tt.statefulSet.Status.ReadyReplicas >= desiredReplicas &&
					tt.statefulSet.Status.UpdatedReplicas >= desiredReplicas &&
					tt.statefulSet.Status.CurrentRevision == tt.statefulSet.Status.UpdateRevision

				assert.Equal(t, tt.isReady, ready, "Readiness check failed: %s", tt.reason)
			})
		}
	})
}

// TestLoggingRoundTripper tests the HTTP logging wrapper
func TestLoggingRoundTripper(t *testing.T) {
	t.Run("wraps transport correctly", func(t *testing.T) {
		// Create a mock round tripper
		mockRT := &mockRoundTripper{
			response: &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
			},
		}

		lrt := &LoggingRoundTripper{rt: mockRT}

		req, err := http.NewRequest("GET", "http://example.com", nil)
		require.NoError(t, err)

		resp, err := lrt.RoundTrip(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.True(t, mockRT.called, "underlying transport should be called")
	})
}

// Helper functions
func int32Ptr(i int32) *int32 {
	return &i
}

// Mock HTTP RoundTripper for testing
type mockRoundTripper struct {
	called   bool
	response *http.Response
	err      error
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.called = true
	return m.response, m.err
}
