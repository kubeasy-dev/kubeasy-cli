// Code generated from website/types/cli-api.ts
// Manually updated: 2025-11-27

package api

// UserResponse represents the response from GET /api/cli/user
type UserResponse struct {
	FirstName string  `json:"firstName"`
	LastName  *string `json:"lastName,omitempty"`
}

// ChallengeResponse represents the response from GET /api/cli/challenge/[slug]
type ChallengeResponse struct {
	ID               int    `json:"id"`
	Title            string `json:"title"`
	Slug             string `json:"slug"`
	Description      string `json:"description"`
	Difficulty       string `json:"difficulty"` // "easy" | "medium" | "hard"
	Theme            string `json:"theme"`
	InitialSituation string `json:"initial_situation"`
	Objective        string `json:"objective"`
}

// ChallengeStatusResponse represents the response from GET /api/cli/challenge/[slug]/status
type ChallengeStatusResponse struct {
	Status      string  `json:"status"`                // "not_started" | "in_progress" | "completed"
	StartedAt   *string `json:"startedAt,omitempty"`   // ISO 8601 date string
	CompletedAt *string `json:"completedAt,omitempty"` // ISO 8601 date string
}

// ChallengeStartResponse represents the response from POST /api/cli/challenge/[slug]/start
type ChallengeStartResponse struct {
	Status    string  `json:"status"`    // "in_progress" | "completed"
	StartedAt string  `json:"startedAt"` // ISO 8601 date string
	Message   *string `json:"message,omitempty"`
}

// ObjectiveResult represents the raw validation result from a CRD
// This is the simplified payload sent by the CLI - no processing, just CRD status
type ObjectiveResult struct {
	ObjectiveKey string  `json:"objectiveKey"`      // CRD metadata.name (e.g., "pod-ready-check")
	Passed       bool    `json:"passed"`            // CRD status.allPassed
	Message      *string `json:"message,omitempty"` // CRD status message or error
}

// ChallengeSubmitRequest represents the request body for POST /api/cli/challenge/[slug]/submit
type ChallengeSubmitRequest struct {
	Results []ObjectiveResult `json:"results"` // Raw results from validation CRDs
}

// ChallengeSubmitSuccessResponse represents a successful submission response
type ChallengeSubmitSuccessResponse struct {
	Success        bool   `json:"success"` // Always true for this type
	XpAwarded      int    `json:"xpAwarded"`
	TotalXp        int    `json:"totalXp"`
	Rank           string `json:"rank"`
	RankUp         *bool  `json:"rankUp,omitempty"`
	FirstChallenge *bool  `json:"firstChallenge,omitempty"`
}

// ChallengeSubmitFailureResponse represents a failed submission response
type ChallengeSubmitFailureResponse struct {
	Success bool   `json:"success"` // Always false for this type
	Message string `json:"message"`
}

// ChallengeSubmitResponse is a union type that can be either success or failure
// Check the Success field to determine which type it is
type ChallengeSubmitResponse struct {
	Success        bool    `json:"success"`
	XpAwarded      *int    `json:"xpAwarded,omitempty"`
	TotalXp        *int    `json:"totalXp,omitempty"`
	Rank           *string `json:"rank,omitempty"`
	RankUp         *bool   `json:"rankUp,omitempty"`
	FirstChallenge *bool   `json:"firstChallenge,omitempty"`
	Message        *string `json:"message,omitempty"`
}

// ChallengeResetResponse represents the response from POST /api/cli/challenge/[slug]/reset
type ChallengeResetResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ErrorResponse represents a standard error response from the API
type ErrorResponse struct {
	Error   string  `json:"error"`
	Details *string `json:"details,omitempty"`
}

// TrackRequest represents the request body for POST /api/cli/track/*
type TrackRequest struct {
	CLIVersion string `json:"cliVersion"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
}

// Type aliases for backward compatibility with existing CLI code
type UserProfile = UserResponse
type ChallengeEntity = ChallengeResponse
