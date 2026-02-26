package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
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

// newTestScheme returns a scheme with core and apps types registered.
func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	return scheme
}

// TestApplyManifest_DocumentSplitting tests manifest document splitting logic
func TestApplyManifest_DocumentSplitting(t *testing.T) {
	t.Run("single document", func(t *testing.T) {
		scheme := newTestScheme()
		mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(simpleConfigMapManifest), "default", mapper, dynamicClient)
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

		scheme := newTestScheme()
		mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(manifest), "default", mapper, dynamicClient)
		require.NoError(t, err)
	})

	t.Run("empty documents are skipped", func(t *testing.T) {
		manifest := `
---
` + simpleConfigMapManifest + `
---
`

		scheme := newTestScheme()
		mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(manifest), "default", mapper, dynamicClient)
		require.NoError(t, err)
	})
}

// TestApplyManifest_NamespaceInjection tests namespace injection for namespaced resources
func TestApplyManifest_NamespaceInjection(t *testing.T) {
	t.Run("injects namespace when not specified", func(t *testing.T) {
		scheme := newTestScheme()
		mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(simpleConfigMapManifest), "custom-namespace", mapper, dynamicClient)
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

		scheme := newTestScheme()
		mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(manifest), "default", mapper, dynamicClient)
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

		scheme := newTestScheme()
		mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(manifest), "default", mapper, dynamicClient)
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
		scheme := newTestScheme()
		mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(simpleConfigMapManifest), "default", mapper, dynamicClient)
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

		scheme := newTestScheme()
		mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(initialManifest), "default", mapper, dynamicClient)
		require.NoError(t, err)

		// Now update with new data
		updatedManifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: updated-value`

		err = ApplyManifest(ctx, []byte(updatedManifest), "default", mapper, dynamicClient)
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

		scheme := newTestScheme()
		mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		// Should not error - invalid documents are logged and skipped
		err := ApplyManifest(ctx, []byte(manifest), "default", mapper, dynamicClient)
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

		scheme := newTestScheme()
		mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(manifest), "default", mapper, dynamicClient)
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

	t.Run("skips unknown kinds gracefully", func(t *testing.T) {
		manifest := `apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: test-policy`

		scheme := newTestScheme() // kyverno.io not registered
		mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		// Should not error - unknown kinds are logged and skipped
		err := ApplyManifest(ctx, []byte(manifest), "default", mapper, dynamicClient)
		assert.NoError(t, err)
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

		scheme := newTestScheme()
		mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(manifest), "default", mapper, dynamicClient)
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

// TestApplyManifest_RESTMapperGVR verifies that GVR resolution relies on the mapper,
// covering types that previously required a manual switch (apps/v1, rbac, networking).
func TestApplyManifest_RESTMapperGVR(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		gvr      schema.GroupVersionResource
		resName  string
	}{
		{
			name: "apps/v1 Deployment resolved to deployments",
			manifest: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deploy
spec:
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: test
        image: nginx`,
			gvr:     schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			resName: "test-deploy",
		},
		{
			name: "rbac ClusterRole resolved to clusterroles (cluster-scoped, no namespace)",
			manifest: `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: test-clusterrole`,
			gvr:     schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
			resName: "test-clusterrole",
		},
		{
			name: "core/v1 ServiceAccount resolved to serviceaccounts",
			manifest: `apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa`,
			gvr:     schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"},
			resName: "test-sa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme()
			mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
			dynamicClient := fake.NewSimpleDynamicClient(scheme)
			ctx := context.Background()

			err := ApplyManifest(ctx, []byte(tt.manifest), "default", mapper, dynamicClient)
			require.NoError(t, err)

			// Verify the resource landed at the correct GVR
			if tt.gvr.Group == "rbac.authorization.k8s.io" {
				// ClusterRole is cluster-scoped
				_, err = dynamicClient.Resource(tt.gvr).Get(ctx, tt.resName, metav1.GetOptions{})
			} else {
				_, err = dynamicClient.Resource(tt.gvr).Namespace("default").Get(ctx, tt.resName, metav1.GetOptions{})
			}
			require.NoError(t, err, "resource should be reachable at expected GVR %v", tt.gvr)
		})
	}
}

// TestApplyManifest_RESTMapperScope verifies that namespace injection is driven by mapper scope,
// not by a hardcoded list.
func TestApplyManifest_RESTMapperScope(t *testing.T) {
	t.Run("ClusterRole (rbac, cluster-scoped) receives no namespace", func(t *testing.T) {
		manifest := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: test-clusterrole`

		scheme := newTestScheme()
		mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(manifest), "injected-ns", mapper, dynamicClient)
		require.NoError(t, err)

		gvr := schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}
		obj, err := dynamicClient.Resource(gvr).Get(ctx, "test-clusterrole", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Empty(t, obj.GetNamespace(), "cluster-scoped resource must not receive a namespace")
	})

	t.Run("ServiceAccount (namespaced) receives injected namespace", func(t *testing.T) {
		manifest := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa`

		scheme := newTestScheme()
		mapper := testrestmapper.TestOnlyStaticRESTMapper(scheme)
		dynamicClient := fake.NewSimpleDynamicClient(scheme)
		ctx := context.Background()

		err := ApplyManifest(ctx, []byte(manifest), "injected-ns", mapper, dynamicClient)
		require.NoError(t, err)

		gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"}
		obj, err := dynamicClient.Resource(gvr).Namespace("injected-ns").Get(ctx, "test-sa", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, "injected-ns", obj.GetNamespace())
	})
}
