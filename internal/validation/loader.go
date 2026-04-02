package validation

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
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

	// MaxTriggerWaitSeconds caps WaitSeconds, WaitAfterSeconds, and DurationSeconds to
	// prevent challenges from hanging the CLI indefinitely due to misconfiguration.
	MaxTriggerWaitSeconds = 3600

	// MaxLoadRPS caps the requestsPerSecond for the load trigger to prevent
	// accidental goroutine exhaustion on the CLI host. At the default 5 s HTTP
	// timeout, rps goroutines can pile up to rps * 5 concurrently.
	MaxLoadRPS = 1000
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
	}

	// Developer override: set KUBEASY_LOCAL_CHALLENGES_DIR to load from local clone
	if localDir := os.Getenv("KUBEASY_LOCAL_CHALLENGES_DIR"); localDir != "" {
		paths = append(paths, filepath.Join(localDir, slug, "challenge.yaml"))
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil { //nolint:gosec // slug sanitized with filepath.Base above
			return p
		}
	}
	return ""
}

// httpFetchRaw fetches the content at url, which must start with ChallengesRepoBaseURL.
func httpFetchRaw(url string) ([]byte, error) {
	if !strings.HasPrefix(url, ChallengesRepoBaseURL) {
		return nil, fmt.Errorf("invalid URL: must be from %s", ChallengesRepoBaseURL)
	}

	resp, err := http.Get(url) //nolint:gosec // URL validated against ChallengesRepoBaseURL above
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	return data, nil
}

