package validation

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kubeasy-dev/kubeasy-cli/internal/api"
	"github.com/kubeasy-dev/registry/pkg/challenges"
	"go.yaml.in/yaml/v3"
)

const (
	// DefaultLogSinceSeconds is the default time window for log searches (5 minutes).
	DefaultLogSinceSeconds = 300

	// DefaultEventSinceSeconds is the default time window for event searches (5 minutes).
	DefaultEventSinceSeconds = 300

	// DefaultConnectivityTimeoutSeconds is the default timeout for connectivity checks.
	DefaultConnectivityTimeoutSeconds = 5

	// MaxTriggerWaitSeconds caps WaitSeconds, WaitAfterSeconds, and DurationSeconds.
	MaxTriggerWaitSeconds = 3600

	// MaxLoadRPS caps requestsPerSecond for the load trigger.
	MaxLoadRPS = 1000
)

// LoadFromFile loads validations from a local challenge.yaml file.
func LoadFromFile(path string) (*ValidationConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return Parse(data)
}

// FindLocalChallengeFile looks for challenge.yaml in common local development paths.
func FindLocalChallengeFile(slug string) string {
	slug = filepath.Base(slug) // prevent path traversal

	paths := []string{
		filepath.Join(".", slug, "challenge.yaml"),
		filepath.Join("..", "challenges", slug, "challenge.yaml"),
	}

	if localDir := os.Getenv("KUBEASY_LOCAL_CHALLENGES_DIR"); localDir != "" {
		paths = append(paths, filepath.Join(localDir, slug, "challenge.yaml"))
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil { //nolint:gosec // slug sanitized above
			return p
		}
	}
	return ""
}

// Parse parses a challenge.yaml into a ValidationConfig ready for execution.
// Delegates to the registry's shared parser and applies CLI-specific defaults.
func Parse(data []byte) (*ValidationConfig, error) {
	c, err := challenges.ParseBytes(data, "")
	if err != nil {
		return nil, fmt.Errorf("failed to parse challenge: %w", err)
	}
	return fromChallenge(c), nil
}

// fromChallenge converts a registry Challenge into a CLI ValidationConfig.
// Applies execution defaults (e.g. SinceSeconds) and dereferences pointer specs
// to match the value-type assertions in the executor.
func fromChallenge(c *challenges.Challenge) *ValidationConfig {
	validations := make([]Validation, len(c.Objectives))
	for i, obj := range c.Objectives {
		validations[i] = fromObjective(obj)
	}
	return &ValidationConfig{Validations: validations}
}

func fromObjective(obj challenges.Objective) Validation {
	v := Validation{
		Key:         obj.Key,
		Title:       obj.Title,
		Description: obj.Description,
		Order:       obj.Order,
		Type:        obj.Type,
	}

	// Registry stores pointer specs; executors assert value types — dereference here.
	switch s := obj.Spec.(type) {
	case *StatusSpec:
		v.Spec = *s
	case *ConditionSpec:
		v.Spec = *s
	case *LogSpec:
		cp := *s
		if cp.SinceSeconds == 0 {
			cp.SinceSeconds = DefaultLogSinceSeconds
		}
		v.Spec = cp
	case *EventSpec:
		cp := *s
		if cp.SinceSeconds == 0 {
			cp.SinceSeconds = DefaultEventSinceSeconds
		}
		v.Spec = cp
	case *ConnectivitySpec:
		cp := *s
		for i := range cp.Targets {
			if cp.Targets[i].TimeoutSeconds == 0 {
				cp.Targets[i].TimeoutSeconds = DefaultConnectivityTimeoutSeconds
			}
		}
		v.Spec = cp
	case *RbacSpec:
		v.Spec = *s
	case *SpecSpec:
		v.Spec = *s
	case *challenges.TriggeredSpec:
		then := make([]Validation, len(s.Then))
		for j, thenObj := range s.Then {
			then[j] = fromObjective(thenObj)
		}
		v.Spec = TriggeredSpec{
			Trigger:          s.Trigger,
			WaitAfterSeconds: s.WaitAfterSeconds,
			Then:             then,
		}
	default:
		// Unknown objective type not yet supported by this CLI version.
		// v.Spec remains nil; the executor will report an "Unknown validation type" error.
	}

	return v
}

// LoadForChallenge loads validations for a challenge slug.
// Tries local file first (dev override), then the Kubeasy API.
func LoadForChallenge(slug string) (*ValidationConfig, error) {
	if localPath := FindLocalChallengeFile(slug); localPath != "" {
		return LoadFromFile(localPath)
	}

	client, err := api.NewPublicClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetChallengeYamlWithResponse(context.Background(), slug)
	if err != nil {
		return nil, fmt.Errorf("failed to load challenge %q from API: %w", slug, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("API returned HTTP %d for challenge %q", resp.StatusCode(), slug)
	}

	return Parse(resp.Body)
}

// ParseChallengeYaml parses challenge.yaml bytes into a ChallengeYamlSpec (for lint/display).
func ParseChallengeYaml(data []byte) (*ChallengeYamlSpec, error) {
	var spec ChallengeYamlSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse challenge.yaml: %w", err)
	}
	return &spec, nil
}

// LoadChallengeYamlForChallenge loads the full ChallengeYamlSpec for display in kubeasy start.
// Tries local file first, then the Kubeasy API.
func LoadChallengeYamlForChallenge(slug string) (*ChallengeYamlSpec, error) {
	if localPath := FindLocalChallengeFile(slug); localPath != "" {
		data, err := os.ReadFile(localPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
		return ParseChallengeYaml(data)
	}

	client, err := api.NewPublicClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.GetChallengeYamlWithResponse(context.Background(), slug)
	if err != nil {
		return nil, fmt.Errorf("failed to load challenge %q from API: %w", slug, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("API returned HTTP %d for challenge %q", resp.StatusCode(), slug)
	}

	return ParseChallengeYaml(resp.Body)
}
