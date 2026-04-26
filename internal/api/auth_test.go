package api

import (
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/keystore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAuthToken_Success(t *testing.T) {
	// Set token in keystore for testing
	testToken := "test-token-123"
	_, err := keystore.Set(testToken)
	require.NoError(t, err)
	defer func() { _ = keystore.Delete() }()

	token, err := getAuthToken()
	require.NoError(t, err)
	assert.Equal(t, testToken, token)
}

func TestGetAuthToken_Error(t *testing.T) {
	// Delete token from keystore
	_ = keystore.Delete()

	_, err := getAuthToken()
	require.Error(t, err)
}

func TestNewPublicClient(t *testing.T) {
	client, err := NewPublicClient()
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewAuthenticatedClient_Success(t *testing.T) {
	// Set token in keystore
	testToken := "test-auth-token"
	_, err := keystore.Set(testToken)
	require.NoError(t, err)
	defer func() { _ = keystore.Delete() }()

	client, err := NewAuthenticatedClient()
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewAuthenticatedClient_NoToken(t *testing.T) {
	// Ensure no token
	_ = keystore.Delete()

	client, err := NewAuthenticatedClient()
	require.Error(t, err)
	assert.Nil(t, client)
}
