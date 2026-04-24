package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
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

// GetProfile fetches the current user's profile via GET /api/user/me
func GetProfile(ctx context.Context) (*UserProfile, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetUserMeWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	user := *resp.JSON200
	firstName := user.Name
	var lastName *string
	if parts := strings.SplitN(user.Name, " ", 2); len(parts) > 1 {
		firstName = parts[0]
		lastName = &parts[1]
	}

	profile := &UserProfile{
		FirstName: firstName,
		LastName:  lastName,
	}
	return profile, nil
}

// Login tracks CLI login and returns the user profile
func Login(ctx context.Context) (*LoginResponse, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	// 1. Track login
	trackResp, err := client.TrackCliLoginWithResponse(ctx, apigen.TrackCliLoginJSONRequestBody{
		CliVersion: constants.Version,
		Os:         runtime.GOOS,
		Arch:       runtime.GOARCH,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to track login: %w", err)
	}
	if trackResp.StatusCode() != http.StatusOK {
		return nil, parseErrorResponse(trackResp.HTTPResponse, trackResp.Body)
	}

	// 2. Get profile
	profile, err := GetProfile(ctx)
	if err != nil {
		return nil, err
	}

	result := &LoginResponse{
		FirstName:  profile.FirstName,
		LastName:   profile.LastName,
		FirstLogin: &trackResp.JSON200.FirstLogin,
	}
	return result, nil
}

// GetChallengeBySlug fetches a challenge by its slug from the API
func GetChallengeBySlug(ctx context.Context, slug string) (*ChallengeEntity, error) {
	client, err := NewPublicClient()
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

	if resp.JSON200 == nil || resp.JSON200.Challenge == nil {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	c := resp.JSON200.Challenge
	challenge := &ChallengeEntity{
		Title:            c.Title,
		Slug:             c.Slug,
		Description:      c.Description,
		Difficulty:       string(c.Difficulty),
		Theme:            c.Theme,
		InitialSituation: c.InitialSituation,
	}
	return challenge, nil
}

// GetChallengeStatus fetches the user's progress status via GET /api/progress/:slug
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
		StartedAt:   timeToStringPtr(resp.JSON200.StartedAt),
		CompletedAt: timeToStringPtr(resp.JSON200.CompletedAt),
	}
	return status, nil
}

// StartChallengeWithResponse starts a challenge via POST /api/progress/:slug/start
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
		StartedAt: timeToString(resp.JSON200.StartedAt),
		Message:   resp.JSON200.Message,
	}
	return result, nil
}

// SubmitChallenge submits a challenge via POST /api/challenges/:slug/submit.
func SubmitChallenge(ctx context.Context, slug string, req ChallengeSubmitRequest) (*ChallengeSubmitResponse, error) {
	client, err := NewAuthenticatedClient()
	if err != nil {
		return nil, err
	}

	// Map CLI types to generated types
	results := make([]struct {
		Message      *string `json:"message,omitempty"`
		ObjectiveKey string  `json:"objectiveKey"`
		Passed       bool    `json:"passed"`
	}, len(req.Results))
	for i, r := range req.Results {
		r := r
		results[i] = struct {
			Message      *string `json:"message,omitempty"`
			ObjectiveKey string  `json:"objectiveKey"`
			Passed       bool    `json:"passed"`
		}{
			ObjectiveKey: r.ObjectiveKey,
			Passed:       r.Passed,
			Message:      r.Message,
		}
	}

	auditEvents := make([]struct {
		Name         *string   `json:"name,omitempty"`
		Namespace    *string   `json:"namespace,omitempty"`
		Resource     string    `json:"resource"`
		ResponseCode *int      `json:"responseCode,omitempty"`
		Subresource  *string   `json:"subresource,omitempty"`
		Timestamp    time.Time `json:"timestamp"`
		UserAgent    *string   `json:"userAgent,omitempty"`
		Verb         string    `json:"verb"`
	}, len(req.AuditEvents))
	for i, e := range req.AuditEvents {
		e := e
		var subresource, name, namespace, userAgent *string
		if e.Subresource != "" {
			subresource = &e.Subresource
		}
		if e.Name != "" {
			name = &e.Name
		}
		if e.Namespace != "" {
			namespace = &e.Namespace
		}
		if e.UserAgent != "" {
			userAgent = &e.UserAgent
		}
		rc := e.ResponseCode

		auditEvents[i] = struct {
			Name         *string   `json:"name,omitempty"`
			Namespace    *string   `json:"namespace,omitempty"`
			Resource     string    `json:"resource"`
			ResponseCode *int      `json:"responseCode,omitempty"`
			Subresource  *string   `json:"subresource,omitempty"`
			Timestamp    time.Time `json:"timestamp"`
			UserAgent    *string   `json:"userAgent,omitempty"`
			Verb         string    `json:"verb"`
		}{
			Timestamp:    e.Timestamp,
			Verb:         e.Verb,
			Resource:     e.Resource,
			Subresource:  subresource,
			Name:         name,
			Namespace:    namespace,
			UserAgent:    userAgent,
			ResponseCode: &rc,
		}
	}

	resp, err := client.SubmitChallengeWithResponse(ctx, slug, apigen.SubmitChallengeJSONRequestBody{
		Results:     results,
		AuditEvents: &auditEvents,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("challenge '%s' not found", slug)
	}

	if resp.StatusCode() == http.StatusOK || resp.StatusCode() == http.StatusUnprocessableEntity {
		var result ChallengeSubmitResponse
		if err := json.Unmarshal(resp.Body, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return &result, nil
	}

	return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
}

// ResetChallenge resets the user's progress via POST /api/progress/:slug/reset
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

// GetTypes fetches challenge types from the API.
func GetTypes(ctx context.Context) ([]string, error) {
	client, err := NewPublicClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetChallengeMetaWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API meta: %w", err)
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

// GetThemes fetches challenge themes from the API.
func GetThemes(ctx context.Context) ([]string, error) {
	client, err := NewPublicClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetChallengeMetaWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API meta: %w", err)
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

// GetDifficulties fetches challenge difficulties from the API.
func GetDifficulties(ctx context.Context) ([]string, error) {
	client, err := NewPublicClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetChallengeMetaWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API meta: %w", err)
	}

	if resp.JSON200 == nil {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	return resp.JSON200.Difficulties, nil
}
