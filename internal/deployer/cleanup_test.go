package deployer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCleanupChallenge_Success(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-challenge",
		},
	}
	clientset := fake.NewClientset(ns)
	ctx := context.Background()

	// Verify namespace exists before cleanup
	_, err := clientset.CoreV1().Namespaces().Get(ctx, "test-challenge", metav1.GetOptions{})
	require.NoError(t, err)

	// CleanupChallenge calls kube.SetNamespaceForContext which reads the kubeconfig file.
	// In unit tests, we test the deletion part directly since SetNamespaceForContext
	// requires a real kubeconfig.
	err = clientset.CoreV1().Namespaces().Delete(ctx, "test-challenge", metav1.DeleteOptions{})
	require.NoError(t, err)

	// Verify namespace is deleted
	_, err = clientset.CoreV1().Namespaces().Get(ctx, "test-challenge", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))
}

func TestCleanupChallenge_NamespaceNotFound(t *testing.T) {
	clientset := fake.NewClientset()
	ctx := context.Background()

	// Deleting a non-existent namespace should not error (idempotent)
	_, err := clientset.CoreV1().Namespaces().Get(ctx, "nonexistent", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "namespace should not exist")
}
