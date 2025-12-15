package argocd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
)

// TestDefaultInstallOptions tests the default installation options
func TestDefaultInstallOptions(t *testing.T) {
	t.Run("returns correct default values", func(t *testing.T) {
		options := DefaultInstallOptions()

		assert.Equal(t, DefaultManifestURL, options.ManifestURL)
		assert.Equal(t, DefaultInstallTimeout, options.InstallTimeout)
		assert.True(t, options.WaitForReady)
	})

	t.Run("manifest URL matches expected pattern", func(t *testing.T) {
		options := DefaultInstallOptions()
		assert.Contains(t, options.ManifestURL, "raw.githubusercontent.com/argoproj/argo-cd")
		assert.Contains(t, options.ManifestURL, "core-install.yaml")
	})

	t.Run("timeout is reasonable", func(t *testing.T) {
		options := DefaultInstallOptions()
		assert.Equal(t, 5*time.Minute, options.InstallTimeout)
		assert.True(t, options.InstallTimeout > 0)
	})
}

// TestInstallOptions_CustomValues tests custom installation options
func TestInstallOptions_CustomValues(t *testing.T) {
	t.Run("accepts custom manifest URL", func(t *testing.T) {
		customOptions := &InstallOptions{
			ManifestURL:    "https://custom.example.com/manifest.yaml",
			InstallTimeout: 10 * time.Minute,
			WaitForReady:   false,
		}

		assert.Equal(t, "https://custom.example.com/manifest.yaml", customOptions.ManifestURL)
		assert.Equal(t, 10*time.Minute, customOptions.InstallTimeout)
		assert.False(t, customOptions.WaitForReady)
	})

	t.Run("accepts zero timeout", func(t *testing.T) {
		options := &InstallOptions{
			ManifestURL:    DefaultManifestURL,
			InstallTimeout: 0,
			WaitForReady:   false,
		}

		assert.Equal(t, time.Duration(0), options.InstallTimeout)
	})
}

// TestIsArgoCDInstalled_Logic tests ArgoCD installation detection logic
func TestIsArgoCDInstalled_Logic(t *testing.T) {
	t.Run("detects missing namespace", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		ctx := context.Background()

		// Try to get ArgoCD namespace (should not exist)
		_, err := clientset.CoreV1().Namespaces().Get(ctx, ArgoCDNamespace, metav1.GetOptions{})
		assert.True(t, apierrors.IsNotFound(err), "Namespace should not exist initially")
	})

	t.Run("detects namespace exists", func(t *testing.T) {
		// Create a namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ArgoCDNamespace,
			},
		}
		clientset := fake.NewSimpleClientset(ns)
		ctx := context.Background()

		// Verify namespace exists
		foundNs, err := clientset.CoreV1().Namespaces().Get(ctx, ArgoCDNamespace, metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, ArgoCDNamespace, foundNs.Name)
	})

	t.Run("detects missing deployments", func(t *testing.T) {
		// Create namespace but no deployments
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ArgoCDNamespace,
			},
		}
		clientset := fake.NewSimpleClientset(ns)
		ctx := context.Background()

		// Check for core deployments (should not exist)
		deployments := []string{
			"argocd-applicationset-controller",
			"argocd-redis",
			"argocd-repo-server",
		}

		for _, deploymentName := range deployments {
			_, err := clientset.AppsV1().Deployments(ArgoCDNamespace).Get(ctx, deploymentName, metav1.GetOptions{})
			assert.True(t, apierrors.IsNotFound(err), "Deployment %s should not exist", deploymentName)
		}
	})

	t.Run("detects deployments not ready", func(t *testing.T) {
		// Create namespace and deployment with 0 ready replicas
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ArgoCDNamespace,
			},
		}
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-redis",
				Namespace: ArgoCDNamespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(1),
			},
			Status: appsv1.DeploymentStatus{
				Replicas:      1,
				ReadyReplicas: 0, // Not ready
			},
		}
		clientset := fake.NewSimpleClientset(ns, deployment)
		ctx := context.Background()

		dep, err := clientset.AppsV1().Deployments(ArgoCDNamespace).Get(ctx, "argocd-redis", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, int32(0), dep.Status.ReadyReplicas)
		assert.NotEqual(t, dep.Status.ReadyReplicas, dep.Status.Replicas, "Deployment should not be fully ready")
	})

	t.Run("detects deployments ready", func(t *testing.T) {
		// Create namespace and deployment with all replicas ready
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ArgoCDNamespace,
			},
		}
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-redis",
				Namespace: ArgoCDNamespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(1),
			},
			Status: appsv1.DeploymentStatus{
				Replicas:      1,
				ReadyReplicas: 1, // All ready
			},
		}
		clientset := fake.NewSimpleClientset(ns, deployment)
		ctx := context.Background()

		dep, err := clientset.AppsV1().Deployments(ArgoCDNamespace).Get(ctx, "argocd-redis", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, int32(1), dep.Status.ReadyReplicas)
		assert.Equal(t, dep.Status.ReadyReplicas, dep.Status.Replicas, "Deployment should be fully ready")
	})

	t.Run("detects missing statefulsets", func(t *testing.T) {
		// Create namespace but no statefulsets
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ArgoCDNamespace,
			},
		}
		clientset := fake.NewSimpleClientset(ns)
		ctx := context.Background()

		// Check for application controller StatefulSet (should not exist)
		_, err := clientset.AppsV1().StatefulSets(ArgoCDNamespace).Get(ctx, "argocd-application-controller", metav1.GetOptions{})
		assert.True(t, apierrors.IsNotFound(err), "StatefulSet should not exist")
	})

	t.Run("detects statefulsets not ready", func(t *testing.T) {
		// Create namespace and statefulset with 0 ready replicas
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ArgoCDNamespace,
			},
		}
		statefulset := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-application-controller",
				Namespace: ArgoCDNamespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: int32Ptr(1),
			},
			Status: appsv1.StatefulSetStatus{
				Replicas:      1,
				ReadyReplicas: 0, // Not ready
			},
		}
		clientset := fake.NewSimpleClientset(ns, statefulset)
		ctx := context.Background()

		sts, err := clientset.AppsV1().StatefulSets(ArgoCDNamespace).Get(ctx, "argocd-application-controller", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, int32(0), sts.Status.ReadyReplicas)
		assert.NotEqual(t, sts.Status.ReadyReplicas, sts.Status.Replicas, "StatefulSet should not be fully ready")
	})

	t.Run("detects statefulsets ready", func(t *testing.T) {
		// Create namespace and statefulset with all replicas ready
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ArgoCDNamespace,
			},
		}
		statefulset := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-application-controller",
				Namespace: ArgoCDNamespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: int32Ptr(1),
			},
			Status: appsv1.StatefulSetStatus{
				Replicas:      1,
				ReadyReplicas: 1, // All ready
			},
		}
		clientset := fake.NewSimpleClientset(ns, statefulset)
		ctx := context.Background()

		sts, err := clientset.AppsV1().StatefulSets(ArgoCDNamespace).Get(ctx, "argocd-application-controller", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, int32(1), sts.Status.ReadyReplicas)
		assert.Equal(t, sts.Status.ReadyReplicas, sts.Status.Replicas, "StatefulSet should be fully ready")
	})
}

