package api

import (
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
		assert.Equal(t, "/api/cli/user", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := UserProfile{
			FirstName: "Test",
			LastName:  strPtr("User"),
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	profile, err := GetProfile()

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

	profile, err := GetProfile()

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

	profile, err := GetProfile()

	require.Error(t, err)
	assert.Nil(t, profile)
}

func TestGetUserProfile_Alias(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := UserProfile{FirstName: "Test", LastName: strPtr("User")}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	profile, err := GetUserProfile()

	require.NoError(t, err)
	assert.Equal(t, "Test", profile.FirstName)
}

func TestGetChallengeBySlug_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/cli/challenge/pod-evicted", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := ChallengeEntity{
			ID:               123,
			Slug:             "pod-evicted",
			Title:            "Pod Evicted",
			Description:      "Fix a pod that keeps getting evicted",
			Difficulty:       "easy",
			Theme:            "resources-scaling",
			InitialSituation: "A pod is being evicted",
			Objective:        "Fix the pod",
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	challenge, err := GetChallengeBySlug("pod-evicted")

	require.NoError(t, err)
	assert.Equal(t, 123, challenge.ID)
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

	challenge, err := GetChallengeBySlug("nonexistent")

	require.Error(t, err)
	assert.Nil(t, challenge)
	assert.Contains(t, err.Error(), "challenge 'nonexistent' not found")
}

func TestGetChallengeStatus_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/cli/challenge/pod-evicted/status", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := ChallengeStatusResponse{
			Status:    "in_progress",
			StartedAt: strPtr("2024-01-01T00:00:00Z"),
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	status, err := GetChallengeStatus("pod-evicted")

	require.NoError(t, err)
	assert.Equal(t, "in_progress", status.Status)
	assert.NotNil(t, status.StartedAt)
}

func TestStartChallengeWithResponse_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/cli/challenge/pod-evicted/start", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := ChallengeStartResponse{
			Status:    "in_progress",
			StartedAt: "2024-01-01T00:00:00Z",
			Message:   strPtr("Challenge started successfully"),
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	response, err := StartChallengeWithResponse("pod-evicted")

	require.NoError(t, err)
	assert.Equal(t, "in_progress", response.Status)
	assert.Equal(t, "2024-01-01T00:00:00Z", response.StartedAt)
	assert.NotNil(t, response.Message)
	assert.Equal(t, "Challenge started successfully", *response.Message)
}

func TestStartChallenge_BackwardCompatibility(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := ChallengeStartResponse{
			Status:    "in_progress",
			StartedAt: "2024-01-01T00:00:00Z",
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	err := StartChallenge("pod-evicted")

	require.NoError(t, err)
}

func TestSubmitChallenge_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/cli/challenge/pod-evicted/submit", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req ChallengeSubmitRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Len(t, req.Results, 2)

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
			{ObjectiveKey: "obj-2", Passed: true, Message: strPtr("Passed")},
		},
	}
	response, err := SubmitChallenge("pod-evicted", req)

	require.NoError(t, err)
	assert.True(t, response.Success)
}

func TestSubmitChallenge_PartialSuccess(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		response := ChallengeSubmitResponse{
			Success: false,
			Message: strPtr("Some validations failed"),
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	req := ChallengeSubmitRequest{
		Results: []ObjectiveResult{
			{ObjectiveKey: "obj-1", Passed: false, Message: strPtr("Failed")},
		},
	}
	response, err := SubmitChallenge("pod-evicted", req)

	require.NoError(t, err)
	assert.False(t, response.Success)
}

func TestSendSubmit_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := ChallengeSubmitResponse{Success: true, Message: strPtr("Success")}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	results := []ObjectiveResult{
		{ObjectiveKey: "obj-1", Passed: true, Message: strPtr("Passed")},
	}
	err := SendSubmit("pod-evicted", results)

	require.NoError(t, err)
}

func TestSendSubmit_Failure(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := ChallengeSubmitResponse{
			Success: false,
			Message: strPtr("Validation failed"),
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	results := []ObjectiveResult{
		{ObjectiveKey: "obj-1", Passed: false, Message: strPtr("Failed")},
	}
	err := SendSubmit("pod-evicted", results)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Validation failed")
}

func TestResetChallenge_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/cli/challenge/pod-evicted/reset", r.URL.Path)

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

	response, err := ResetChallenge("pod-evicted")

	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "Challenge reset successfully", response.Message)
}

func TestResetChallengeProgress_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := ChallengeResetResponse{Success: true, Message: "Reset"}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	err := ResetChallengeProgress("pod-evicted")

	require.NoError(t, err)
}

func TestResetChallengeProgress_Failure(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := ChallengeResetResponse{
			Success: false,
			Message: "Reset failed",
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	err := ResetChallengeProgress("pod-evicted")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Reset failed")
}

func TestGetChallenge_Alias(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := ChallengeEntity{
			ID:               456,
			Slug:             "test",
			Title:            "Test",
			Description:      "Test challenge",
			Difficulty:       "easy",
			Theme:            "testing",
			InitialSituation: "Test",
			Objective:        "Test",
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	challenge, err := GetChallenge("test")

	require.NoError(t, err)
	assert.Equal(t, 456, challenge.ID)
}

func TestGetChallengeProgress_Alias(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := ChallengeStatusResponse{Status: "completed"}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	status, err := GetChallengeProgress("test")

	require.NoError(t, err)
	assert.Equal(t, "completed", status.Status)
}

func TestLogin_Success(t *testing.T) {
	setupKeyring(t, "test-token")
	defer cleanupKeyring(t)

	server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/cli/user", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req struct {
			CliVersion string `json:"cliVersion"`
			Os         string `json:"os"`
			Arch       string `json:"arch"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.NotEmpty(t, req.CliVersion)
		assert.NotEmpty(t, req.Os)
		assert.NotEmpty(t, req.Arch)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		firstLogin := true
		response := LoginResponse{
			FirstName:  "Paul",
			LastName:   strPtr("Brissaud"),
			FirstLogin: &firstLogin,
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	result, err := Login()

	require.NoError(t, err)
	assert.Equal(t, "Paul", result.FirstName)
	require.NotNil(t, result.LastName)
	assert.Equal(t, "Brissaud", *result.LastName)
	require.NotNil(t, result.FirstLogin)
	assert.True(t, *result.FirstLogin)
}

func TestLogin_APIError(t *testing.T) {
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

	result, err := Login()

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "API error: Unauthorized")
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
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"firstTime":false}`))
	})
	defer server.Close()
	defer overrideServerURL(t, server.URL)()

	TrackSetup()

	assert.True(t, called, "expected tracking request to be sent")
}

func TestTrackSetup_NoAuth(t *testing.T) {
	keyring.MockInit()
	_ = keyring.Delete(constants.KeyringServiceName, "api_key")

	// Should not panic
	TrackSetup()
}

func TestGetAuthToken_NoKeyring(t *testing.T) {
	keyring.MockInit()
	_ = keyring.Delete(constants.KeyringServiceName, "api_key")

	defer overrideServerURL(t, "http://localhost:9999")()

	_, err := GetProfile()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "please run 'kubeasy login'")
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
