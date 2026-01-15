package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
)

const (
	// Test manifests used across multiple tests
	simpleConfigMapManifest = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value`
)

// TestIsNamespaced tests the IsNamespaced function
func TestIsNamespaced(t *testing.T) {
	tests := []struct {
		name              string
		kind              string
		expectsNamespaced bool
	}{
		// Cluster-scoped resources
		{name: "Namespace", kind: "Namespace", expectsNamespaced: false},
		{name: "Node", kind: "Node", expectsNamespaced: false},
		{name: "PersistentVolume", kind: "PersistentVolume", expectsNamespaced: false},
		{name: "ClusterRole", kind: "ClusterRole", expectsNamespaced: false},
		{name: "ClusterRoleBinding", kind: "ClusterRoleBinding", expectsNamespaced: false},
		{name: "CustomResourceDefinition", kind: "CustomResourceDefinition", expectsNamespaced: false},
		{name: "StorageClass", kind: "StorageClass", expectsNamespaced: false},
		{name: "PodSecurityPolicy", kind: "PodSecurityPolicy", expectsNamespaced: false},
		{name: "MutatingWebhookConfiguration", kind: "MutatingWebhookConfiguration", expectsNamespaced: false},
		{name: "ValidatingWebhookConfiguration", kind: "ValidatingWebhookConfiguration", expectsNamespaced: false},
		{name: "VolumeAttachment", kind: "VolumeAttachment", expectsNamespaced: false},
		{name: "RuntimeClass", kind: "RuntimeClass", expectsNamespaced: false},
		{name: "PriorityClass", kind: "PriorityClass", expectsNamespaced: false},
		{name: "CSIDriver", kind: "CSIDriver", expectsNamespaced: false},
		{name: "CSINode", kind: "CSINode", expectsNamespaced: false},
		{name: "APIService", kind: "APIService", expectsNamespaced: false},
		{name: "CertificateSigningRequest", kind: "CertificateSigningRequest", expectsNamespaced: false},

		// Namespaced resources
		{name: "Pod", kind: "Pod", expectsNamespaced: true},
		{name: "Deployment", kind: "Deployment", expectsNamespaced: true},
		{name: "Service", kind: "Service", expectsNamespaced: true},
		{name: "ConfigMap", kind: "ConfigMap", expectsNamespaced: true},
		{name: "Secret", kind: "Secret", expectsNamespaced: true},
		{name: "ServiceAccount", kind: "ServiceAccount", expectsNamespaced: true},
		{name: "Role", kind: "Role", expectsNamespaced: true},
		{name: "RoleBinding", kind: "RoleBinding", expectsNamespaced: true},
		{name: "Ingress", kind: "Ingress", expectsNamespaced: true},
		{name: "NetworkPolicy", kind: "NetworkPolicy", expectsNamespaced: true},
		{name: "StatefulSet", kind: "StatefulSet", expectsNamespaced: true},
		{name: "DaemonSet", kind: "DaemonSet", expectsNamespaced: true},

		// Case insensitive
		{name: "namespace (lowercase)", kind: "namespace", expectsNamespaced: false},
		{name: "NAMESPACE (uppercase)", kind: "NAMESPACE", expectsNamespaced: false},
		{name: "pod (lowercase)", kind: "pod", expectsNamespaced: true},
		{name: "POD (uppercase)", kind: "POD", expectsNamespaced: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNamespaced(tt.kind)
			assert.Equal(t, tt.expectsNamespaced, result,
				"IsNamespaced(%s) should return %v", tt.kind, tt.expectsNamespaced)
		})
	}
}

// TestApplyManifest_DocumentSplitting tests manifest document splitting logic
func TestApplyManifest_DocumentSplitting(t *testing.T) {
	t.Run("single document", func(t *testing.T) {
		scheme := runtime.NewScheme()
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(simpleConfigMapManifest), "default", nil, dynamicClient)
		require.NoError(t, err)
	})

	t.Run("multiple documents separated by ---", func(t *testing.T) {
		manifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: config1
data:
  key: value1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config2
data:
  key: value2`

		scheme := runtime.NewScheme()
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(manifest), "default", nil, dynamicClient)
		require.NoError(t, err)
	})

	t.Run("empty documents are skipped", func(t *testing.T) {
		manifest := `
---
` + simpleConfigMapManifest + `
---
`

		scheme := runtime.NewScheme()
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(manifest), "default", nil, dynamicClient)
		require.NoError(t, err)
	})
}

