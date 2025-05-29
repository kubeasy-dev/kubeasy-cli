package api

import (
	"encoding/json"
	"fmt"
	"time"

	// Added for JWT parsing
	"github.com/golang-jwt/jwt/v5" // Added for JWT parsing
	"github.com/kubeasy-dev/kubeasy-cli/pkg/constants"
	"github.com/supabase-community/supabase-go"
	"github.com/zalando/go-keyring"
)

// ChallengeEntity defines the structure for challenge data
type ChallengeEntity struct {
	Id               string `json:"id"`
	Title            string `json:"title"`
	Slug             string `json:"slug"`
	Description      string `json:"description"`
	Difficulty       string `json:"difficulty"`
	Theme            string `json:"theme"`
	InitialSituation string `json:"initial_situation"`
	Objective        string `json:"objective"`
}

type ProgressStatus string

const (
	ProgressStatusInProgress ProgressStatus = "in_progress"
	ProgressStatusCompleted  ProgressStatus = "completed"
)

type UserProgress struct {
	Id          string         `json:"id"`
	UserId      string         `json:"user_id"`
	ChallengeId string         `json:"challenge_id"`
	Status      ProgressStatus `json:"status"`
	CompletedAt string         `json:"completed_at"`
	StartedAt   string         `json:"started_at"`
}

// createSupabaseClient initializes and returns a Supabase client.
// It retrieves the API key from the system keyring.
func createSupabaseClient() (*supabase.Client, error) {
	apiKey, err := keyring.Get(constants.KeyringServiceName, "api_key")
	if err != nil {
		// Return an error instead of panicking if the key is not found or keyring fails
		return nil, fmt.Errorf("failed to get API key from keyring: %w. Please run 'kubeasy-cli login'", err)
	}

	// Initialize Supabase client
	client, err := supabase.NewClient(constants.RestAPIUrl, apiKey, &supabase.ClientOptions{})
	if err != nil {
		// Return error if client initialization fails
		return nil, fmt.Errorf("cannot initialize supabase client: %w", err)
	}

	return client, nil
}

func getUserIdFromKeyring() (string, error) {
	apiKey, err := keyring.Get(constants.KeyringServiceName, "api_key")
	if err != nil {
		// Return an error instead of panicking if the key is not found or keyring fails
		return "", fmt.Errorf("failed to get API key from keyring: %w. Please run 'kubeasy-cli login'", err)
	}

	// apiKey is a JWT token, we need to decode it to get the user id (sub)
	token, _, err := new(jwt.Parser).ParseUnverified(apiKey, jwt.MapClaims{})
	if err != nil {
		return "", fmt.Errorf("failed to parse JWT token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid JWT claims format")
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", fmt.Errorf("could not find 'sub' claim in JWT token")
	}

	return sub, nil
}

// GetChallenge retrieves a specific challenge by its slug name from the API.
func GetChallenge(challengeSlug string) (*ChallengeEntity, error) {
	client, err := createSupabaseClient()
	if err != nil {
		// Propagate error from client creation (e.g., missing API key)
		return nil, err
	}

	// Fetch the challenge data
	data, _, err := client.From("challenges").Select("*", "exact", false).Eq("slug", challengeSlug).Single().Execute()
	if err != nil {
		// Return error if the API call fails (e.g., challenge not found, network issue)
		return nil, fmt.Errorf("failed to fetch challenge '%s': %w", challengeSlug, err)
	}

	// Unmarshal the JSON response into ExerciseEntity struct
	var exercise ChallengeEntity
	err = json.Unmarshal(data, &exercise)
	if err != nil {
		// Return error if JSON parsing fails
		return nil, fmt.Errorf("failed to parse challenge data for '%s': %w", challengeSlug, err)
	}

	return &exercise, nil
}

func GetChallengeProgress(challengeSlug string) (*UserProgress, error) {
	client, err := createSupabaseClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Supabase client: %w", err)
	}

	challenge, err := GetChallenge(challengeSlug)
	if err != nil {
		return nil, err
	}

	userId, err := getUserIdFromKeyring()
	if err != nil {
		return nil, err
	}

	data, _, err := client.From("user_progress").Select("*", "exact", false).Eq("user_id", userId).Eq("challenge_id", challenge.Id).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user progress for challenge '%s': %w", challengeSlug, err)
	}

	var progresses []UserProgress
	err = json.Unmarshal(data, &progresses)
	if err != nil {
		return nil, fmt.Errorf("failed to parse user progress for challenge '%s': %w", challengeSlug, err)
	}

	if len(progresses) == 0 {
		return nil, nil
	}

	return &progresses[0], nil
}

func StartChallenge(challengeSlug string) error {
	client, err := createSupabaseClient()
	if err != nil {
		return fmt.Errorf("failed to create Supabase client: %w", err)
	}

	challenge, err := GetChallenge(challengeSlug)
	if err != nil {
		return err
	}

	userId, err := getUserIdFromKeyring()
	if err != nil {
		return err
	}

	progressData := map[string]interface{}{
		"user_id":      userId,
		"challenge_id": challenge.Id,
		"status":       "in_progress",
		"completed_at": nil,
		"started_at":   time.Now(),
	}

	_, _, err = client.From("user_progress").Insert(progressData, true, "", "", "exact").Execute()

	if err != nil {
		return fmt.Errorf("failed to upsert user progress for challenge '%s': %w", challengeSlug, err)
	}

	return nil
}

func SendSubmit(challengeId string, staticValidation bool, dynamicValidation bool, payload interface{}) error {
	client, err := createSupabaseClient()
	if err != nil {
		return fmt.Errorf("failed to create Supabase client: %w", err)
	}

	userId, err := getUserIdFromKeyring()
	if err != nil {
		return err
	}

	submitData := map[string]interface{}{
		"user_progress":      fmt.Sprintf("%s+%s", userId, challengeId),
		"static_validation":  staticValidation,
		"dynamic_validation": dynamicValidation,
		"payload":            payload,
	}

	_, _, err = client.From("user_submissions").Insert(submitData, false, "", "id", "exact").Execute()

	if err != nil {
		return err
	}

	return nil
}

func ResetChallengeProgress(challengeId string) error {
	client, err := createSupabaseClient()
	if err != nil {
		return fmt.Errorf("failed to create Supabase client: %w", err)
	}

	userId, err := getUserIdFromKeyring()
	if err != nil {
		return err
	}

	_, _, err = client.From("user_progress").Delete("", "exact").Filter("user_id", "eq", userId).Filter("challenge_id", "eq", challengeId).Execute()

	if err != nil {
		return fmt.Errorf("failed to delete user progress for challenge '%s': %w", challengeId, err)
	}

	return nil
}
