package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSendSubmit_Logic tests the SendSubmit wrapper function logic
func TestSendSubmit_RequestConstruction(t *testing.T) {
	t.Run("constructs correct request structure", func(t *testing.T) {
		results := []ObjectiveResult{
			{ObjectiveKey: "obj1", Passed: true, Message: strPtr("success")},
			{ObjectiveKey: "obj2", Passed: false, Message: strPtr("failed")},
		}

		req := ChallengeSubmitRequest{
			Results: results,
		}

		assert.Len(t, req.Results, 2)
		assert.Equal(t, "obj1", req.Results[0].ObjectiveKey)
		assert.True(t, req.Results[0].Passed)
		assert.Equal(t, "obj2", req.Results[1].ObjectiveKey)
		assert.False(t, req.Results[1].Passed)
	})

	t.Run("handles empty results", func(t *testing.T) {
		req := ChallengeSubmitRequest{
			Results: []ObjectiveResult{},
		}

		assert.Len(t, req.Results, 0)
	})

	t.Run("handles nil message", func(t *testing.T) {
		result := ObjectiveResult{
			ObjectiveKey: "test",
			Passed:       true,
			Message:      nil,
		}

		assert.Equal(t, "test", result.ObjectiveKey)
		assert.True(t, result.Passed)
		assert.Nil(t, result.Message)
	})
}

// TestRequestMarshaling tests JSON marshaling of request types
func TestRequestMarshaling(t *testing.T) {
	t.Run("ChallengeSubmitRequest marshaling", func(t *testing.T) {
		req := ChallengeSubmitRequest{
			Results: []ObjectiveResult{
				{ObjectiveKey: "test", Passed: true, Message: strPtr("success")},
			},
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)
		assert.Contains(t, string(data), "objectiveKey")
		assert.Contains(t, string(data), "passed")
		assert.Contains(t, string(data), "success")

		// Unmarshal back
		var decoded ChallengeSubmitRequest
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Len(t, decoded.Results, 1)
		assert.Equal(t, "test", decoded.Results[0].ObjectiveKey)
		assert.True(t, decoded.Results[0].Passed)
		require.NotNil(t, decoded.Results[0].Message)
		assert.Equal(t, "success", *decoded.Results[0].Message)
	})

	t.Run("ObjectiveResult with nil message", func(t *testing.T) {
		result := ObjectiveResult{
			ObjectiveKey: "test",
			Passed:       false,
			Message:      nil,
		}

		data, err := json.Marshal(result)
		require.NoError(t, err)

		// message field should be omitted when nil
		assert.NotContains(t, string(data), "message")
	})
}

