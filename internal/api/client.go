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
func GetProfile(ctx context.Context) (*UserProfile, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetUserWithResponse(ctx)
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
func Login(ctx context.Context) (*LoginResponse, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.LoginUserWithResponse(ctx, apigen.LoginUserJSONRequestBody{
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

// GetChallengeBySlug fetches a challenge by its slug
func GetChallengeBySlug(ctx context.Context, slug string) (*ChallengeEntity, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetChallengeWithResponse(ctx, slug)
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
	}
	return challenge, nil
}

// GetChallengeStatus fetches the user's progress status for a challenge
func GetChallengeStatus(ctx context.Context, slug string) (*ChallengeStatusResponse, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetChallengeStatusWithResponse(ctx, slug)
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
func StartChallengeWithResponse(ctx context.Context, slug string) (*ChallengeStartResponse, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.StartChallengeWithResponse(ctx, slug)
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

// SubmitChallenge submits a challenge with validation results
func SubmitChallenge(ctx context.Context, slug string, req ChallengeSubmitRequest) (*ChallengeSubmitResponse, error) {
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

	resp, err := client.SubmitChallengeWithResponse(ctx, slug, apigen.SubmitChallengeJSONRequestBody{
		Results: results,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}

	// The submit endpoint uses distinct HTTP codes:
	//   200 → success ({ success: true, xpAwarded, ... })
	//   422 → validation failed ({ success: false, message, failedObjectives })
	// We parse both as ChallengeSubmitResponse since they share the same
	// discriminated union keyed on the "success" boolean.
	if resp.StatusCode() == http.StatusOK || resp.StatusCode() == http.StatusUnprocessableEntity {
		var result ChallengeSubmitResponse
		if err := json.Unmarshal(resp.Body, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return &result, nil
	}

	return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
}

// ResetChallenge resets the user's progress for a challenge
func ResetChallenge(ctx context.Context, slug string) (*ChallengeResetResponse, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.ResetChallengeWithResponse(ctx, slug)
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
func TrackSetup(ctx context.Context) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		logger.Debug("Failed to create client for tracking: %v", err)
		return
	}

	_, err = client.TrackSetupWithResponse(ctx, apigen.TrackSetupJSONRequestBody{
		CliVersion: constants.Version,
		Os:         runtime.GOOS,
		Arch:       runtime.GOARCH,
	})
	if err != nil {
		logger.Debug("Failed to send tracking event: %v", err)
	}
}

// GetTypes fetches challenge types from the public API.
func GetTypes(ctx context.Context) ([]string, error) {
	client, err := NewPublicClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetTypesWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch challenge types: %w", err)
	}

	if resp.JSON200 == nil {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	slugs := make([]string, len(resp.JSON200.Types))
	for i, t := range resp.JSON200.Types {
		slugs[i] = t.Slug
	}
	return slugs, nil
}

// GetThemes fetches challenge themes from the public API.
func GetThemes(ctx context.Context) ([]string, error) {
	client, err := NewPublicClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetThemesWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch challenge themes: %w", err)
	}

	if resp.JSON200 == nil {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	slugs := make([]string, len(resp.JSON200.Themes))
	for i, t := range resp.JSON200.Themes {
		slugs[i] = t.Slug
	}
	return slugs, nil
}

// GetDifficulties fetches challenge difficulties from the public API.
func GetDifficulties(ctx context.Context) ([]string, error) {
	client, err := NewPublicClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetDifficultiesWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch challenge difficulties: %w", err)
	}

	if resp.JSON200 == nil {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	difficulties := make([]string, len(resp.JSON200.Difficulties))
	for i, d := range resp.JSON200.Difficulties {
		difficulties[i] = string(d)
	}
	return difficulties, nil
}
