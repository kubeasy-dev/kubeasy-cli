package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCleanRunE_InvalidSlug verifies that an invalid slug is rejected before any cluster call.
func TestCleanRunE_InvalidSlug(t *testing.T) {
	err := cleanChallengeCmd.RunE(cleanChallengeCmd, []string{"INVALID_SLUG"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid challenge slug")
}
