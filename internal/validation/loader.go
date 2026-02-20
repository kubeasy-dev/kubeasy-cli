package validation

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// ChallengesRepoBaseURL is the base URL for the challenges repository
	ChallengesRepoBaseURL = "https://raw.githubusercontent.com/kubeasy-dev/challenges/main"

	// DefaultLogSinceSeconds is the default time window for log searches (5 minutes).
	// 300 seconds balances capturing recent activity while avoiding false positives
	// from old logs. Most K8s applications complete startup within this window.
	DefaultLogSinceSeconds = 300

	// DefaultEventSinceSeconds is the default time window for event searches (5 minutes).
	// Matches log window for consistency. Events older than this are typically
	// resolved or no longer relevant to the current validation attempt.
	DefaultEventSinceSeconds = 300

	// DefaultConnectivityTimeoutSeconds is the default timeout for connectivity checks.
	// 5 seconds is sufficient for healthy in-cluster HTTP requests while detecting
	// network issues without making validations excessively slow.
	DefaultConnectivityTimeoutSeconds = 5
)

// LoadFromFile loads validations from a local file
func LoadFromFile(path string) (*ValidationConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return Parse(data)
}

// FindLocalChallengeFile looks for challenge.yaml in common locations
func FindLocalChallengeFile(slug string) string {
	// Sanitize slug to prevent path traversal
	slug = filepath.Base(slug)

	// Check common development paths
	paths := []string{
		filepath.Join(".", slug, "challenge.yaml"),
		filepath.Join("..", "challenges", slug, "challenge.yaml"),
		filepath.Join(os.Getenv("HOME"), "Workspace", "kubeasy", "challenges", slug, "challenge.yaml"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil { //nolint:gosec // slug sanitized with filepath.Base above
			return p
		}
	}
	return ""
}

// loadFromURL loads validations from a remote URL (internal use only)
// URL must be from ChallengesRepoBaseURL for security
func loadFromURL(url string) (*ValidationConfig, error) {
	// Validate URL starts with trusted base URL
	if !strings.HasPrefix(url, ChallengesRepoBaseURL) {
		return nil, fmt.Errorf("invalid URL: must be from %s", ChallengesRepoBaseURL)
	}

	resp, err := http.Get(url) //nolint:gosec // URL validated against ChallengesRepoBaseURL
	if err != nil {
		return nil, fmt.Errorf("failed to fetch validations: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch validations: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return Parse(data)
}

// LoadForChallenge loads validations for a specific challenge
// It tries local file first (for development), then falls back to GitHub
func LoadForChallenge(slug string) (*ValidationConfig, error) {
	// Try local file first (for development)
	if localPath := FindLocalChallengeFile(slug); localPath != "" {
		return LoadFromFile(localPath)
	}

	// Fall back to GitHub
	url := fmt.Sprintf("%s/%s/challenge.yaml", ChallengesRepoBaseURL, slug)
	return loadFromURL(url)
}

// Parse parses validation config from YAML data
func Parse(data []byte) (*ValidationConfig, error) {
	var config ValidationConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse validations: %w", err)
	}

	// Parse specs into typed structs
	for i := range config.Validations {
		if err := parseSpec(&config.Validations[i]); err != nil {
			return nil, fmt.Errorf("failed to parse spec for %s: %w", config.Validations[i].Key, err)
		}
	}

	return &config, nil
}

// parseSpec converts RawSpec to the appropriate typed spec based on Type
func parseSpec(v *Validation) error {
	if v.RawSpec == nil {
		return fmt.Errorf("spec is required")
	}

	// Convert RawSpec back to YAML then to typed struct
	specYAML, err := yaml.Marshal(v.RawSpec)
	if err != nil {
		return err
	}

	switch v.Type {
	case TypeStatus:
		var spec StatusSpec
		if err := yaml.Unmarshal(specYAML, &spec); err != nil {
			return err
		}
		if err := validateTarget(spec.Target); err != nil {
			return err
		}
		// Validate checks
		if len(spec.Checks) == 0 {
			return fmt.Errorf("status validation must have at least one check")
		}
		for i, check := range spec.Checks {
			if check.Field == "" {
				return fmt.Errorf("check %d: field is required", i)
			}
			if check.Operator == "" {
				return fmt.Errorf("check %d: operator is required", i)
			}
			if check.Value == nil {
				return fmt.Errorf("check %d: value is required", i)
			}
			// Validate operator
			validOperators := []string{"==", "!=", ">", "<", ">=", "<="}
			if !containsString(validOperators, check.Operator) {
				return fmt.Errorf("check %d: invalid operator %s (valid: %v)", i, check.Operator, validOperators)
			}
			// Validate field path exists in Kind's Status using reflection.
			// Only validate for supported kinds (skip for custom resources).
			// Note: This validates that the field EXISTS in the Go type definition,
			// but some fields are conditionally populated at runtime (e.g., containerStatuses
			// only exists after containers start). Such fields pass parse-time validation
			// but may still fail at runtime if the resource isn't in the expected state.
			// See docs/VALIDATION_EXAMPLES.md "Troubleshooting" section for details.
			if IsKindSupported(spec.Target.Kind) {
				if err := ValidateFieldPath(spec.Target.Kind, check.Field); err != nil {
					return fmt.Errorf("check %d (field %q): %w", i, check.Field, err)
				}
			}
		}
		v.Spec = spec

	case TypeCondition:
		var spec ConditionSpec
		if err := yaml.Unmarshal(specYAML, &spec); err != nil {
			return err
		}
		if err := validateTarget(spec.Target); err != nil {
			return err
		}
		// Validate checks
		if len(spec.Checks) == 0 {
			return fmt.Errorf("condition validation must have at least one check")
		}
		for i, check := range spec.Checks {
			if check.Type == "" {
				return fmt.Errorf("check %d: type is required", i)
			}
			if check.Status == "" {
				return fmt.Errorf("check %d: status is required", i)
			}
		}
		v.Spec = spec

	case TypeLog:
		var spec LogSpec
		if err := yaml.Unmarshal(specYAML, &spec); err != nil {
			return err
		}
		if err := validateTarget(spec.Target); err != nil {
			return err
		}
		// Apply default sinceSeconds if not specified
		if spec.SinceSeconds == 0 {
			spec.SinceSeconds = DefaultLogSinceSeconds
		}
		v.Spec = spec

	case TypeEvent:
		var spec EventSpec
		if err := yaml.Unmarshal(specYAML, &spec); err != nil {
			return err
		}
		if err := validateTarget(spec.Target); err != nil {
			return err
		}
		// Apply default sinceSeconds if not specified
		if spec.SinceSeconds == 0 {
			spec.SinceSeconds = DefaultEventSinceSeconds
		}
		v.Spec = spec

	case TypeConnectivity:
		var spec ConnectivitySpec
		if err := yaml.Unmarshal(specYAML, &spec); err != nil {
			return err
		}
		if err := validateSourcePod(spec.SourcePod); err != nil {
			return err
		}
		// Apply default timeout to connectivity targets if not specified
		for i := range spec.Targets {
			if spec.Targets[i].TimeoutSeconds == 0 {
				spec.Targets[i].TimeoutSeconds = DefaultConnectivityTimeoutSeconds
			}
		}
		v.Spec = spec

	default:
		return fmt.Errorf("unknown validation type: %s", v.Type)
	}

	return nil
}

// validateTarget checks if a target has at least name or labelSelector
func validateTarget(target Target) error {
	if target.Name == "" && len(target.LabelSelector) == 0 {
		return fmt.Errorf("target must specify either name or labelSelector")
	}
	return nil
}

// validateSourcePod checks if a source pod has at least name or labelSelector
func validateSourcePod(sourcePod SourcePod) error {
	if sourcePod.Name == "" && len(sourcePod.LabelSelector) == 0 {
		return fmt.Errorf("sourcePod must specify either name or labelSelector")
	}
	return nil
}

// containsString checks if a string is in a slice
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