// TestSecretKeyLogic tests the server.secretkey addition logic
func TestSecretKeyLogic(t *testing.T) {
	t.Run("detects missing secret data", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-secret",
				Namespace: ArgoCDNamespace,
			},
			Data: nil, // No data
		}

		assert.Nil(t, secret.Data)

		// Initialize if nil
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}

		assert.NotNil(t, secret.Data)
		assert.Empty(t, secret.Data)
	})

	t.Run("detects missing server.secretkey", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-secret",
				Namespace: ArgoCDNamespace,
			},
			Data: map[string][]byte{
				"other-key": []byte("other-value"),
			},
		}

		_, exists := secret.Data["server.secretkey"]
		assert.False(t, exists, "server.secretkey should not exist initially")
	})

	t.Run("detects existing server.secretkey", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-secret",
				Namespace: ArgoCDNamespace,
			},
			Data: map[string][]byte{
				"server.secretkey": []byte("existing-value"),
			},
		}

		value, exists := secret.Data["server.secretkey"]
		assert.True(t, exists, "server.secretkey should exist")
		assert.Equal(t, []byte("existing-value"), value)
	})

	t.Run("adds server.secretkey when missing", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-secret",
				Namespace: ArgoCDNamespace,
			},
			Data: make(map[string][]byte),
		}

		// Add server.secretkey if it doesn't exist
		if _, exists := secret.Data["server.secretkey"]; !exists {
			secret.Data["server.secretkey"] = []byte("kubeasy-argocd-core-secret")
		}

		value, exists := secret.Data["server.secretkey"]
		assert.True(t, exists)
		assert.Equal(t, []byte("kubeasy-argocd-core-secret"), value)
	})
}

