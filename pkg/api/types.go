// Code generated from website/types/cli-api.ts - DO NOT EDIT manually
// To regenerate, update website/types/cli-api.ts and sync changes here

package api

// UserProfile represents the response from GET /api/cli/user
type UserProfile struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// UserResponse is an alias for UserProfile for compatibility
type UserResponse = UserProfile

// ChallengeEntity represents the response from GET /api/cli/challenge/[slug]
// This is the main type name used in the codebase
type ChallengeEntity struct {
	ID               int    `json:"id"`
	Title            string `json:"title"`
	Slug             string `json:"slug"`
	Description      string `json:"description"`
	Difficulty       string `json:"difficulty"` // "easy" | "medium" | "hard"
	Theme            string `json:"theme"`
	InitialSituation string `json:"initial_situation"`
	Objective        string `json:"objective"`
}

// ChallengeResponse is an alias for ChallengeEntity for consistency with API naming
type ChallengeResponse = ChallengeEntity

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

// ChallengeSubmitRequest represents the request body for POST /api/cli/challenge/[slug]/submit
type ChallengeSubmitRequest struct {
	Validated         bool        `json:"validated"`                    // Overall validation result (required)
	StaticValidation  *bool       `json:"static_validation,omitempty"`  // Static validation result (optional)
	DynamicValidation *bool       `json:"dynamic_validation,omitempty"` // Dynamic validation result (optional)
	Payload           interface{} `json:"payload,omitempty"`            // Additional validation details (optional)
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