// TestApplyManifest_NamespaceInjection tests namespace injection for namespaced resources
func TestApplyManifest_NamespaceInjection(t *testing.T) {
	t.Run("injects namespace when not specified", func(t *testing.T) {
		scheme := runtime.NewScheme()
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(simpleConfigMapManifest), "custom-namespace", nil, dynamicClient)
		require.NoError(t, err)

		// Verify the ConfigMap was created in the correct namespace
		gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
		obj, err := dynamicClient.Resource(gvr).Namespace("custom-namespace").Get(ctx, "test-config", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, "custom-namespace", obj.GetNamespace())
	})

	t.Run("preserves existing namespace", func(t *testing.T) {
		manifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: original-namespace
data:
  key: value`

		scheme := runtime.NewScheme()
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(manifest), "default", nil, dynamicClient)
		require.NoError(t, err)

		// Verify the ConfigMap was created in the original namespace, not the default
		gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
		obj, err := dynamicClient.Resource(gvr).Namespace("original-namespace").Get(ctx, "test-config", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, "original-namespace", obj.GetNamespace())
	})

	t.Run("does not inject namespace for cluster-scoped resources", func(t *testing.T) {
		manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: new-namespace`

		scheme := runtime.NewScheme()
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(manifest), "default", nil, dynamicClient)
		require.NoError(t, err)

		// Verify the Namespace was created without a namespace field
		gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
		obj, err := dynamicClient.Resource(gvr).Get(ctx, "new-namespace", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Empty(t, obj.GetNamespace(), "Cluster-scoped resources should not have a namespace")
	})
}

// TestApplyManifest_ResourceCreation tests resource creation logic
func TestApplyManifest_ResourceCreation(t *testing.T) {
	t.Run("creates new resource successfully", func(t *testing.T) {
		scheme := runtime.NewScheme()
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(simpleConfigMapManifest), "default", nil, dynamicClient)
		require.NoError(t, err)

		// Verify the ConfigMap was created
		gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
		obj, err := dynamicClient.Resource(gvr).Namespace("default").Get(ctx, "test-config", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, "test-config", obj.GetName())
		assert.Equal(t, "ConfigMap", obj.GetKind())
	})

	t.Run("updates existing resource", func(t *testing.T) {
		// First create a resource
		initialManifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: original-value`

		scheme := runtime.NewScheme()
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(initialManifest), "default", nil, dynamicClient)
		require.NoError(t, err)

		// Now update with new data
		updatedManifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: updated-value`

		err = ApplyManifest(ctx, []byte(updatedManifest), "default", nil, dynamicClient)
		require.NoError(t, err)

		// Verify the ConfigMap was updated
		gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
		obj, err := dynamicClient.Resource(gvr).Namespace("default").Get(ctx, "test-config", metav1.GetOptions{})
		require.NoError(t, err)

		data, found, err := unstructured.NestedString(obj.Object, "data", "key")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "updated-value", data)
	})
}

// TestApplyManifest_ErrorHandling tests error handling scenarios
func TestApplyManifest_ErrorHandling(t *testing.T) {
	t.Run("handles invalid YAML gracefully", func(t *testing.T) {
		manifest := `invalid: yaml: content:
  this is not: proper: yaml:`

		scheme := runtime.NewScheme()
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		// Should not error - invalid documents are logged and skipped
		err := ApplyManifest(ctx, []byte(manifest), "default", nil, dynamicClient)
		assert.NoError(t, err, "ApplyManifest should continue processing even with invalid YAML")
	})

	t.Run("handles mixed valid and invalid documents", func(t *testing.T) {
		manifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: valid-config
data:
  key: value
---
invalid: yaml: content:
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: another-valid-config
data:
  key: value2`

		scheme := runtime.NewScheme()
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(manifest), "default", nil, dynamicClient)
		require.NoError(t, err)

		// Verify the valid ConfigMaps were created
		gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
		obj1, err := dynamicClient.Resource(gvr).Namespace("default").Get(ctx, "valid-config", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, "valid-config", obj1.GetName())

		obj2, err := dynamicClient.Resource(gvr).Namespace("default").Get(ctx, "another-valid-config", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, "another-valid-config", obj2.GetName())
	})
}

// TestApplyManifest_MultipleResourceTypes tests applying different resource types
func TestApplyManifest_MultipleResourceTypes(t *testing.T) {
	t.Run("applies multiple resource types", func(t *testing.T) {
		manifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
---
apiVersion: v1
kind: Secret
metadata:
  name: test-secret
type: Opaque
data:
  password: c2VjcmV0`

		scheme := runtime.NewScheme()
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(manifest), "default", nil, dynamicClient)
		require.NoError(t, err)

		// Verify ConfigMap was created
		cmGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
		cm, err := dynamicClient.Resource(cmGVR).Namespace("default").Get(ctx, "test-config", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, "ConfigMap", cm.GetKind())

		// Verify Secret was created
		secretGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
		secret, err := dynamicClient.Resource(secretGVR).Namespace("default").Get(ctx, "test-secret", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, "Secret", secret.GetKind())
	})
}

// TestApplyManifest_NilClientset tests that nil clientset is handled
func TestApplyManifest_NilClientset(t *testing.T) {
	t.Run("works with nil kubernetes clientset", func(t *testing.T) {
		scheme := runtime.NewScheme()
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		// Pass nil for kubernetes.Clientset (not used in current implementation)
		var kubeClient *kubernetes.Clientset = nil
		err := ApplyManifest(ctx, []byte(simpleConfigMapManifest), "default", kubeClient, dynamicClient)
		require.NoError(t, err)
	})
}
