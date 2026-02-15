// Package api provides the HTTP client layer for the Kubeasy CLI API.
//
// Response types are kept here as stable interfaces for the rest of the CLI.
// The generated client (internal/apigen) uses inline anonymous structs, so
// these named types provide backward compatibility and a cleaner API.

package api

// UserResponse represents the response from GET /api/cli/user
type UserResponse struct {
	FirstName string  `json:"firstName"`
	LastName  *string `json:"lastName,omitempty"`
}

// LoginResponse represents the response from POST /api/cli/user
type LoginResponse struct {
	FirstName  string  `json:"firstName"`
	LastName   *string `json:"lastName,omitempty"`
	FirstLogin *bool   `json:"firstLogin,omitempty"`
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
type ObjectiveResult struct {
	ObjectiveKey string  `json:"objectiveKey"`      // CRD metadata.name
	Passed       bool    `json:"passed"`            // CRD status.allPassed
	Message      *string `json:"message,omitempty"` // CRD status message or error
}

// ChallengeSubmitRequest represents the request body for POST /api/cli/challenge/[slug]/submit
type ChallengeSubmitRequest struct {
	Results []ObjectiveResult `json:"results"`
}

// ChallengeSubmitResponse is a union type that can be either success or failure.
// Check the Success field to determine which type it is.
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

// Type aliases for backward compatibility
type UserProfile = UserResponse
type ChallengeEntity = ChallengeResponse
