package argocd

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

// Helper function to create a fake ArgoCD Application
func createFakeApp(name, namespace string, health, sync, phase string) *unstructured.Unstructured {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":            name,
				"namespace":       namespace,
				"resourceVersion": "1",
			},
		},
	}

	if health != "" || sync != "" || phase != "" {
		status := make(map[string]interface{})
		if health != "" {
			status["health"] = map[string]interface{}{
				"status": health,
			}
		}
		if sync != "" {
			status["sync"] = map[string]interface{}{
				"status": sync,
			}
		}
		if phase != "" {
			status["operationState"] = map[string]interface{}{
				"phase": phase,
			}
		}
		app.Object["status"] = status
	}

	return app
}

func TestDeleteChallengeApplication_Success(t *testing.T) {
	scheme := runtime.NewScheme()
	app := createFakeApp("test-challenge", ArgoCDNamespace, "Healthy", "Synced", "")

	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	deleteCalled := false

	// Make Delete return immediately (simulate successful deletion)
	dynamicClient.PrependReactor("delete", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		deleteCalled = true
		return true, nil, nil
	})

	// Make subsequent Get calls return NotFound (simulating successful deletion)
	getCallCount := 0
	dynamicClient.PrependReactor("get", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		getCallCount++
		// First call for finalizer check, second call for deletion verification
		if getCallCount == 1 {
			return true, app, nil
		}
		// After patch, return NotFound to simulate deletion
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, "test-challenge")
	})

	// Handle patch for finalizer
	dynamicClient.PrependReactor("patch", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, app, nil
	})

	ctx := context.Background()

	err := DeleteChallengeApplication(ctx, dynamicClient, "test-challenge", ArgoCDNamespace)

	require.NoError(t, err)
	assert.True(t, deleteCalled, "Delete should have been called")
}

func TestDeleteChallengeApplication_NotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	// Return NotFound immediately
	dynamicClient.PrependReactor("get", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, "nonexistent")
	})

	ctx := context.Background()

	err := DeleteChallengeApplication(ctx, dynamicClient, "nonexistent", ArgoCDNamespace)

	// Should return nil (idempotent - app already deleted)
	require.NoError(t, err)
}

func TestDeleteChallengeApplication_FinalizerLogic(t *testing.T) {
	t.Run("adds finalizer when missing", func(t *testing.T) {
		scheme := runtime.NewScheme()
		app := createFakeApp("test-challenge", ArgoCDNamespace, "Healthy", "Synced", "")
		// No finalizers initially

		dynamicClient := fake.NewSimpleDynamicClient(scheme)

		// Track patch calls
		var patchedFinalizers []string
		patchCount := 0
		dynamicClient.PrependReactor("patch", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			patchAction := action.(k8stesting.PatchAction)
			var patchObj map[string]interface{}
			err = json.Unmarshal(patchAction.GetPatch(), &patchObj)
			require.NoError(t, err)

			metadata := patchObj["metadata"].(map[string]interface{})
			finalizers := metadata["finalizers"].([]interface{})
			for _, f := range finalizers {
				patchedFinalizers = append(patchedFinalizers, f.(string))
			}
			patchCount++

			// Return updated app with finalizer
			updatedApp := app.DeepCopy()
			err = unstructured.SetNestedStringSlice(updatedApp.Object, patchedFinalizers, "metadata", "finalizers")
			require.NoError(t, err)
			return true, updatedApp, nil
		})

		// Make Get return the patched app, then NotFound after delete
		getCallCount := 0
		deleteHappened := false
		dynamicClient.PrependReactor("get", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			getCallCount++
			if getCallCount == 1 {
				return true, app, nil // Initial get without finalizer
			}
			// After delete, return NotFound
			if deleteHappened {
				return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, "test-challenge")
			}
			// After any patch, immediately return with finalizer (simulate fast update)
			appWithFinalizer := app.DeepCopy()
			err = unstructured.SetNestedStringSlice(appWithFinalizer.Object, []string{"resources-finalizer.argocd.argoproj.io"}, "metadata", "finalizers")
			require.NoError(t, err)
			return true, appWithFinalizer, nil
		})

		// Simulate deletion completion
		deleteCount := 0
		dynamicClient.PrependReactor("delete", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			deleteCount++
			deleteHappened = true
			return true, nil, nil
		})

		ctx := context.Background()

		err := DeleteChallengeApplication(ctx, dynamicClient, "test-challenge", ArgoCDNamespace)

		require.NoError(t, err)
		assert.Equal(t, 1, patchCount, "Patch should have been called once")
		assert.Contains(t, patchedFinalizers, "resources-finalizer.argocd.argoproj.io/")
		assert.Equal(t, 1, deleteCount, "Delete should have been called")
	})

	t.Run("preserves existing finalizer", func(t *testing.T) {
		scheme := runtime.NewScheme()
		app := createFakeApp("test-challenge", ArgoCDNamespace, "Healthy", "Synced", "")
		// Already has the finalizer
		err := unstructured.SetNestedStringSlice(app.Object, []string{"resources-finalizer.argocd.argoproj.io/"}, "metadata", "finalizers")
		require.NoError(t, err)

		dynamicClient := fake.NewSimpleDynamicClient(scheme)

		var patchCalled bool
		dynamicClient.PrependReactor("patch", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			patchCalled = true
			return true, app, nil
		})

		// Return app with finalizer, then NotFound after delete
		getCount := 0
		deleteHappened := false
		dynamicClient.PrependReactor("get", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			getCount++
			if deleteHappened {
				return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, "test-challenge")
			}
			return true, app, nil
		})

		dynamicClient.PrependReactor("delete", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			deleteHappened = true
			return true, nil, nil
		})

		ctx := context.Background()

		err = DeleteChallengeApplication(ctx, dynamicClient, "test-challenge", ArgoCDNamespace)

		require.NoError(t, err)
		assert.False(t, patchCalled, "Should not patch when finalizer already exists")
	})
}

