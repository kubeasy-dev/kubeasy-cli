package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/api"
	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTempChallengeYaml creates a temporary challenge.yaml for the given slug under a temp
// directory, sets KUBEASY_LOCAL_CHALLENGES_DIR so the loader finds it, and returns a cleanup func.
func writeTempChallengeYaml(t *testing.T, slug, content string) func() {
	t.Helper()
	dir := t.TempDir()
	challengeDir := filepath.Join(dir, slug)
	require.NoError(t, os.MkdirAll(challengeDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(challengeDir, "challenge.yaml"), []byte(content), 0o600))
	t.Setenv("KUBEASY_LOCAL_CHALLENGES_DIR", dir)
	return func() {}
}

// TestCheckMinRequiredVersion covers the four branches of the version gate.
func TestCheckMinRequiredVersion(t *testing.T) {
	origVersion := constants.Version
	t.Cleanup(func() { constants.Version = origVersion })

	tests := []struct {
		name        string
		cliVersion  string
		yamlContent string
		wantErr     bool
		errContains string
	}{
		{
			name:       "no minRequiredVersion field — always passes",
			cliVersion: "2.0.0",
			yamlContent: `title: "Test"
type: fix
theme: networking
difficulty: easy
estimatedTime: 30
initialSituation: "test"
description: "test"
objective: "test"
objectives: []
`,
			wantErr: false,
		},
		{
			name:       "pre-release CLI build — skips check",
			cliVersion: "dev",
			yamlContent: `title: "Test"
minRequiredVersion: "99.0.0"
objectives: []
`,
			wantErr: false,
		},
		{
			name:       "CLI version >= required — passes",
			cliVersion: "2.1.0",
			yamlContent: `title: "Test"
minRequiredVersion: "2.0.0"
objectives: []
`,
			wantErr: false,
		},
		{
			name:       "CLI version == required — passes",
			cliVersion: "2.0.0",
			yamlContent: `title: "Test"
minRequiredVersion: "2.0.0"
objectives: []
`,
			wantErr: false,
		},
		{
			name:       "CLI version < required — blocked",
			cliVersion: "1.9.0",
			yamlContent: `title: "Test"
minRequiredVersion: "2.0.0"
objectives: []
`,
			wantErr:     true,
			errContains: "requires kubeasy-cli >= 2.0.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			constants.Version = tc.cliVersion
			writeTempChallengeYaml(t, "test-challenge", tc.yamlContent)

			err := checkMinRequiredVersion("test-challenge")
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

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

	apiGetChallenge = func(ctx context.Context, slug string) (*api.ChallengeEntity, error) {
		return &api.ChallengeEntity{Title: "Test"}, nil
	}
	apiGetChallengeProgress = func(ctx context.Context, slug string) (*api.ChallengeStatusResponse, error) {
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

	apiGetChallenge = func(ctx context.Context, slug string) (*api.ChallengeEntity, error) {
		return &api.ChallengeEntity{Title: "Test"}, nil
	}
	apiGetChallengeProgress = func(ctx context.Context, slug string) (*api.ChallengeStatusResponse, error) {
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

	apiGetChallenge = func(ctx context.Context, slug string) (*api.ChallengeEntity, error) {
		return nil, fmt.Errorf("network error")
	}

	assert.NotPanics(t, func() {
		err := startChallengeCmd.RunE(startChallengeCmd, []string{"pod-evicted"})
		require.Error(t, err)
	})
}