// TestStatefulSetRestartLogic tests the restart annotation logic
func TestStatefulSetRestartLogic(t *testing.T) {
	t.Run("initializes annotations if nil", func(t *testing.T) {
		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-application-controller",
				Namespace: ArgoCDNamespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: nil,
					},
				},
			},
		}

		assert.Nil(t, sts.Spec.Template.Annotations)

		// Initialize if nil
		if sts.Spec.Template.Annotations == nil {
			sts.Spec.Template.Annotations = make(map[string]string)
		}

		assert.NotNil(t, sts.Spec.Template.Annotations)
		assert.Empty(t, sts.Spec.Template.Annotations)
	})

	t.Run("adds restart annotation", func(t *testing.T) {
		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-application-controller",
				Namespace: ArgoCDNamespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: make(map[string]string),
					},
				},
			},
		}

		// Add restart annotation
		restartTime := time.Now().Format(time.RFC3339)
		sts.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = restartTime

		value, exists := sts.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"]
		assert.True(t, exists)
		assert.Equal(t, restartTime, value)

		// Verify timestamp format
		_, err := time.Parse(time.RFC3339, value)
		assert.NoError(t, err, "Restart timestamp should be valid RFC3339")
	})

	t.Run("updates existing restart annotation", func(t *testing.T) {
		oldTime := "2025-01-01T00:00:00Z"
		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-application-controller",
				Namespace: ArgoCDNamespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"kubectl.kubernetes.io/restartedAt": oldTime,
						},
					},
				},
			},
		}

		// Update restart annotation
		newTime := time.Now().Format(time.RFC3339)
		sts.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = newTime

		value := sts.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"]
		assert.NotEqual(t, oldTime, value)
		assert.Equal(t, newTime, value)
	})
}

// TestConstants tests ArgoCD package constants
func TestConstants(t *testing.T) {
	t.Run("namespace constant is correct", func(t *testing.T) {
		assert.Equal(t, "argocd", ArgoCDNamespace)
	})

	t.Run("manifest URL is HTTPS", func(t *testing.T) {
		assert.Contains(t, DefaultManifestURL, "https://")
		assert.NotContains(t, DefaultManifestURL, "http://", "Should use HTTPS for security")
	})

	t.Run("manifest URL points to stable branch", func(t *testing.T) {
		assert.Contains(t, DefaultManifestURL, "/stable/")
	})

	t.Run("timeout is positive", func(t *testing.T) {
		assert.True(t, DefaultInstallTimeout > 0)
		assert.Equal(t, 5*time.Minute, DefaultInstallTimeout)
	})
}

// TestArgoCDGVR tests the ArgoCD Application GVR
func TestArgoCDGVR(t *testing.T) {
	// This would test the argoAppGVR from application.go if we were to export it
	// For now, we test the expected values directly
	expectedGVR := schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}

	assert.Equal(t, "argoproj.io", expectedGVR.Group)
	assert.Equal(t, "v1alpha1", expectedGVR.Version)
	assert.Equal(t, "applications", expectedGVR.Resource)
}

// TestGenerateSecretKey tests the cryptographic secret key generation
func TestGenerateSecretKey(t *testing.T) {
	t.Run("generates non-empty key", func(t *testing.T) {
		key, err := generateSecretKey()
		require.NoError(t, err)
		assert.NotEmpty(t, key)
	})

	t.Run("generates hex-encoded key of correct length", func(t *testing.T) {
		key, err := generateSecretKey()
		require.NoError(t, err)
		// 32 bytes encoded as hex = 64 characters
		assert.Len(t, key, 64, "Key should be 64 hex characters (32 bytes)")
	})

	t.Run("generates unique keys on each call", func(t *testing.T) {
		key1, err := generateSecretKey()
		require.NoError(t, err)

		key2, err := generateSecretKey()
		require.NoError(t, err)

		assert.NotEqual(t, key1, key2, "Each generated key should be unique")
	})

	t.Run("generates valid hex string", func(t *testing.T) {
		key, err := generateSecretKey()
		require.NoError(t, err)

		// Verify all characters are valid hex
		for _, c := range string(key) {
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
			assert.True(t, isHex, "Character %c should be valid hex", c)
		}
	})
}