func TestWaitForApplicationStatus_HealthAndSync(t *testing.T) {
	scheme := runtime.NewScheme()
	app := createFakeApp("test-app", ArgoCDNamespace, "Healthy", "Synced", "Succeeded")

	dynamicClient := fake.NewSimpleDynamicClient(scheme, app)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	options := WaitOptions{
		CheckHealth: true,
		CheckSync:   true,
		Timeout:     0,
	}

	err := WaitForApplicationStatus(ctx, dynamicClient, "test-app", ArgoCDNamespace, options)

	require.NoError(t, err)
}

func TestWaitForApplicationStatus_OnlyHealth(t *testing.T) {
	scheme := runtime.NewScheme()
	app := createFakeApp("test-app", ArgoCDNamespace, "Healthy", "OutOfSync", "")

	dynamicClient := fake.NewSimpleDynamicClient(scheme, app)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	options := WaitOptions{
		CheckHealth: true,
		CheckSync:   false,
		Timeout:     0,
	}

	err := WaitForApplicationStatus(ctx, dynamicClient, "test-app", ArgoCDNamespace, options)

	require.NoError(t, err)
}

func TestWaitForApplicationStatus_OnlySync(t *testing.T) {
	scheme := runtime.NewScheme()
	app := createFakeApp("test-app", ArgoCDNamespace, "Degraded", "Synced", "")

	dynamicClient := fake.NewSimpleDynamicClient(scheme, app)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	options := WaitOptions{
		CheckHealth: false,
		CheckSync:   true,
		Timeout:     0,
	}

	err := WaitForApplicationStatus(ctx, dynamicClient, "test-app", ArgoCDNamespace, options)

	require.NoError(t, err)
}

