package cmd

import (
	"fmt"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSubmitRunE_InvalidSlug verifies that an invalid slug is rejected before any API call.
func TestSubmitRunE_InvalidSlug(t *testing.T) {
	err := submitCmd.RunE(submitCmd, []string{"INVALID_SLUG"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid challenge slug")
}

// TestSubmitRunE_ProgressNil verifies that a nil progress response returns nil (challenge not started guard).
func TestSubmitRunE_ProgressNil(t *testing.T) {
	origGetChallenge := apiGetChallengeForSubmit
	origGetProgress := apiGetProgressForSubmit
	t.Cleanup(func() {
		apiGetChallengeForSubmit = origGetChallenge
		apiGetProgressForSubmit = origGetProgress
	})

	apiGetChallengeForSubmit = func(slug string) (*api.ChallengeEntity, error) {
		return &api.ChallengeEntity{Title: "Test"}, nil
	}
	apiGetProgressForSubmit = func(slug string) (*api.ChallengeStatusResponse, error) {
		return nil, nil
	}

	err := submitCmd.RunE(submitCmd, []string{"pod-evicted"})
	assert.NoError(t, err)
}

// TestSubmitRunE_AlreadyCompleted verifies that a completed challenge returns nil (already completed guard).
func TestSubmitRunE_AlreadyCompleted(t *testing.T) {
	origGetChallenge := apiGetChallengeForSubmit
	origGetProgress := apiGetProgressForSubmit
	t.Cleanup(func() {
		apiGetChallengeForSubmit = origGetChallenge
		apiGetProgressForSubmit = origGetProgress
	})

	apiGetChallengeForSubmit = func(slug string) (*api.ChallengeEntity, error) {
		return &api.ChallengeEntity{Title: "Test"}, nil
	}
	apiGetProgressForSubmit = func(slug string) (*api.ChallengeStatusResponse, error) {
		return &api.ChallengeStatusResponse{Status: "completed"}, nil
	}

	err := submitCmd.RunE(submitCmd, []string{"pod-evicted"})
	assert.NoError(t, err)
}

// TestSubmitRunE_APIFailure verifies that a GetChallenge API failure returns a non-nil error without panic.
func TestSubmitRunE_APIFailure(t *testing.T) {
	origGetChallenge := apiGetChallengeForSubmit
	t.Cleanup(func() {
		apiGetChallengeForSubmit = origGetChallenge
	})

	apiGetChallengeForSubmit = func(slug string) (*api.ChallengeEntity, error) {
		return nil, fmt.Errorf("network error")
	}

	assert.NotPanics(t, func() {
		err := submitCmd.RunE(submitCmd, []string{"pod-evicted"})
		require.Error(t, err)
	})
}