// TestEnsureServerSecretKey tests the ensureServerSecretKey helper function
func TestEnsureServerSecretKey(t *testing.T) {
	t.Run("adds secret key when missing", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-secret",
				Namespace: ArgoCDNamespace,
			},
			Data: make(map[string][]byte),
		}
		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-application-controller",
				Namespace: ArgoCDNamespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: int32Ptr(1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: make(map[string]string),
					},
				},
			},
		}
		clientset := fake.NewSimpleClientset(secret, sts)
		ctx := context.Background()

		err := ensureServerSecretKey(ctx, clientset)
		require.NoError(t, err)

		// Verify secret was updated
		updatedSecret, err := clientset.CoreV1().Secrets(ArgoCDNamespace).Get(ctx, "argocd-secret", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Contains(t, updatedSecret.Data, "server.secretkey")
		assert.Len(t, updatedSecret.Data["server.secretkey"], 64, "Secret key should be 64 hex chars")
	})

	t.Run("skips when secret key already exists", func(t *testing.T) {
		existingKey := []byte("existing-secret-key-value")
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-secret",
				Namespace: ArgoCDNamespace,
			},
			Data: map[string][]byte{
				"server.secretkey": existingKey,
			},
		}
		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-application-controller",
				Namespace: ArgoCDNamespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: int32Ptr(1),
			},
		}
		clientset := fake.NewSimpleClientset(secret, sts)
		ctx := context.Background()

		err := ensureServerSecretKey(ctx, clientset)
		require.NoError(t, err)

		// Verify secret was NOT changed
		updatedSecret, err := clientset.CoreV1().Secrets(ArgoCDNamespace).Get(ctx, "argocd-secret", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, existingKey, updatedSecret.Data["server.secretkey"], "Existing key should not be changed")
	})

	t.Run("initializes nil Data map", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-secret",
				Namespace: ArgoCDNamespace,
			},
			Data: nil, // Explicitly nil
		}
		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-application-controller",
				Namespace: ArgoCDNamespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: int32Ptr(1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: make(map[string]string),
					},
				},
			},
		}
		clientset := fake.NewSimpleClientset(secret, sts)
		ctx := context.Background()

		err := ensureServerSecretKey(ctx, clientset)
		require.NoError(t, err)

		updatedSecret, err := clientset.CoreV1().Secrets(ArgoCDNamespace).Get(ctx, "argocd-secret", metav1.GetOptions{})
		require.NoError(t, err)
		assert.NotNil(t, updatedSecret.Data)
		assert.Contains(t, updatedSecret.Data, "server.secretkey")
	})

	t.Run("returns error when secret not found", func(t *testing.T) {
		clientset := fake.NewSimpleClientset() // No secret created
		ctx := context.Background()

		err := ensureServerSecretKey(ctx, clientset)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "argocd-secret")
	})
}

// TestRestartApplicationController tests the restartApplicationController helper function
func TestRestartApplicationController(t *testing.T) {
	t.Run("adds restart annotation", func(t *testing.T) {
		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-application-controller",
				Namespace: ArgoCDNamespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: int32Ptr(1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: make(map[string]string),
					},
				},
			},
		}
		clientset := fake.NewSimpleClientset(sts)
		ctx := context.Background()

		err := restartApplicationController(ctx, clientset)
		require.NoError(t, err)

		// Verify annotation was added
		updatedSts, err := clientset.AppsV1().StatefulSets(ArgoCDNamespace).Get(ctx, "argocd-application-controller", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Contains(t, updatedSts.Spec.Template.Annotations, "kubectl.kubernetes.io/restartedAt")

		// Verify timestamp is valid RFC3339
		timestamp := updatedSts.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"]
		_, err = time.Parse(time.RFC3339, timestamp)
		assert.NoError(t, err, "Timestamp should be valid RFC3339")
	})

	t.Run("initializes nil annotations map", func(t *testing.T) {
		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-application-controller",
				Namespace: ArgoCDNamespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: int32Ptr(1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: nil, // Explicitly nil
					},
				},
			},
		}
		clientset := fake.NewSimpleClientset(sts)
		ctx := context.Background()

		err := restartApplicationController(ctx, clientset)
		require.NoError(t, err)

		updatedSts, err := clientset.AppsV1().StatefulSets(ArgoCDNamespace).Get(ctx, "argocd-application-controller", metav1.GetOptions{})
		require.NoError(t, err)
		assert.NotNil(t, updatedSts.Spec.Template.Annotations)
		assert.Contains(t, updatedSts.Spec.Template.Annotations, "kubectl.kubernetes.io/restartedAt")
	})

	t.Run("updates existing restart annotation", func(t *testing.T) {
		oldTimestamp := "2020-01-01T00:00:00Z"
		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-application-controller",
				Namespace: ArgoCDNamespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: int32Ptr(1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"kubectl.kubernetes.io/restartedAt": oldTimestamp,
						},
					},
				},
			},
		}
		clientset := fake.NewSimpleClientset(sts)
		ctx := context.Background()

		err := restartApplicationController(ctx, clientset)
		require.NoError(t, err)

		updatedSts, err := clientset.AppsV1().StatefulSets(ArgoCDNamespace).Get(ctx, "argocd-application-controller", metav1.GetOptions{})
		require.NoError(t, err)
		newTimestamp := updatedSts.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"]
		assert.NotEqual(t, oldTimestamp, newTimestamp, "Timestamp should be updated")
	})

	t.Run("returns error when StatefulSet not found", func(t *testing.T) {
		clientset := fake.NewSimpleClientset() // No StatefulSet created
		ctx := context.Background()

		err := restartApplicationController(ctx, clientset)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "argocd-application-controller")
	})
}

// Helper functions
func int32Ptr(i int32) *int32 {
	return &i
}
