package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/keystore"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
)

// getAuthToken retrieves the API token from available storage
func getAuthToken() (string, error) {
	token, err := keystore.Get()
	if err != nil {
		return "", err
	}
	return token, nil
}

// makeAuthenticatedRequest makes an HTTP request with authentication
func makeAuthenticatedRequest(method, path string, body interface{}) (*http.Response, error) {
	token, err := getAuthToken()
	if err != nil {
		return nil, err
	}

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := constants.RestAPIUrl + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	return resp, nil
}

// GetProfile fetches the current user's profile via GET /user
func GetProfile() (*UserProfile, error) {
	resp, err := makeAuthenticatedRequest("GET", "/user", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("API error: %s", errResp.Error)
	}

	var user UserProfile
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &user, nil
}

// Login sends a POST /user with CLI metadata, combining profile fetch and login tracking
// in a single call. Returns the user profile along with a firstLogin indicator.
func Login() (*LoginResponse, error) {
	body := TrackRequest{
		CLIVersion: constants.Version,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
	}

	resp, err := makeAuthenticatedRequest("POST", "/user", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("API error: %s", errResp.Error)
	}

	var result LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetUserProfile is an alias for GetProfile for consistency with type names
func GetUserProfile() (*UserProfile, error) {
	return GetProfile()
}

// GetChallengeBySlug fetches a challenge by its slug
func GetChallengeBySlug(slug string) (*ChallengeEntity, error) {
	resp, err := makeAuthenticatedRequest("GET", "/challenge/"+slug, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("API error: %s", errResp.Error)
	}

	var challenge ChallengeEntity
	if err := json.NewDecoder(resp.Body).Decode(&challenge); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &challenge, nil
}

// GetChallengeStatus fetches the user's progress status for a challenge
func GetChallengeStatus(slug string) (*ChallengeStatusResponse, error) {
	resp, err := makeAuthenticatedRequest("GET", "/challenge/"+slug+"/status", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("API error: %s", errResp.Error)
	}

	var status ChallengeStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &status, nil
}

// StartChallengeWithResponse starts a challenge for the user and returns the full response
func StartChallengeWithResponse(slug string) (*ChallengeStartResponse, error) {
	resp, err := makeAuthenticatedRequest("POST", "/challenge/"+slug+"/start", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("API error: %s", errResp.Error)
	}

	var result ChallengeStartResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// StartChallenge starts a challenge for the user (backward compatibility wrapper)
// Returns only an error for compatibility with existing code
func StartChallenge(slug string) error {
	_, err := StartChallengeWithResponse(slug)
	return err
}

// SubmitChallenge submits a challenge with validation results
func SubmitChallenge(slug string, req ChallengeSubmitRequest) (*ChallengeSubmitResponse, error) {
	resp, err := makeAuthenticatedRequest("POST", "/challenge/"+slug+"/submit", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("API error: %s", errResp.Error)
	}

	var result ChallengeSubmitResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetChallenge is a wrapper for GetChallengeBySlug for backward compatibility
func GetChallenge(slug string) (*ChallengeEntity, error) {
	return GetChallengeBySlug(slug)
}

// GetChallengeProgress fetches the challenge status and returns it in the old format
// This function is provided for backward compatibility with existing code
func GetChallengeProgress(slug string) (*ChallengeStatusResponse, error) {
	return GetChallengeStatus(slug)
}

// SendSubmit submits a challenge with raw validation results from CRDs
func SendSubmit(challengeSlug string, results []ObjectiveResult) error {
	req := ChallengeSubmitRequest{
		Results: results,
	}

	// Submit the challenge
	result, err := SubmitChallenge(challengeSlug, req)
	if err != nil {
		return err
	}

	// Check if submission was successful
	if !result.Success {
		if result.Message != nil {
			return fmt.Errorf("submission failed: %s", *result.Message)
		}
		return fmt.Errorf("submission failed")
	}

	return nil
}

// ResetChallenge resets the user's progress for a challenge
func ResetChallenge(slug string) (*ChallengeResetResponse, error) {
	resp, err := makeAuthenticatedRequest("POST", "/challenge/"+slug+"/reset", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("API error: %s", errResp.Error)
	}

	var result ChallengeResetResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// TrackEvent sends anonymous usage telemetry to help improve Kubeasy.
// It reports CLI version, OS, and architecture to the given tracking endpoint.
// The call runs in a background goroutine and never blocks the CLI.
// Errors are silently logged at debug level.
func TrackEvent(path string) {
	go sendTrackEvent(path)
}

// sendTrackEvent performs the actual tracking HTTP request.
// Separated from TrackEvent for testability.
func sendTrackEvent(path string) {
	token, err := getAuthToken()
	if err != nil {
		logger.Debug("Failed to get auth token for tracking: %v", err)
		return
	}

	body := TrackRequest{
		CLIVersion: constants.Version,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
	}
	jsonData, err := json.Marshal(body)
	if err != nil {
		logger.Debug("Failed to marshal tracking request: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", constants.RestAPIUrl+path, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Debug("Failed to create tracking request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Debug("Failed to send tracking event to %s: %v", path, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Debug("Tracking event to %s returned status %d", path, resp.StatusCode)
	}
}

// ResetChallengeProgress is a wrapper for ResetChallenge for backward compatibility
// The old API used challenge ID, but the new API uses slug, so we accept slug here
func ResetChallengeProgress(slugOrID string) error {
	result, err := ResetChallenge(slugOrID)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("reset failed: %s", result.Message)
	}

	return nil
}
