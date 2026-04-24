package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

// setupMockServer creates a test HTTP server with common test handlers
func setupMockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

// setupKeyring sets up a mock keyring for testing
func setupKeyring(t *testing.T, token string) {
	t.Helper()
	keyring.MockInit()
	err := keyring.Set(constants.KeyringServiceName, "api_key", token)
	require.NoError(t, err, "Failed to set mock keyring")
}

// cleanupKeyring cleans up the mock keyring
func cleanupKeyring(t *testing.T) {
	t.Helper()
	_ = keyring.Delete(constants.KeyringServiceName, "api_key")
}

// overrideServerURL overrides WebsiteURL for testing, returns cleanup func
func overrideServerURL(t *testing.T, serverURL string) func() {
	t.Helper()
	oldWebsiteURL := constants.WebsiteURL
	constants.WebsiteURL = serverURL
	return func() {
		constants.WebsiteURL = oldWebsiteURL
	}
}

func TestGetProfile_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/user/me", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"id":    "user-123",
			"email": "test@example.com",
			"name":  "Test User",
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	profile, err := GetProfile(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "Test", profile.FirstName)
	assert.NotNil(t, profile.LastName)
	assert.Equal(t, "User", *profile.LastName)
}

func TestGetProfile_APIError(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		response := ErrorResponse{Error: "Unauthorized"}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	profile, err := GetProfile(context.Background())

	require.Error(t, err)
	assert.Nil(t, profile)
	assert.Contains(t, err.Error(), "API error: Unauthorized")
}

func TestGetProfile_InvalidJSON(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	profile, err := GetProfile(context.Background())

	require.Error(t, err)
	assert.Nil(t, profile)
}

func TestGetChallengeBySlug_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/challenges/pod-evicted", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"challenge": map[string]interface{}{
				"slug":             "pod-evicted",
				"title":            "Pod Evicted",
				"description":      "Fix a pod that keeps getting evicted",
				"difficulty":       "easy",
				"theme":            "resources-scaling",
				"initialSituation": "A pod is being evicted",
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	challenge, err := GetChallengeBySlug(context.Background(), "pod-evicted")

	require.NoError(t, err)
	assert.Equal(t, "pod-evicted", challenge.Slug)
	assert.Equal(t, "Pod Evicted", challenge.Title)
}

func TestGetChallengeBySlug_NotFound(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		response := ErrorResponse{Error: "Not found"}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	challenge, err := GetChallengeBySlug(context.Background(), "nonexistent")

	require.Error(t, err)
	assert.Nil(t, challenge)
	assert.Contains(t, err.Error(), "challenge 'nonexistent' not found")
}

func TestGetChallengeStatus_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/progress/pod-evicted", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"status":    "in_progress",
			"startedAt": "2024-01-01T00:00:00Z",
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	status, err := GetChallengeStatus(context.Background(), "pod-evicted")

	require.NoError(t, err)
	assert.Equal(t, "in_progress", status.Status)
	assert.NotNil(t, status.StartedAt)
}

func TestStartChallengeWithResponse_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/progress/pod-evicted/start", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"status":    "in_progress",
			"startedAt": "2024-01-01T00:00:00Z",
			"message":   "Challenge started successfully",
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	response, err := StartChallengeWithResponse(context.Background(), "pod-evicted")

	require.NoError(t, err)
	assert.Equal(t, "in_progress", response.Status)
	assert.Equal(t, "2024-01-01T00:00:00Z", response.StartedAt)
	assert.NotNil(t, response.Message)
	assert.Equal(t, "Challenge started successfully", *response.Message)
}

func TestSubmitChallenge_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/challenges/pod-evicted/submit", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := ChallengeSubmitResponse{
			Success: true,
			Message: strPtr("All validations passed"),
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	req := ChallengeSubmitRequest{
		Results: []ObjectiveResult{
			{ObjectiveKey: "obj-1", Passed: true, Message: strPtr("Passed")},
		},
	}
	response, err := SubmitChallenge(context.Background(), "pod-evicted", req)

	require.NoError(t, err)
	assert.True(t, response.Success)
}

func TestResetChallenge_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/progress/pod-evicted/reset", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := ChallengeResetResponse{
			Success: true,
			Message: "Challenge reset successfully",
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	response, err := ResetChallenge(context.Background(), "pod-evicted")

	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "Challenge reset successfully", response.Message)
}

func TestLogin_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if r.Method == "POST" && r.URL.Path == "/api/cli/track/login" {
			_, _ = w.Write([]byte(`{"firstLogin":true}`))
			return
		}

		if r.Method == "GET" && r.URL.Path == "/api/user/me" {
			response := map[string]interface{}{
				"id":    "user-123",
				"email": "test@example.com",
				"name":  "Paul Brissaud",
			}
			_ = json.NewEncoder(w).Encode(response)
			return
		}
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	result, err := Login(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "Paul", result.FirstName)
	require.NotNil(t, result.LastName)
	assert.Equal(t, "Brissaud", *result.LastName)
	require.NotNil(t, result.FirstLogin)
	assert.True(t, *result.FirstLogin)
}

func TestTrackSetup_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	called := false
	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/cli/track/setup", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"firstTime":false}`))
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	TrackSetup(context.Background())

	assert.True(t, called, "expected tracking request to be sent")
}

// Helper function for string pointers
func strPtr(s string) *string {
	return &s
}

// TestMain sets up test environment
func TestMain(m *testing.M) {
	keyring.MockInit()
	code := m.Run()
	os.Exit(code)
}
