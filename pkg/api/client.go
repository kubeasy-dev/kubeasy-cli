package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/constants"
	"github.com/zalando/go-keyring"
)

// getAuthToken retrieves the API token from the keyring
func getAuthToken() (string, error) {
	token, err := keyring.Get(constants.KeyringServiceName, "api_key")
	if err != nil {
		return "", fmt.Errorf("failed to get API key from keyring: %w. Please run 'kubeasy login'", err)
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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	return resp, nil
}

// GetProfile fetches the current user's profile (main function name for compatibility)
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

// SendSubmit submits a challenge with validation results (backward compatibility wrapper)
// This function wraps SubmitChallenge to maintain compatibility with old code that uses challenge ID
func SendSubmit(challengeSlug string, staticValidation bool, dynamicValidation bool, payload interface{}) error {
	// Create request object
	req := ChallengeSubmitRequest{
		Validated:         staticValidation && dynamicValidation,
		StaticValidation:  &staticValidation,
		DynamicValidation: &dynamicValidation,
		Payload:           payload,
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