func TestWaitForApplicationStatus_Timeout(t *testing.T) {
	scheme := runtime.NewScheme()
	app := createFakeApp("test-app", ArgoCDNamespace, "Progressing", "OutOfSync", "Running")

	dynamicClient := fake.NewSimpleDynamicClient(scheme, app)

	ctx := context.Background()

	options := WaitOptions{
		CheckHealth: true,
		CheckSync:   true,
		Timeout:     100 * time.Millisecond,
	}

	err := WaitForApplicationStatus(ctx, dynamicClient, "test-app", ArgoCDNamespace, options)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestWaitForApplicationStatus_EventualSuccess(t *testing.T) {
	scheme := runtime.NewScheme()
	app := createFakeApp("test-app", ArgoCDNamespace, "Progressing", "OutOfSync", "Running")

	dynamicClient := fake.NewSimpleDynamicClient(scheme, app)

	// After 2 gets, return healthy and synced
	getCount := 0
	dynamicClient.PrependReactor("get", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		getCount++
		if getCount <= 2 {
			return true, createFakeApp("test-app", ArgoCDNamespace, "Progressing", "OutOfSync", "Running"), nil
		}
		return true, createFakeApp("test-app", ArgoCDNamespace, "Healthy", "Synced", "Succeeded"), nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	options := WaitOptions{
		CheckHealth: true,
		CheckSync:   true,
		Timeout:     0,
	}

	err := WaitForApplicationStatus(ctx, dynamicClient, "test-app", ArgoCDNamespace, options)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, getCount, 3, "Should have polled multiple times before success")
}

func TestGetApplicationFullStatus_Success(t *testing.T) {
	scheme := runtime.NewScheme()
	app := createFakeApp("test-app", ArgoCDNamespace, "Healthy", "Synced", "Succeeded")

	dynamicClient := fake.NewSimpleDynamicClient(scheme, app)

	health, sync, phase, err := getApplicationFullStatus(context.Background(), dynamicClient, "test-app", ArgoCDNamespace)

	require.NoError(t, err)
	assert.Equal(t, "Healthy", health)
	assert.Equal(t, "Synced", sync)
	assert.Equal(t, "Succeeded", phase)
}

func TestGetApplicationFullStatus_MissingStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	// Create app without status fields
	app := createFakeApp("test-app", ArgoCDNamespace, "", "", "")

	dynamicClient := fake.NewSimpleDynamicClient(scheme, app)

	health, sync, phase, err := getApplicationFullStatus(context.Background(), dynamicClient, "test-app", ArgoCDNamespace)

	require.NoError(t, err)
	assert.Equal(t, "", health)
	assert.Equal(t, "", sync)
	assert.Equal(t, "", phase)
}

func TestGetApplicationFullStatus_NotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	_, _, _, err := getApplicationFullStatus(context.Background(), dynamicClient, "nonexistent", ArgoCDNamespace)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get ArgoCD Application")
}

func TestGetApplicationFullStatusWithConditions_Success(t *testing.T) {
	scheme := runtime.NewScheme()
	app := createFakeApp("test-app", ArgoCDNamespace, "Degraded", "OutOfSync", "Failed")

	// Add conditions
	conditions := []interface{}{
		map[string]interface{}{
			"type":    "ComparisonError",
			"message": "Failed to compare desired state",
		},
		map[string]interface{}{
			"type":    "SyncError",
			"message": "Sync operation failed",
		},
	}
	err := unstructured.SetNestedSlice(app.Object, conditions, "status", "conditions")
	require.NoError(t, err)

	dynamicClient := fake.NewSimpleDynamicClient(scheme, app)

	health, sync, phase, conditionsStr, err := getApplicationFullStatusWithConditions(context.Background(), dynamicClient, "test-app", ArgoCDNamespace)

	require.NoError(t, err)
	assert.Equal(t, "Degraded", health)
	assert.Equal(t, "OutOfSync", sync)
	assert.Equal(t, "Failed", phase)
	assert.Contains(t, conditionsStr, "ComparisonError: Failed to compare desired state")
	assert.Contains(t, conditionsStr, "SyncError: Sync operation failed")
}

func TestGetApplicationFullStatusWithConditions_NoConditions(t *testing.T) {
	scheme := runtime.NewScheme()
	app := createFakeApp("test-app", ArgoCDNamespace, "Healthy", "Synced", "Succeeded")

	dynamicClient := fake.NewSimpleDynamicClient(scheme, app)

	health, sync, phase, conditionsStr, err := getApplicationFullStatusWithConditions(context.Background(), dynamicClient, "test-app", ArgoCDNamespace)

	require.NoError(t, err)
	assert.Equal(t, "Healthy", health)
	assert.Equal(t, "Synced", sync)
	assert.Equal(t, "Succeeded", phase)
	assert.Equal(t, "none", conditionsStr)
}

func TestDefaultWaitOptions(t *testing.T) {
	options := DefaultWaitOptions()

	assert.True(t, options.CheckHealth)
	assert.True(t, options.CheckSync)
	assert.Equal(t, time.Duration(0), options.Timeout)
}

func TestWaitOptions_CustomValues(t *testing.T) {
	options := WaitOptions{
		CheckHealth: false,
		CheckSync:   true,
		Timeout:     5 * time.Minute,
	}

	assert.False(t, options.CheckHealth)
	assert.True(t, options.CheckSync)
	assert.Equal(t, 5*time.Minute, options.Timeout)
}

func TestArgoAppGVR(t *testing.T) {
	assert.Equal(t, "argoproj.io", argoAppGVR.Group)
	assert.Equal(t, "v1alpha1", argoAppGVR.Version)
	assert.Equal(t, "applications", argoAppGVR.Resource)
}
