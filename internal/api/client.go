package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"

	"github.com/kubeasy-dev/kubeasy-cli/internal/apigen"
	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
)

// parseErrorResponse extracts an error message from a generated response.
func parseErrorResponse(resp *http.Response, body []byte) error {
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}
	return fmt.Errorf("API error: %s", errResp.Error)
}

// GetProfile fetches the current user's profile via GET /api/cli/user
func GetProfile() (*UserProfile, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetUserWithResponse(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.JSON200 == nil {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	profile := &UserProfile{
		FirstName: resp.JSON200.FirstName,
		LastName:  resp.JSON200.LastName,
	}
	return profile, nil
}

// Login sends a POST /api/cli/user with CLI metadata, combining profile fetch
// and login tracking in a single call.
func Login() (*LoginResponse, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.LoginUserWithResponse(context.Background(), apigen.LoginUserJSONRequestBody{
		CliVersion: constants.Version,
		Os:         runtime.GOOS,
		Arch:       runtime.GOARCH,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.JSON200 == nil {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	result := &LoginResponse{
		FirstName:  resp.JSON200.FirstName,
		LastName:   resp.JSON200.LastName,
		FirstLogin: &resp.JSON200.FirstLogin,
	}
	return result, nil
}

// GetUserProfile is an alias for GetProfile for consistency with type names
func GetUserProfile() (*UserProfile, error) {
	return GetProfile()
}

// GetChallengeBySlug fetches a challenge by its slug
func GetChallengeBySlug(slug string) (*ChallengeEntity, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetChallengeWithResponse(context.Background(), slug)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}

	if resp.JSON200 == nil {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	challenge := &ChallengeEntity{
		ID:               resp.JSON200.Id,
		Title:            resp.JSON200.Title,
		Slug:             resp.JSON200.Slug,
		Description:      resp.JSON200.Description,
		Difficulty:       string(resp.JSON200.Difficulty),
		Theme:            resp.JSON200.Theme,
		InitialSituation: resp.JSON200.InitialSituation,
		Objective:        resp.JSON200.Objective,
	}
	return challenge, nil
}

// GetChallengeStatus fetches the user's progress status for a challenge
func GetChallengeStatus(slug string) (*ChallengeStatusResponse, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetChallengeStatusWithResponse(context.Background(), slug)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}

	if resp.JSON200 == nil {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	status := &ChallengeStatusResponse{
		Status:      string(resp.JSON200.Status),
		StartedAt:   resp.JSON200.StartedAt,
		CompletedAt: resp.JSON200.CompletedAt,
	}
	return status, nil
}

// StartChallengeWithResponse starts a challenge for the user and returns the full response
func StartChallengeWithResponse(slug string) (*ChallengeStartResponse, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.StartChallengeWithResponse(context.Background(), slug)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}

	if resp.JSON200 == nil {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	result := &ChallengeStartResponse{
		Status:    string(resp.JSON200.Status),
		StartedAt: resp.JSON200.StartedAt,
		Message:   resp.JSON200.Message,
	}
	return result, nil
}

// StartChallenge starts a challenge for the user (backward compatibility wrapper)
func StartChallenge(slug string) error {
	_, err := StartChallengeWithResponse(slug)
	return err
}

// SubmitChallenge submits a challenge with validation results
func SubmitChallenge(slug string, req ChallengeSubmitRequest) (*ChallengeSubmitResponse, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	// Convert ObjectiveResult to the generated request body type
	results := make([]struct {
		Message      *string `json:"message,omitempty"`
		ObjectiveKey string  `json:"objectiveKey"`
		Passed       bool    `json:"passed"`
	}, len(req.Results))
	for i, r := range req.Results {
		results[i].ObjectiveKey = r.ObjectiveKey
		results[i].Passed = r.Passed
		results[i].Message = r.Message
	}

	resp, err := client.SubmitChallengeWithResponse(context.Background(), slug, apigen.SubmitChallengeJSONRequestBody{
		Results: results,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}

	// The submit endpoint returns 200 for both success and failure.
	// The response is a union type discriminated by the "success" field.
	// We always parse the raw body since the generated client's JSON200 type
	// only matches the success variant.
	if resp.StatusCode() == http.StatusOK || resp.StatusCode() == http.StatusBadRequest {
		var result ChallengeSubmitResponse
		if err := json.Unmarshal(resp.Body, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return &result, nil
	}

	return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
}

// GetChallenge is a wrapper for GetChallengeBySlug for backward compatibility
func GetChallenge(slug string) (*ChallengeEntity, error) {
	return GetChallengeBySlug(slug)
}

// GetChallengeProgress fetches the challenge status (backward compatibility)
func GetChallengeProgress(slug string) (*ChallengeStatusResponse, error) {
	return GetChallengeStatus(slug)
}

// SendSubmit submits a challenge with raw validation results from CRDs
func SendSubmit(challengeSlug string, results []ObjectiveResult) error {
	req := ChallengeSubmitRequest{
		Results: results,
	}

	result, err := SubmitChallenge(challengeSlug, req)
	if err != nil {
		return err
	}

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
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.ResetChallengeWithResponse(context.Background(), slug)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}

	if resp.JSON200 == nil {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	return &ChallengeResetResponse{
		Success: resp.JSON200.Success,
		Message: resp.JSON200.Message,
	}, nil
}

// TrackSetup sends a setup tracking event using the generated client.
func TrackSetup() {
	client, err := NewAuthenticatedClient()
	if err != nil {
		logger.Debug("Failed to create client for tracking: %v", err)
		return
	}

	_, err = client.TrackSetupWithResponse(context.Background(), apigen.TrackSetupJSONRequestBody{
		CliVersion: constants.Version,
		Os:         runtime.GOOS,
		Arch:       runtime.GOARCH,
	})
	if err != nil {
		logger.Debug("Failed to send tracking event: %v", err)
	}
}

// ResetChallengeProgress is a wrapper for ResetChallenge for backward compatibility
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
