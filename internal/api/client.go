package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/apigen"
	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
)

func timeToStringPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

func timeToString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

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

// GetChallengeBySlug fetches a challenge by its slug via GET /api/cli/challenge/:slug
func GetChallengeBySlug(ctx context.Context, slug string) (*ChallengeEntity, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.CliGetChallengeWithResponse(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}

	if resp.JSON200 == nil {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	if resp.JSON200.Challenge == nil {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}
	c := resp.JSON200.Challenge
	challenge := &ChallengeEntity{
		ID:               c.Id,
		Title:            c.Title,
		Slug:             c.Slug,
		Description:      c.Description,
		Difficulty:       string(c.Difficulty),
		Theme:            c.Theme,
		InitialSituation: c.InitialSituation,
	}
	return challenge, nil
}

// GetChallengeStatus fetches the user's progress status via GET /api/cli/challenge/:slug/status
func GetChallengeStatus(ctx context.Context, slug string) (*ChallengeStatusResponse, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.CliGetChallengeStatusWithResponse(ctx, slug)
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
		StartedAt:   timeToStringPtr(resp.JSON200.StartedAt),
		CompletedAt: timeToStringPtr(resp.JSON200.CompletedAt),
	}
	return status, nil
}

// StartChallengeWithResponse starts a challenge via POST /api/cli/challenge/:slug/start
func StartChallengeWithResponse(ctx context.Context, slug string) (*ChallengeStartResponse, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.CliStartChallengeWithResponse(ctx, slug)
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
		StartedAt: timeToString(resp.JSON200.StartedAt),
		Message:   resp.JSON200.Message,
	}
	return result, nil
}

// SubmitChallenge submits a challenge via POST /api/cli/challenge/:slug/submit.
// It uses raw JSON marshaling so that extra fields (e.g. auditEvents) are forwarded
// without requiring regeneration of the OpenAPI client.
func SubmitChallenge(ctx context.Context, slug string, req ChallengeSubmitRequest) (*ChallengeSubmitResponse, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := client.CliSubmitChallengeLegacyWithBodyWithResponse(ctx, slug, "application/json", bytes.NewReader(body))
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

// ResetChallenge resets the user's progress via POST /api/cli/challenge/:slug/reset
func ResetChallenge(ctx context.Context, slug string) (*ChallengeResetResponse, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.CliResetChallengeWithResponse(ctx, slug)
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

	slugs := make([]string, len(*resp.JSON200))
	for i, t := range *resp.JSON200 {
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

	slugs := make([]string, len(*resp.JSON200))
	for i, t := range *resp.JSON200 {
		slugs[i] = t.Slug
	}
	return slugs, nil
}

// GetDifficulties is no longer available as a dedicated endpoint.
func GetDifficulties(_ context.Context) ([]string, error) {
	return nil, fmt.Errorf("difficulties endpoint removed")
}