// TestResponseUnmarshaling tests JSON unmarshaling of response types
func TestResponseUnmarshaling(t *testing.T) {
	t.Run("ChallengeSubmitResponse success", func(t *testing.T) {
		jsonData := `{
			"success": true,
			"xpAwarded": 100,
			"totalXp": 500,
			"rank": "Novice",
			"rankUp": true,
			"firstChallenge": false
		}`

		var resp ChallengeSubmitResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.True(t, resp.Success)
		require.NotNil(t, resp.XpAwarded)
		assert.Equal(t, 100, *resp.XpAwarded)
		require.NotNil(t, resp.TotalXp)
		assert.Equal(t, 500, *resp.TotalXp)
		require.NotNil(t, resp.Rank)
		assert.Equal(t, "Novice", *resp.Rank)
		require.NotNil(t, resp.RankUp)
		assert.True(t, *resp.RankUp)
	})

	t.Run("ChallengeSubmitResponse failure", func(t *testing.T) {
		jsonData := `{
			"success": false,
			"message": "Some objectives failed"
		}`

		var resp ChallengeSubmitResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.False(t, resp.Success)
		require.NotNil(t, resp.Message)
		assert.Equal(t, "Some objectives failed", *resp.Message)
		assert.Nil(t, resp.XpAwarded)
		assert.Nil(t, resp.TotalXp)
	})

	t.Run("ChallengeStatusResponse in progress", func(t *testing.T) {
		jsonData := `{
			"status": "in_progress",
			"startedAt": "2025-01-01T00:00:00Z"
		}`

		var resp ChallengeStatusResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.Equal(t, "in_progress", resp.Status)
		require.NotNil(t, resp.StartedAt)
		assert.Equal(t, "2025-01-01T00:00:00Z", *resp.StartedAt)
		assert.Nil(t, resp.CompletedAt)
	})

	t.Run("ChallengeStatusResponse completed", func(t *testing.T) {
		jsonData := `{
			"status": "completed",
			"startedAt": "2025-01-01T00:00:00Z",
			"completedAt": "2025-01-02T00:00:00Z"
		}`

		var resp ChallengeStatusResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.Equal(t, "completed", resp.Status)
		require.NotNil(t, resp.StartedAt)
		assert.Equal(t, "2025-01-01T00:00:00Z", *resp.StartedAt)
		require.NotNil(t, resp.CompletedAt)
		assert.Equal(t, "2025-01-02T00:00:00Z", *resp.CompletedAt)
	})

	t.Run("ChallengeEntity complete structure", func(t *testing.T) {
		jsonData := `{
			"id": 42,
			"title": "Pod Evicted",
			"slug": "pod-evicted",
			"description": "Fix the evicted pod",
			"difficulty": "easy",
			"theme": "resources-scaling",
			"initial_situation": "Pod is evicted",
			"objective": "Make it run"
		}`

		var challenge ChallengeEntity
		err := json.Unmarshal([]byte(jsonData), &challenge)
		require.NoError(t, err)

		assert.Equal(t, 42, challenge.ID)
		assert.Equal(t, "Pod Evicted", challenge.Title)
		assert.Equal(t, "pod-evicted", challenge.Slug)
		assert.Equal(t, "Fix the evicted pod", challenge.Description)
		assert.Equal(t, "easy", challenge.Difficulty)
		assert.Equal(t, "resources-scaling", challenge.Theme)
		assert.Equal(t, "Pod is evicted", challenge.InitialSituation)
		assert.Equal(t, "Make it run", challenge.Objective)
	})

	t.Run("UserProfile with optional lastName", func(t *testing.T) {
		jsonData := `{
			"firstName": "John",
			"lastName": "Doe"
		}`

		var user UserProfile
		err := json.Unmarshal([]byte(jsonData), &user)
		require.NoError(t, err)

		assert.Equal(t, "John", user.FirstName)
		require.NotNil(t, user.LastName)
		assert.Equal(t, "Doe", *user.LastName)
	})

	t.Run("UserProfile without lastName", func(t *testing.T) {
		jsonData := `{
			"firstName": "Jane"
		}`

		var user UserProfile
		err := json.Unmarshal([]byte(jsonData), &user)
		require.NoError(t, err)

		assert.Equal(t, "Jane", user.FirstName)
		assert.Nil(t, user.LastName)
	})

	t.Run("ErrorResponse structure", func(t *testing.T) {
		jsonData := `{
			"error": "Not found",
			"details": "Challenge with slug 'invalid' does not exist"
		}`

		var errResp ErrorResponse
		err := json.Unmarshal([]byte(jsonData), &errResp)
		require.NoError(t, err)

		assert.Equal(t, "Not found", errResp.Error)
		require.NotNil(t, errResp.Details)
		assert.Contains(t, *errResp.Details, "Challenge with slug")
	})

	t.Run("ChallengeResetResponse success", func(t *testing.T) {
		jsonData := `{
			"success": true,
			"message": "Challenge reset successfully"
		}`

		var resp ChallengeResetResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.True(t, resp.Success)
		assert.Equal(t, "Challenge reset successfully", resp.Message)
	})

	t.Run("ChallengeStartResponse", func(t *testing.T) {
		jsonData := `{
			"status": "in_progress",
			"startedAt": "2025-01-01T00:00:00Z",
			"message": "Challenge started successfully"
		}`

		var resp ChallengeStartResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.Equal(t, "in_progress", resp.Status)
		assert.Equal(t, "2025-01-01T00:00:00Z", resp.StartedAt)
		require.NotNil(t, resp.Message)
		assert.Equal(t, "Challenge started successfully", *resp.Message)
	})

	t.Run("LoginResponse with firstLogin", func(t *testing.T) {
		jsonData := `{
			"firstName": "Paul",
			"lastName": "Brissaud",
			"firstLogin": true
		}`

		var resp LoginResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.Equal(t, "Paul", resp.FirstName)
		require.NotNil(t, resp.LastName)
		assert.Equal(t, "Brissaud", *resp.LastName)
		require.NotNil(t, resp.FirstLogin)
		assert.True(t, *resp.FirstLogin)
	})

	t.Run("LoginResponse without firstLogin", func(t *testing.T) {
		jsonData := `{
			"firstName": "Jane"
		}`

		var resp LoginResponse
		err := json.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		assert.Equal(t, "Jane", resp.FirstName)
		assert.Nil(t, resp.LastName)
		assert.Nil(t, resp.FirstLogin)
	})

}

// TestObjectiveResultVariations tests different ObjectiveResult scenarios
func TestObjectiveResultVariations(t *testing.T) {
	tests := []struct {
		name     string
		result   ObjectiveResult
		wantJSON string
	}{
		{
			name: "passed with message",
			result: ObjectiveResult{
				ObjectiveKey: "pod-ready",
				Passed:       true,
				Message:      strPtr("Pod is running"),
			},
			wantJSON: `{"objectiveKey":"pod-ready","passed":true,"message":"Pod is running"}`,
		},
		{
			name: "failed without message",
			result: ObjectiveResult{
				ObjectiveKey: "deployment-ready",
				Passed:       false,
				Message:      nil,
			},
			wantJSON: `{"objectiveKey":"deployment-ready","passed":false}`,
		},
		{
			name: "passed without message",
			result: ObjectiveResult{
				ObjectiveKey: "service-available",
				Passed:       true,
				Message:      nil,
			},
			wantJSON: `{"objectiveKey":"service-available","passed":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.result)
			require.NoError(t, err)
			assert.JSONEq(t, tt.wantJSON, string(data))

			// Test unmarshaling back
			var decoded ObjectiveResult
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, tt.result.ObjectiveKey, decoded.ObjectiveKey)
			assert.Equal(t, tt.result.Passed, decoded.Passed)
			if tt.result.Message != nil {
				require.NotNil(t, decoded.Message)
				assert.Equal(t, *tt.result.Message, *decoded.Message)
			} else {
				assert.Nil(t, decoded.Message)
			}
		})
	}
}

// strPtr is an alias for strPtr defined in client_http_test.go
// Helper functions are defined there to avoid duplication.