// loadFromURL loads validations from a remote URL (internal use only)
// URL must be from ChallengesRepoBaseURL for security
func loadFromURL(url string) (*ValidationConfig, error) {
	data, err := httpFetchRaw(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch validations: %w", err)
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

// ParseChallengeYaml parses the full challenge.yaml into a ChallengeYamlSpec.
func ParseChallengeYaml(data []byte) (*ChallengeYamlSpec, error) {
	var spec ChallengeYamlSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse challenge.yaml: %w", err)
	}
	return &spec, nil
}

// LoadChallengeYamlForChallenge loads the full ChallengeYamlSpec for a given challenge slug.
// It tries a local file first (for development), then falls back to GitHub.
func LoadChallengeYamlForChallenge(slug string) (*ChallengeYamlSpec, error) {
	if localPath := FindLocalChallengeFile(slug); localPath != "" {
		data, err := os.ReadFile(localPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
		return ParseChallengeYaml(data)
	}

	// Fall back to GitHub
	url := fmt.Sprintf("%s/%s/challenge.yaml", ChallengesRepoBaseURL, slug)
	data, err := httpFetchRaw(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch challenge.yaml: %w", err)
	}
	return ParseChallengeYaml(data)
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
			validOperators := []string{"==", "!=", ">", "<", ">=", "<=", "in", "contains"}
			if !containsString(validOperators, check.Operator) {
				return fmt.Errorf("check %d: invalid operator %s (valid: %v)", i, check.Operator, validOperators)
			}
			// Validate operator-specific value types
			if check.Operator == "in" {
				list, ok := check.Value.([]interface{})
				if !ok {
					return fmt.Errorf("check %d: operator 'in' requires a list value, got %T", i, check.Value)
				}
				if len(list) == 0 {
					return fmt.Errorf("check %d: operator 'in' requires a non-empty list", i)
				}
			}
			if check.Operator == "contains" {
				if _, ok := check.Value.(string); !ok {
					return fmt.Errorf("check %d: operator 'contains' requires a string value, got %T", i, check.Value)
				}
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
		if spec.MatchMode != "" && spec.MatchMode != MatchModeAllOf && spec.MatchMode != MatchModeAnyOf {
			return fmt.Errorf("log spec: matchMode must be %q or %q, got %q", MatchModeAllOf, MatchModeAnyOf, spec.MatchMode)
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
		if len(spec.ForbiddenReasons) == 0 && len(spec.RequiredReasons) == 0 {
			return fmt.Errorf("event validation must specify at least one of forbiddenReasons or requiredReasons")
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
		// EXT-01: fail fast if mode: external is combined with sourcePod (incoherent spec)
		if spec.Mode == ConnectivityModeExternal {
			sp := spec.SourcePod
			if sp.Name != "" || len(sp.LabelSelector) > 0 || sp.Namespace != "" {
				return fmt.Errorf("mode: external is incompatible with sourcePod (remove sourcePod or use mode: internal)")
			}
		} else if spec.Mode != "" && spec.Mode != ConnectivityModeInternal {
			return fmt.Errorf("invalid mode %q: must be \"internal\" or \"external\"", spec.Mode)
		}
		// Apply default timeout to connectivity targets if not specified
		for i := range spec.Targets {
			if spec.Targets[i].TimeoutSeconds == 0 {
				spec.Targets[i].TimeoutSeconds = DefaultConnectivityTimeoutSeconds
			}
		}
		// Validate per-target TLS configuration.
		for i, t := range spec.Targets {
			if t.TLS == nil {
				continue
			}
			// TLS config is only meaningful for external mode over https://.
			if spec.Mode != ConnectivityModeExternal {
				return fmt.Errorf("target %d: tls config is only valid with mode: external", i)
			}
			// TLS config requires an https:// URL.
			if !strings.HasPrefix(t.URL, "https://") {
				return fmt.Errorf("target %d: tls config requires an https:// URL, got %q", i, t.URL)
			}
			// InsecureSkipVerify is incompatible with explicit validation flags.
			if t.TLS.InsecureSkipVerify && (t.TLS.ValidateExpiry || t.TLS.ValidateSANs) {
				return fmt.Errorf("target %d: insecureSkipVerify: true is incompatible with validateExpiry or validateSANs", i)
			}
		}
		v.Spec = spec

	case TypeRbac:
		var spec RbacSpec
		if err := yaml.Unmarshal(specYAML, &spec); err != nil {
			return err
		}
		if spec.ServiceAccount == "" {
			return fmt.Errorf("rbac validation must specify serviceAccount")
		}
		if spec.Namespace == "" {
			return fmt.Errorf("rbac validation must specify namespace")
		}
		if len(spec.Checks) == 0 {
			return fmt.Errorf("rbac validation must have at least one check")
		}
		validVerbs := []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"}
		for i, check := range spec.Checks {
			if check.Verb == "" {
				return fmt.Errorf("check %d: verb is required", i)
			}
			if !containsString(validVerbs, check.Verb) {
				return fmt.Errorf("check %d: invalid verb %q (valid: %v)", i, check.Verb, validVerbs)
			}
			if check.Resource == "" {
				return fmt.Errorf("check %d: resource is required", i)
			}
		}
		v.Spec = spec

	case TypeSpec:
		var spec SpecSpec
		if err := yaml.Unmarshal(specYAML, &spec); err != nil {
			return err
		}
		if err := validateTarget(spec.Target); err != nil {
			return err
		}
		if len(spec.Checks) == 0 {
			return fmt.Errorf("spec validation must have at least one check")
		}
		for i, check := range spec.Checks {
			if check.Path == "" {
				return fmt.Errorf("check %d: path is required", i)
			}
			checkCount := 0
			if check.Exists != nil {
				checkCount++
			}
			if check.Value != nil {
				checkCount++
			}
			if check.Contains != nil {
				checkCount++
			}
			if checkCount == 0 {
				return fmt.Errorf("check %d (path %q): one of exists, value, or contains is required", i, check.Path)
			}
			if checkCount > 1 {
				return fmt.Errorf("check %d (path %q): only one of exists, value, or contains may be set", i, check.Path)
			}
			if check.Contains != nil {
				if _, ok := check.Contains.(map[string]interface{}); !ok {
					return fmt.Errorf("check %d (path %q): contains must be a map", i, check.Path)
				}
			}
		}
		v.Spec = spec

	case TypeTriggered:
		var spec TriggeredSpec
		if err := yaml.Unmarshal(specYAML, &spec); err != nil {
			return err
		}
		// Validate trigger type
		validTriggerTypes := []string{
			string(TriggerTypeLoad),
			string(TriggerTypeWait),
			string(TriggerTypeDelete),
			string(TriggerTypeRollout),
			string(TriggerTypeScale),
		}
		if !containsString(validTriggerTypes, string(spec.Trigger.Type)) {
			return fmt.Errorf("invalid trigger type %q (valid: %v)", spec.Trigger.Type, validTriggerTypes)
		}
		// Validate trigger type specific fields
		switch spec.Trigger.Type {
		case TriggerTypeLoad:
			if spec.Trigger.URL == "" {
				return fmt.Errorf("load trigger requires url")
			}
			if !strings.HasPrefix(spec.Trigger.URL, "http://") && !strings.HasPrefix(spec.Trigger.URL, "https://") {
				return fmt.Errorf("load trigger url must start with http:// or https://")
			}
			if spec.Trigger.DurationSeconds > MaxTriggerWaitSeconds {
				return fmt.Errorf("load trigger durationSeconds %d exceeds maximum %d", spec.Trigger.DurationSeconds, MaxTriggerWaitSeconds)
			}
			if spec.Trigger.RequestsPerSecond > MaxLoadRPS {
				return fmt.Errorf("load trigger requestsPerSecond %d exceeds maximum %d", spec.Trigger.RequestsPerSecond, MaxLoadRPS)
			}
		case TriggerTypeWait:
			if spec.Trigger.WaitSeconds > MaxTriggerWaitSeconds {
				return fmt.Errorf("wait trigger waitSeconds %d exceeds maximum %d", spec.Trigger.WaitSeconds, MaxTriggerWaitSeconds)
			}
		case TriggerTypeDelete:
			if spec.Trigger.Target == nil {
				return fmt.Errorf("delete trigger requires target")
			}
			if spec.Trigger.Target.Kind == "" {
				return fmt.Errorf("delete trigger requires target.kind")
			}
			if err := validateTarget(*spec.Trigger.Target); err != nil {
				return fmt.Errorf("delete trigger target: %w", err)
			}
		case TriggerTypeRollout:
			if spec.Trigger.Target == nil {
				return fmt.Errorf("rollout trigger requires target")
			}
			if spec.Trigger.Target.Name == "" {
				return fmt.Errorf("rollout trigger requires target.name")
			}
			if spec.Trigger.Image == "" {
				return fmt.Errorf("rollout trigger requires image")
			}
			validRolloutKinds := []string{"Deployment", "StatefulSet", "DaemonSet"}
			if !containsString(validRolloutKinds, spec.Trigger.Target.Kind) {
				return fmt.Errorf("rollout trigger target.kind must be one of %v, got %q", validRolloutKinds, spec.Trigger.Target.Kind)
			}
		case TriggerTypeScale:
			if spec.Trigger.Target == nil {
				return fmt.Errorf("scale trigger requires target")
			}
			if spec.Trigger.Target.Kind == "" {
				return fmt.Errorf("scale trigger requires target.kind")
			}
			if spec.Trigger.Target.Name == "" {
				return fmt.Errorf("scale trigger requires target.name")
			}
			if spec.Trigger.Replicas == nil {
				return fmt.Errorf("scale trigger requires replicas")
			}
		}
		// Validate WaitAfterSeconds cap
		if spec.WaitAfterSeconds > MaxTriggerWaitSeconds {
			return fmt.Errorf("waitAfterSeconds %d exceeds maximum %d", spec.WaitAfterSeconds, MaxTriggerWaitSeconds)
		}
		// Validate then sub-validations
		if len(spec.Then) == 0 {
			return fmt.Errorf("triggered validation must have at least one then validator")
		}
		for i := range spec.Then {
			// Nested triggered validators would create unbounded orchestration chains
			if spec.Then[i].Type == TypeTriggered {
				return fmt.Errorf("then[%d]: nested triggered validators are not supported", i)
			}
			// Assign a default key if not provided
			if spec.Then[i].Key == "" {
				spec.Then[i].Key = fmt.Sprintf("then[%d]", i)
			}
			if err := parseSpec(&spec.Then[i]); err != nil {
				return fmt.Errorf("then[%d]: %w", i, err)
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

// containsString checks if a string is in a slice
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
