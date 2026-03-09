package cmd

import (
	"fmt"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStartRunE_InvalidSlug verifies that an invalid slug is rejected before any API call.
func TestStartRunE_InvalidSlug(t *testing.T) {
	err := startChallengeCmd.RunE(startChallengeCmd, []string{"INVALID_SLUG"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid challenge slug")
}

// TestStartRunE_AlreadyInProgress verifies that a challenge already in progress returns nil (no error).
func TestStartRunE_AlreadyInProgress(t *testing.T) {
	origGetChallenge := apiGetChallenge
	origGetProgress := apiGetChallengeProgress
	t.Cleanup(func() {
		apiGetChallenge = origGetChallenge
		apiGetChallengeProgress = origGetProgress
	})

	apiGetChallenge = func(slug string) (*api.ChallengeEntity, error) {
		return &api.ChallengeEntity{Title: "Test"}, nil
	}
	apiGetChallengeProgress = func(slug string) (*api.ChallengeStatusResponse, error) {
		return &api.ChallengeStatusResponse{Status: "in_progress"}, nil
	}

	err := startChallengeCmd.RunE(startChallengeCmd, []string{"pod-evicted"})
	assert.NoError(t, err)
}

// TestStartRunE_AlreadyCompleted verifies that a completed challenge returns nil (no error).
func TestStartRunE_AlreadyCompleted(t *testing.T) {
	origGetChallenge := apiGetChallenge
	origGetProgress := apiGetChallengeProgress
	t.Cleanup(func() {
		apiGetChallenge = origGetChallenge
		apiGetChallengeProgress = origGetProgress
	})

	apiGetChallenge = func(slug string) (*api.ChallengeEntity, error) {
		return &api.ChallengeEntity{Title: "Test"}, nil
	}
	apiGetChallengeProgress = func(slug string) (*api.ChallengeStatusResponse, error) {
		return &api.ChallengeStatusResponse{Status: "completed"}, nil
	}

	err := startChallengeCmd.RunE(startChallengeCmd, []string{"pod-evicted"})
	assert.NoError(t, err)
}

// TestStartRunE_APIFailure verifies that a GetChallenge API failure returns a non-nil error without panic.
func TestStartRunE_APIFailure(t *testing.T) {
	origGetChallenge := apiGetChallenge
	t.Cleanup(func() {
		apiGetChallenge = origGetChallenge
	})

	apiGetChallenge = func(slug string) (*api.ChallengeEntity, error) {
		return nil, fmt.Errorf("network error")
	}

	assert.NotPanics(t, func() {
		err := startChallengeCmd.RunE(startChallengeCmd, []string{"pod-evicted"})
		require.Error(t, err)
	})
}
