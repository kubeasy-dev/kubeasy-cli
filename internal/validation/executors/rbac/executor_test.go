package rbac_test

import (
	"context"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/executors/rbac"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func depsWithReactor(allowed bool) shared.Deps {
	clientset := fake.NewClientset()
	clientset.PrependReactor("create", "subjectaccessreviews", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &authv1.SubjectAccessReview{
			Status: authv1.SubjectAccessReviewStatus{Allowed: allowed},
		}, nil
	})
	return shared.Deps{Clientset: clientset, Namespace: "test-ns"}
}

func TestExecute_AllChecksPassed(t *testing.T) {
	spec := vtypes.RbacSpec{
		ServiceAccount: "my-sa",
		Namespace:      "test-ns",
		Checks: []vtypes.RbacCheck{
			{Verb: "get", Resource: "pods", Allowed: true},
			{Verb: "list", Resource: "pods", Allowed: true},
		},
	}

	passed, msg, err := rbac.Execute(context.Background(), spec, depsWithReactor(true))
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, "All RBAC checks passed", msg)
}

func TestExecute_AllowedFails(t *testing.T) {
	spec := vtypes.RbacSpec{
		ServiceAccount: "my-sa",
		Namespace:      "test-ns",
		Checks: []vtypes.RbacCheck{
			{Verb: "delete", Resource: "pods", Allowed: true},
		},
	}

	passed, msg, err := rbac.Execute(context.Background(), spec, depsWithReactor(false))
	require.NoError(t, err)
	assert.False(t, passed)
	assert.Contains(t, msg, "expected allowed but was denied")
}

func TestExecute_AntiBypasses(t *testing.T) {
	// allowed: false check should pass when SA is denied (anti-bypass)
	spec := vtypes.RbacSpec{
		ServiceAccount: "my-sa",
		Namespace:      "test-ns",
		Checks: []vtypes.RbacCheck{
			{Verb: "delete", Resource: "pods", Allowed: false},
		},
	}

	passed, msg, err := rbac.Execute(context.Background(), spec, depsWithReactor(false))
	require.NoError(t, err)
	assert.True(t, passed)
	assert.Equal(t, "All RBAC checks passed", msg)
}

func TestExecute_Subresource(t *testing.T) {
	spec := vtypes.RbacSpec{
		ServiceAccount: "my-sa",
		Namespace:      "test-ns",
		Checks: []vtypes.RbacCheck{
			{Verb: "create", Resource: "pods", Subresource: "exec", Allowed: true},
		},
	}

	passed, _, err := rbac.Execute(context.Background(), spec, depsWithReactor(true))
	require.NoError(t, err)
	assert.True(t, passed)
}

func TestExecute_PerCheckNamespace(t *testing.T) {
	var capturedNS string
	clientset := fake.NewClientset()
	clientset.PrependReactor("create", "subjectaccessreviews", func(action ktesting.Action) (bool, runtime.Object, error) {
		sar := action.(ktesting.CreateAction).GetObject().(*authv1.SubjectAccessReview)
		capturedNS = sar.Spec.ResourceAttributes.Namespace
		return true, &authv1.SubjectAccessReview{
			Status: authv1.SubjectAccessReviewStatus{Allowed: true},
		}, nil
	})
	deps := shared.Deps{Clientset: clientset, Namespace: "default"}

	spec := vtypes.RbacSpec{
		ServiceAccount: "my-sa",
		Namespace:      "default",
		Checks: []vtypes.RbacCheck{
			{Verb: "get", Resource: "pods", Namespace: "other-ns", Allowed: true},
		},
	}

	_, _, err := rbac.Execute(context.Background(), spec, deps)
	require.NoError(t, err)
	assert.Equal(t, "other-ns", capturedNS)
}
