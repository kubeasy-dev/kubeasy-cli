package cmd

import (
	"fmt"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResetRunE_InvalidSlug verifies that an invalid slug is rejected before any API call.
func TestResetRunE_InvalidSlug(t *testing.T) {
	err := resetChallengeCmd.RunE(resetChallengeCmd, []string{"INVALID_SLUG"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid challenge slug")
}

// TestResetRunE_APIFailure verifies that a getChallenge API failure returns a non-nil error without panic.
func TestResetRunE_APIFailure(t *testing.T) {
	orig := getChallengeFn
	t.Cleanup(func() {
		getChallengeFn = orig
	})

	getChallengeFn = func(slug string) (*api.ChallengeEntity, error) {
		return nil, fmt.Errorf("challenge not found")
	}

	assert.NotPanics(t, func() {
		err := resetChallengeCmd.RunE(resetChallengeCmd, []string{"pod-evicted"})
		require.Error(t, err)
	})
}
