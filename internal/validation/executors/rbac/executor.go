// Package rbac implements the "rbac" validation type.
// It validates ServiceAccount permissions using SubjectAccessReview.
package rbac

import (
	"context"
	"fmt"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/vtypes"
	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const msgAllChecksPassed = "All RBAC checks passed" //nolint:gosec // not a credential

// Execute validates ServiceAccount permissions for all specified checks.
func Execute(ctx context.Context, spec vtypes.RbacSpec, deps shared.Deps) (bool, string, error) {
	saUser := fmt.Sprintf("system:serviceaccount:%s:%s", spec.Namespace, spec.ServiceAccount)

	for i, check := range spec.Checks {
		checkNS := spec.Namespace
		if check.Namespace != "" {
			checkNS = check.Namespace
		}

		sar := &authv1.SubjectAccessReview{
			Spec: authv1.SubjectAccessReviewSpec{
				User: saUser,
				// Include SA groups so that permissions granted via group bindings
				// (system:serviceaccounts, system:serviceaccounts:<ns>) are honoured,
				// matching the behaviour of kubectl auth can-i --as system:serviceaccount:ns:sa
				Groups: []string{
					"system:serviceaccounts",
					fmt.Sprintf("system:serviceaccounts:%s", spec.Namespace),
				},
				ResourceAttributes: &authv1.ResourceAttributes{
					Verb:        check.Verb,
					Resource:    check.Resource,
					Subresource: check.Subresource,
					Namespace:   checkNS,
				},
			},
		}

		result, err := deps.Clientset.AuthorizationV1().SubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return false, "", fmt.Errorf("check %d: SubjectAccessReview failed: %w", i, err)
		}

		if result.Status.Allowed != check.Allowed {
			expected := "allowed"
			actual := "denied"
			if !check.Allowed {
				expected = "denied"
				actual = "allowed"
			}
			return false, fmt.Sprintf(
				"check %d: %s %s in namespace %q: expected %s but was %s",
				i, check.Verb, check.Resource, checkNS, expected, actual,
			), nil
		}
	}

	return true, msgAllChecksPassed, nil
}
