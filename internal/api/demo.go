package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/demo"
)

// DemoSessionResponse from GET /api/demo/session
type DemoSessionResponse struct {
	Valid       bool  `json:"valid"`
	CreatedAt   int64 `json:"createdAt,omitempty"`
	CompletedAt int64 `json:"completedAt,omitempty"`
}

// DemoSubmitRequest to POST /api/demo/submit
type DemoSubmitRequest struct {
	Token   string                 `json:"token"`
	Results []demo.ObjectiveResult `json:"results"`
}

// DemoSubmitResponse from POST /api/demo/submit
type DemoSubmitResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// VerifyDemoToken checks if a demo token is valid
func VerifyDemoToken(token string) (*DemoSessionResponse, error) {
	url := fmt.Sprintf("%s/api/demo/session?token=%s", constants.WebsiteURL, token)

	resp, err := http.Get(url) //nolint:gosec // URL is constructed from trusted constant
	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("demo mode is not available on this server")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to verify token: HTTP %d", resp.StatusCode)
	}

	var result DemoSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// SendDemoStart notifies the backend that demo has started
// This triggers a realtime event for the frontend to update
func SendDemoStart(token string) error {
	return sendDemoEvent(token, "start")
}

// SendDemoPodCreated notifies the backend that the demo pod was created
// This triggers a realtime event for the frontend to update
func SendDemoPodCreated(token string) error {
	return sendDemoEvent(token, "pod-created")
}

// sendDemoEvent sends a demo lifecycle event to the backend
func sendDemoEvent(token string, event string) error {
	url := fmt.Sprintf("%s/api/demo/%s", constants.WebsiteURL, event)

	payload := map[string]string{"token": token}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(body)) //nolint:gosec // URL is constructed from trusted constant
	if err != nil {
		return fmt.Errorf("failed to notify %s: %w", event, err)
	}
	defer resp.Body.Close()

	// Don't fail if the notification doesn't work - it's not critical
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notification failed: HTTP %d", resp.StatusCode)
	}

	return nil
}

// SendDemoSubmit submits demo validation results
func SendDemoSubmit(token string, results []demo.ObjectiveResult) (*DemoSubmitResponse, error) {
	url := fmt.Sprintf("%s/api/demo/submit", constants.WebsiteURL)

	payload := DemoSubmitRequest{
		Token:   token,
		Results: results,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(body)) //nolint:gosec // URL is constructed from trusted constant
	if err != nil {
		return nil, fmt.Errorf("failed to submit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid or expired demo token")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("submission failed: HTTP %d", resp.StatusCode)
	}

	var result DemoSubmitResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}
