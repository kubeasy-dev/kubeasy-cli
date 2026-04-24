package api

import (
	"context"
	"net/http"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/apigen"
	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/keystore"
)

// getAuthToken retrieves the API token from available storage
func getAuthToken() (string, error) {
	token, err := keystore.Get()
	if err != nil {
		return "", err
	}
	return token, nil
}

// NewAuthenticatedClient creates an apigen.ClientWithResponses with Bearer token authentication.
func NewAuthenticatedClient() (*apigen.ClientWithResponses, error) {
	token, err := getAuthToken()
	if err != nil {
		return nil, err
	}

	return apigen.NewClientWithResponses(
		constants.WebsiteURL,
		apigen.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+token)
			return nil
		}),
		apigen.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
	)
}

// NewPublicClient creates an apigen.ClientWithResponses without authentication.
// Use this for public endpoints that don't require a Bearer token.
func NewPublicClient() (*apigen.ClientWithResponses, error) {
	return apigen.NewClientWithResponses(
		constants.WebsiteURL,
		apigen.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
	)
}
