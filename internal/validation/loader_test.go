package validation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParse_StatusValidation tests parsing of status validation spec
func TestParse_StatusValidation(t *testing.T) {
	yaml := `
objectives:
  - key: deployment-ready
    title: Deployment Ready
    description: Deployment must have ready replicas
    order: 1
    type: status
    spec:
      target:
        kind: Deployment
        labelSelector:
          app: test-app
      checks:
        - field: readyReplicas
          operator: ">="
          value: 3
`

	config, err := Parse([]byte(yaml))
	require.NoError(t, err)
	require.Len(t, config.Validations, 1)

	v := config.Validations[0]
	assert.Equal(t, "deployment-ready", v.Key)
	assert.Equal(t, "Deployment Ready", v.Title)
	assert.Equal(t, TypeStatus, v.Type)

	spec, ok := v.Spec.(StatusSpec)
	require.True(t, ok, "spec should be StatusSpec")
	assert.Equal(t, "Deployment", spec.Target.Kind)
	assert.Equal(t, "test-app", spec.Target.LabelSelector["app"])
	require.Len(t, spec.Checks, 1)
	assert.Equal(t, "readyReplicas", spec.Checks[0].Field)
	assert.Equal(t, ">=", spec.Checks[0].Operator)
}

// TestParse_ConditionValidation tests parsing of condition validation spec
func TestParse_ConditionValidation(t *testing.T) {
	yaml := `
objectives:
  - key: pod-ready
    title: Pod Ready
    description: Pod must be running
    order: 1
    type: condition
    spec:
      target:
        kind: Pod
        labelSelector:
          app: test-app
      checks:
        - type: Ready
          status: "True"
`

	config, err := Parse([]byte(yaml))
	require.NoError(t, err)
	require.Len(t, config.Validations, 1)

	v := config.Validations[0]
	assert.Equal(t, "pod-ready", v.Key)
	assert.Equal(t, "Pod Ready", v.Title)
	assert.Equal(t, TypeCondition, v.Type)

	spec, ok := v.Spec.(ConditionSpec)
	require.True(t, ok, "spec should be ConditionSpec")
	assert.Equal(t, "Pod", spec.Target.Kind)
	assert.Equal(t, "test-app", spec.Target.LabelSelector["app"])
	require.Len(t, spec.Checks, 1)
	assert.Equal(t, "Ready", spec.Checks[0].Type)
}

// TestParse_LogValidation tests parsing of log validation with defaults
func TestParse_LogValidation(t *testing.T) {
	t.Run("with explicit sinceSeconds", func(t *testing.T) {
		yaml := `
objectives:
  - key: log-check
    type: log
    spec:
      target:
        name: my-pod
      expectedStrings:
        - "Server started"
      sinceSeconds: 600
`

		config, err := Parse([]byte(yaml))
		require.NoError(t, err)

		spec, ok := config.Validations[0].Spec.(LogSpec)
		require.True(t, ok)
		assert.Equal(t, 600, spec.SinceSeconds)
		assert.Equal(t, "my-pod", spec.Target.Name)
		assert.Contains(t, spec.ExpectedStrings, "Server started")
	})

	t.Run("with default sinceSeconds", func(t *testing.T) {
		yaml := `
objectives:
  - key: log-check
    type: log
    spec:
      target:
        name: my-pod
      expectedStrings:
        - "Started"
`

		config, err := Parse([]byte(yaml))
		require.NoError(t, err)

		spec, ok := config.Validations[0].Spec.(LogSpec)
		require.True(t, ok)
		assert.Equal(t, DefaultLogSinceSeconds, spec.SinceSeconds, "should apply default")
	})
}

// TestParse_EventValidation tests parsing of event validation with defaults
func TestParse_EventValidation(t *testing.T) {
	t.Run("with explicit sinceSeconds", func(t *testing.T) {
		yaml := `
objectives:
  - key: no-crashes
    type: event
    spec:
      target:
        labelSelector:
          app: web
      forbiddenReasons:
        - OOMKilled
        - Evicted
      sinceSeconds: 120
`

		config, err := Parse([]byte(yaml))
		require.NoError(t, err)

		spec, ok := config.Validations[0].Spec.(EventSpec)
		require.True(t, ok)
		assert.Equal(t, 120, spec.SinceSeconds)
		assert.Contains(t, spec.ForbiddenReasons, "OOMKilled")
		assert.Contains(t, spec.ForbiddenReasons, "Evicted")
	})

	t.Run("with default sinceSeconds", func(t *testing.T) {
		yaml := `
objectives:
  - key: no-crashes
    type: event
    spec:
      target:
        name: test-pod
      forbiddenReasons:
        - BackOff
`

		config, err := Parse([]byte(yaml))
		require.NoError(t, err)

		spec, ok := config.Validations[0].Spec.(EventSpec)
		require.True(t, ok)
		assert.Equal(t, DefaultEventSinceSeconds, spec.SinceSeconds, "should apply default")
	})
}

// TestParse_StatusValidationMultipleChecks tests parsing status validation with multiple checks
func TestParse_StatusValidationMultipleChecks(t *testing.T) {
	yaml := `
objectives:
  - key: replica-count
    type: status
    spec:
      target:
        kind: Deployment
        name: web-server
      checks:
        - field: replicas
          operator: "=="
          value: 3
        - field: readyReplicas
          operator: ">="
          value: 2
`

	config, err := Parse([]byte(yaml))
	require.NoError(t, err)

	spec, ok := config.Validations[0].Spec.(StatusSpec)
	require.True(t, ok)
	assert.Equal(t, "Deployment", spec.Target.Kind)
	assert.Equal(t, "web-server", spec.Target.Name)

	require.Len(t, spec.Checks, 2)
	assert.Equal(t, "replicas", spec.Checks[0].Field)
	assert.Equal(t, "==", spec.Checks[0].Operator)

	assert.Equal(t, "readyReplicas", spec.Checks[1].Field)
	assert.Equal(t, ">=", spec.Checks[1].Operator)
}

// TestParse_ConnectivityValidation tests parsing of connectivity validation with defaults
func TestParse_ConnectivityValidation(t *testing.T) {
	t.Run("with explicit timeoutSeconds", func(t *testing.T) {
		yaml := `
objectives:
  - key: http-connectivity
    type: connectivity
    spec:
      sourcePod:
        labelSelector:
          app: client
      targets:
        - url: http://service:8080/health
          expectedStatusCode: 200
          timeoutSeconds: 10
`

		config, err := Parse([]byte(yaml))
		require.NoError(t, err)

		spec, ok := config.Validations[0].Spec.(ConnectivitySpec)
		require.True(t, ok)
		assert.Equal(t, "client", spec.SourcePod.LabelSelector["app"])
		require.Len(t, spec.Targets, 1)
		assert.Equal(t, "http://service:8080/health", spec.Targets[0].URL)
		assert.Equal(t, 200, spec.Targets[0].ExpectedStatusCode)
		assert.Equal(t, 10, spec.Targets[0].TimeoutSeconds)
	})

	t.Run("with default timeoutSeconds", func(t *testing.T) {
		yaml := `
objectives:
  - key: http-connectivity
    type: connectivity
    spec:
      sourcePod:
        name: client-pod
      targets:
        - url: http://api:3000
          expectedStatusCode: 200
`

		config, err := Parse([]byte(yaml))
		require.NoError(t, err)

		spec, ok := config.Validations[0].Spec.(ConnectivitySpec)
		require.True(t, ok)
		require.Len(t, spec.Targets, 1)
		assert.Equal(t, DefaultConnectivityTimeoutSeconds, spec.Targets[0].TimeoutSeconds, "should apply default")
	})

	t.Run("multiple targets with mixed timeouts", func(t *testing.T) {
		yaml := `
objectives:
  - key: multi-connectivity
    type: connectivity
    spec:
      sourcePod:
        name: test-pod
      targets:
        - url: http://fast-service:8080
          expectedStatusCode: 200
          timeoutSeconds: 3
        - url: http://slow-service:8080
          expectedStatusCode: 200
`

		config, err := Parse([]byte(yaml))
		require.NoError(t, err)

		spec, ok := config.Validations[0].Spec.(ConnectivitySpec)
		require.True(t, ok)
		require.Len(t, spec.Targets, 2)
		assert.Equal(t, 3, spec.Targets[0].TimeoutSeconds, "explicit timeout should be preserved")
		assert.Equal(t, DefaultConnectivityTimeoutSeconds, spec.Targets[1].TimeoutSeconds, "should apply default to second target")
	})
}

// TestParse_MultipleValidations tests parsing multiple validations in one config
func TestParse_MultipleValidations(t *testing.T) {
	yaml := `
objectives:
  - key: pod-ready
    type: condition
    spec:
      target:
        name: my-pod
      checks:
        - type: Ready
          status: "True"
  - key: no-errors
    type: log
    spec:
      target:
        name: my-pod
      expectedStrings:
        - "Started successfully"
  - key: no-crashes
    type: event
    spec:
      target:
        name: my-pod
      forbiddenReasons:
        - OOMKilled
`

	config, err := Parse([]byte(yaml))
	require.NoError(t, err)
	assert.Len(t, config.Validations, 3)

	assert.Equal(t, "pod-ready", config.Validations[0].Key)
	assert.Equal(t, TypeCondition, config.Validations[0].Type)

	assert.Equal(t, "no-errors", config.Validations[1].Key)
	assert.Equal(t, TypeLog, config.Validations[1].Type)

	assert.Equal(t, "no-crashes", config.Validations[2].Key)
	assert.Equal(t, TypeEvent, config.Validations[2].Type)
}

// TestParse_ValidationErrors tests error handling during parsing
func TestParse_ValidationErrors(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		errorContains string
	}{
		{
			name: "invalid YAML",
			yaml: `
objectives:
  - key: test
    type: status
    spec: [invalid yaml structure
`,
			errorContains: "failed to parse validations",
		},
		{
			name: "missing spec",
			yaml: `
objectives:
  - key: test
    type: status
`,
			errorContains: "spec is required",
		},
		{
			name: "unknown validation type",
			yaml: `
objectives:
  - key: test
    type: unknown-type
    spec:
      foo: bar
`,
			errorContains: "unknown validation type",
		},
		{
			name: "target without name or labelSelector",
			yaml: `
objectives:
  - key: test
    type: status
    spec:
      target:
        kind: Pod
      checks:
        - field: replicas
          operator: "=="
          value: 3
`,
			errorContains: "target must specify either name or labelSelector",
		},
		// Note: empty sourcePod (probe mode) is now valid — no test case needed here.
		// See TestParse_ConnectivityProbeMode for probe mode acceptance test.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}

// TestValidateTarget tests the validateTarget function
func TestValidateTarget(t *testing.T) {
	tests := []struct {
		name        string
		target      Target
		expectError bool
	}{
		{
			name:        "valid - with name",
			target:      Target{Name: "my-pod"},
			expectError: false,
		},
		{
			name:        "valid - with labelSelector",
			target:      Target{LabelSelector: map[string]string{"app": "web"}},
			expectError: false,
		},
		{
			name:        "valid - with both",
			target:      Target{Name: "my-pod", LabelSelector: map[string]string{"app": "web"}},
			expectError: false,
		},
		{
			name:        "invalid - empty",
			target:      Target{},
			expectError: true,
		},
		{
			name:        "invalid - empty labelSelector",
			target:      Target{LabelSelector: map[string]string{}},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTarget(tt.target)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestLoadFromFile tests loading validation config from file
func TestLoadFromFile(t *testing.T) {
	t.Run("success - valid file", func(t *testing.T) {
		// Create temp file
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "challenge.yaml")

		content := `
objectives:
  - key: test
    type: condition
    spec:
      target:
        name: test-pod
      checks:
        - type: Ready
          status: "True"
`
		err := os.WriteFile(filePath, []byte(content), 0600)
		require.NoError(t, err)

		// Load file
		config, err := LoadFromFile(filePath)
		require.NoError(t, err)
		require.Len(t, config.Validations, 1)
		assert.Equal(t, "test", config.Validations[0].Key)
	})

	t.Run("error - file not found", func(t *testing.T) {
		_, err := LoadFromFile("/nonexistent/path/challenge.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read file")
	})

	t.Run("error - invalid YAML in file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "invalid.yaml")

		err := os.WriteFile(filePath, []byte("not: valid: yaml: ["), 0600)
		require.NoError(t, err)

		_, err = LoadFromFile(filePath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse validations")
	})
}

// TestFindLocalChallengeFile tests finding local challenge files
func TestFindLocalChallengeFile(t *testing.T) {
	t.Run("finds file in current directory", func(t *testing.T) {
		// Create temp directory structure
		tmpDir := t.TempDir()
		oldWd, _ := os.Getwd()
		defer func() {
			_ = os.Chdir(oldWd)
		}()

		// Create challenge directory
		challengeDir := filepath.Join(tmpDir, "test-challenge")
		err := os.MkdirAll(challengeDir, 0755)
		require.NoError(t, err)

		// Create challenge.yaml
		yamlPath := filepath.Join(challengeDir, "challenge.yaml")
		err = os.WriteFile(yamlPath, []byte("objectives: []"), 0600)
		require.NoError(t, err)

		// Change to temp directory
		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		// Should find the file
		found := FindLocalChallengeFile("test-challenge")
		assert.NotEmpty(t, found)
		assert.Contains(t, found, "test-challenge/challenge.yaml")
	})

	t.Run("returns empty when file not found", func(t *testing.T) {
		found := FindLocalChallengeFile("nonexistent-challenge")
		assert.Empty(t, found)
	})
}

// TestLoadFromURL_Security tests URL validation security
func TestLoadFromURL_Security(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid - correct base URL",
			url:         "https://raw.githubusercontent.com/kubeasy-dev/challenges/main/pod-evicted/challenge.yaml",
			expectError: false,
		},
		{
			name:          "invalid - wrong domain",
			url:           "https://evil.com/challenges/main/pod-evicted/challenge.yaml",
			expectError:   true,
			errorContains: "invalid URL: must be from",
		},
		{
			name:          "path traversal handled by GitHub (400 error)",
			url:           "https://raw.githubusercontent.com/kubeasy-dev/challenges/main/../../../etc/passwd",
			expectError:   true,
			errorContains: "HTTP 400", // GitHub rejects this, not our code
		},
		{
			name:          "invalid - http instead of https",
			url:           "http://raw.githubusercontent.com/kubeasy-dev/challenges/main/test/challenge.yaml",
			expectError:   true,
			errorContains: "invalid URL: must be from",
		},
		{
			name:          "invalid - subdomain manipulation",
			url:           "https://raw.githubusercontent.com.evil.com/kubeasy-dev/challenges/main/test/challenge.yaml",
			expectError:   true,
			errorContains: "invalid URL: must be from",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadFromURL(tt.url)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else if err != nil {
				// Note: This will fail with HTTP error since we're not mocking HTTP
				// But the URL validation should pass, so we check that the error is HTTP-related, not validation
				assert.NotContains(t, err.Error(), "invalid URL")
			}
		})
	}
}

// TestFindLocalChallengeFile_NoHardcodedPath verifies that when KUBEASY_LOCAL_CHALLENGES_DIR
// is unset and the challenge doesn't exist locally, FindLocalChallengeFile returns empty.
// RED: Before SAFE-03 fix, this test passes vacuously (hardcoded ~/Workspace path doesn't
// have a real challenge.yaml either). After fix, behavior is verified by _HonorsEnvVar.
func TestFindLocalChallengeFile_NoHardcodedPath(t *testing.T) {
	t.Setenv("KUBEASY_LOCAL_CHALLENGES_DIR", "")
	found := FindLocalChallengeFile("nonexistent-challenge-xyz")
	assert.Empty(t, found, "should return empty when challenge doesn't exist and env var is unset")
}

// TestFindLocalChallengeFile_HonorsEnvVar verifies that FindLocalChallengeFile uses
// KUBEASY_LOCAL_CHALLENGES_DIR env var to locate challenges.
// RED: This test FAILS because the env var lookup is not yet implemented in loader.go.
func TestFindLocalChallengeFile_HonorsEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("KUBEASY_LOCAL_CHALLENGES_DIR", tmpDir)

	// Create challenge directory and challenge.yaml inside tmpDir
	challengeDir := filepath.Join(tmpDir, "my-challenge")
	err := os.MkdirAll(challengeDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(challengeDir, "challenge.yaml"), []byte("objectives: []"), 0600)
	require.NoError(t, err)

	found := FindLocalChallengeFile("my-challenge")
	assert.NotEmpty(t, found, "should find challenge.yaml via KUBEASY_LOCAL_CHALLENGES_DIR")
	assert.Contains(t, found, "my-challenge/challenge.yaml")
}

// TestConstants tests that constants are set correctly
func TestConstants(t *testing.T) {
	assert.Equal(t, "https://raw.githubusercontent.com/kubeasy-dev/challenges/main", ChallengesRepoBaseURL)
	assert.Equal(t, 300, DefaultLogSinceSeconds)
	assert.Equal(t, 300, DefaultEventSinceSeconds)
	assert.Equal(t, 5, DefaultConnectivityTimeoutSeconds)
}

// TestParse_StatusFieldValidation tests that invalid field paths are caught at parse time
func TestParse_StatusFieldValidation(t *testing.T) {
	t.Run("valid field path for supported kind", func(t *testing.T) {
		yaml := `
objectives:
  - key: deployment-check
    type: status
    spec:
      target:
        kind: Deployment
        name: web-app
      checks:
        - field: readyReplicas
          operator: ">="
          value: 3
`
		config, err := Parse([]byte(yaml))
		require.NoError(t, err)
		require.Len(t, config.Validations, 1)
	})

	t.Run("invalid field path for supported kind", func(t *testing.T) {
		yaml := `
objectives:
  - key: deployment-check
    type: status
    spec:
      target:
        kind: Deployment
        name: web-app
      checks:
        - field: nonExistentField
          operator: "=="
          value: 3
`
		_, err := Parse([]byte(yaml))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "check 0")
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("array field path validation", func(t *testing.T) {
		yaml := `
objectives:
  - key: pod-check
    type: status
    spec:
      target:
        kind: Pod
        name: my-pod
      checks:
        - field: containerStatuses[0].restartCount
          operator: "<"
          value: 5
`
		config, err := Parse([]byte(yaml))
		require.NoError(t, err)
		require.Len(t, config.Validations, 1)
	})

	t.Run("array filter field path validation", func(t *testing.T) {
		yaml := `
objectives:
  - key: pod-check
    type: status
    spec:
      target:
        kind: Pod
        name: my-pod
      checks:
        - field: conditions[type=Ready].status
          operator: "=="
          value: "True"
`
		config, err := Parse([]byte(yaml))
		require.NoError(t, err)
		require.Len(t, config.Validations, 1)
	})

	t.Run("unsupported kind skips field validation", func(t *testing.T) {
		yaml := `
objectives:
  - key: custom-resource-check
    type: status
    spec:
      target:
        kind: CustomResource
        name: my-resource
      checks:
        - field: anyFieldPath
          operator: "=="
          value: "some-value"
`
		// Should not error - unsupported kinds skip field validation
		config, err := Parse([]byte(yaml))
		require.NoError(t, err)
		require.Len(t, config.Validations, 1)
	})
}

// TestParse_Connectivity_ExternalMode verifies that mode: external parses correctly (EXT-01)
func TestParse_Connectivity_ExternalMode(t *testing.T) {
	yaml := `
objectives:
  - key: ext-check
    type: connectivity
    spec:
      mode: external
      targets:
        - url: http://myapp.127-0-0-1.sslip.io:8080/
          expectedStatusCode: 200
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	spec := cfg.Validations[0].Spec.(ConnectivitySpec)
	assert.Equal(t, "external", spec.Mode)
	assert.Equal(t, "http://myapp.127-0-0-1.sslip.io:8080/", spec.Targets[0].URL)
}

// TestParse_Connectivity_ExternalModeWithSourcePod verifies that mode: external + sourcePod is rejected (EXT-02)
func TestParse_Connectivity_ExternalModeWithSourcePod(t *testing.T) {
	yaml := `
objectives:
  - key: ext-check
    type: connectivity
    spec:
      mode: external
      sourcePod:
        name: my-pod
      targets:
        - url: http://example.com/
          expectedStatusCode: 200
`
	_, err := Parse([]byte(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incompatible with sourcePod")
}

// TestParse_Connectivity_SslipIO verifies that sslip.io URLs parse without modification (EXT-03)
func TestParse_Connectivity_SslipIO(t *testing.T) {
	yaml := `
objectives:
  - key: sslip-check
    type: connectivity
    spec:
      mode: external
      targets:
        - url: http://myapp.127-0-0-1.sslip.io:8080/health
          expectedStatusCode: 200
          timeoutSeconds: 10
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	spec := cfg.Validations[0].Spec.(ConnectivitySpec)
	assert.Equal(t, "http://myapp.127-0-0-1.sslip.io:8080/health", spec.Targets[0].URL)
}

// TestParse_Connectivity_InvalidMode verifies that unknown mode values are rejected (EXT-01)
func TestParse_Connectivity_InvalidMode(t *testing.T) {
	yaml := `
objectives:
  - key: bad-check
    type: connectivity
    spec:
      mode: banana
      targets:
        - url: http://example.com/
          expectedStatusCode: 200
`
	_, err := Parse([]byte(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mode")
}

// TestParse_Connectivity_NoMode verifies that existing specs without mode field parse unchanged (backwards compat)
func TestParse_Connectivity_NoMode(t *testing.T) {
	yaml := `
objectives:
  - key: internal-check
    type: connectivity
    spec:
      sourcePod:
        name: my-pod
      targets:
        - url: http://service:80/
          expectedStatusCode: 200
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	spec := cfg.Validations[0].Spec.(ConnectivitySpec)
	assert.Equal(t, "", spec.Mode)
}

// TestParseConnectivityTLSBlock verifies that the tls: block in a connectivity target
// is correctly parsed into TLSConfig (TLS-01, TLS-02, TLS-03).
func TestParseConnectivityTLSBlock(t *testing.T) {
	t.Run("tls block absent - TLS field is nil (no regression)", func(t *testing.T) {
		yamlData := `
objectives:
  - key: no-tls
    type: connectivity
    spec:
      mode: external
      targets:
        - url: https://myapp.example.com/
          expectedStatusCode: 200
`
		cfg, err := Parse([]byte(yamlData))
		require.NoError(t, err)
		spec := cfg.Validations[0].Spec.(ConnectivitySpec)
		assert.Nil(t, spec.Targets[0].TLS, "TLS field must be nil when tls: block is absent")
	})

	t.Run("insecureSkipVerify: true parses correctly", func(t *testing.T) {
		yamlData := `
objectives:
  - key: skip-verify
    type: connectivity
    spec:
      mode: external
      targets:
        - url: https://myapp.example.com/
          expectedStatusCode: 200
          tls:
            insecureSkipVerify: true
`
		cfg, err := Parse([]byte(yamlData))
		require.NoError(t, err)
		spec := cfg.Validations[0].Spec.(ConnectivitySpec)
		require.NotNil(t, spec.Targets[0].TLS, "TLS field must be non-nil when tls: block is present")
		assert.True(t, spec.Targets[0].TLS.InsecureSkipVerify, "InsecureSkipVerify must be true")
		assert.False(t, spec.Targets[0].TLS.ValidateExpiry, "ValidateExpiry must be false")
		assert.False(t, spec.Targets[0].TLS.ValidateSANs, "ValidateSANs must be false")
	})

	t.Run("validateExpiry: true parses correctly", func(t *testing.T) {
		yamlData := `
objectives:
  - key: validate-expiry
    type: connectivity
    spec:
      mode: external
      targets:
        - url: https://myapp.example.com/
          expectedStatusCode: 200
          tls:
            validateExpiry: true
`
		cfg, err := Parse([]byte(yamlData))
		require.NoError(t, err)
		spec := cfg.Validations[0].Spec.(ConnectivitySpec)
		require.NotNil(t, spec.Targets[0].TLS)
		assert.False(t, spec.Targets[0].TLS.InsecureSkipVerify)
		assert.True(t, spec.Targets[0].TLS.ValidateExpiry, "ValidateExpiry must be true")
		assert.False(t, spec.Targets[0].TLS.ValidateSANs)
	})

	t.Run("validateSANs: true parses correctly", func(t *testing.T) {
		yamlData := `
objectives:
  - key: validate-sans
    type: connectivity
    spec:
      mode: external
      targets:
        - url: https://myapp.example.com/
          expectedStatusCode: 200
          tls:
            validateSANs: true
`
		cfg, err := Parse([]byte(yamlData))
		require.NoError(t, err)
		spec := cfg.Validations[0].Spec.(ConnectivitySpec)
		require.NotNil(t, spec.Targets[0].TLS)
		assert.False(t, spec.Targets[0].TLS.InsecureSkipVerify)
		assert.False(t, spec.Targets[0].TLS.ValidateExpiry)
		assert.True(t, spec.Targets[0].TLS.ValidateSANs, "ValidateSANs must be true")
	})

	t.Run("all three fields true simultaneously - rejected at parse time", func(t *testing.T) {
		// insecureSkipVerify: true is incompatible with validateExpiry/validateSANs — reject at parse.
		yamlData := `
objectives:
  - key: all-tls
    type: connectivity
    spec:
      mode: external
      targets:
        - url: https://myapp.example.com/
          expectedStatusCode: 200
          tls:
            insecureSkipVerify: true
            validateExpiry: true
            validateSANs: true
`
		_, err := Parse([]byte(yamlData))
		require.Error(t, err, "combining insecureSkipVerify with validateExpiry/validateSANs must be rejected")
		assert.Contains(t, err.Error(), "insecureSkipVerify")
	})

	t.Run("empty tls block - non-nil pointer, all bools false", func(t *testing.T) {
		yamlData := `
objectives:
  - key: empty-tls
    type: connectivity
    spec:
      mode: external
      targets:
        - url: https://myapp.example.com/
          expectedStatusCode: 200
          tls: {}
`
		cfg, err := Parse([]byte(yamlData))
		require.NoError(t, err)
		spec := cfg.Validations[0].Spec.(ConnectivitySpec)
		require.NotNil(t, spec.Targets[0].TLS, "TLS pointer must be non-nil for empty tls: {} block")
		assert.False(t, spec.Targets[0].TLS.InsecureSkipVerify, "InsecureSkipVerify must be false")
		assert.False(t, spec.Targets[0].TLS.ValidateExpiry, "ValidateExpiry must be false")
		assert.False(t, spec.Targets[0].TLS.ValidateSANs, "ValidateSANs must be false")
	})
}

// TestParse_ConnectivityProbeMode verifies that a connectivity spec with empty sourcePod
// (probe mode) is accepted by Parse without error (PROBE-01, PROBE-02).
func TestParse_ConnectivityProbeMode(t *testing.T) {
	yamlData := `
objectives:
  - key: network-blocked
    title: Connection Blocked
    description: Verify NetworkPolicy blocks traffic
    order: 1
    type: connectivity
    spec:
      sourcePod: {}
      targets:
        - url: http://my-service:80
          expectedStatusCode: 0
          timeoutSeconds: 3
`

	config, err := Parse([]byte(yamlData))
	require.NoError(t, err, "probe mode (empty sourcePod) must not be rejected by Parse")
	require.Len(t, config.Validations, 1)

	spec, ok := config.Validations[0].Spec.(ConnectivitySpec)
	require.True(t, ok, "spec should be ConnectivitySpec")
	assert.Equal(t, "", spec.SourcePod.Name, "probe mode: Name must be empty")
	assert.Empty(t, spec.SourcePod.LabelSelector, "probe mode: LabelSelector must be empty")
	assert.Equal(t, "", spec.SourcePod.Namespace, "probe mode: Namespace must be empty")
	require.Len(t, spec.Targets, 1)
	assert.Equal(t, 0, spec.Targets[0].ExpectedStatusCode)
}

// TestParse_RbacValidation tests parsing of RBAC validation spec
func TestParse_RbacValidation(t *testing.T) {
	yaml := `
objectives:
  - key: rbac-permissions
    title: RBAC Permissions
    description: ServiceAccount must have the correct permissions
    order: 1
    type: rbac
    spec:
      serviceAccount: monitoring-sa
      namespace: challenge-xyz
      checks:
        - verb: list
          resource: pods
          allowed: true
        - verb: get
          resource: configmaps
          allowed: true
        - verb: list
          resource: secrets
          allowed: false
        - verb: list
          resource: pods
          namespace: kube-system
          allowed: false
`

	config, err := Parse([]byte(yaml))
	require.NoError(t, err)
	require.Len(t, config.Validations, 1)

	v := config.Validations[0]
	assert.Equal(t, "rbac-permissions", v.Key)
	assert.Equal(t, TypeRbac, v.Type)

	spec, ok := v.Spec.(RbacSpec)
	require.True(t, ok, "spec should be RbacSpec")
	assert.Equal(t, "monitoring-sa", spec.ServiceAccount)
	assert.Equal(t, "challenge-xyz", spec.Namespace)
	require.Len(t, spec.Checks, 4)

	assert.Equal(t, "list", spec.Checks[0].Verb)
	assert.Equal(t, "pods", spec.Checks[0].Resource)
	assert.True(t, spec.Checks[0].Allowed)
	assert.Empty(t, spec.Checks[0].Namespace)

	assert.Equal(t, "list", spec.Checks[2].Verb)
	assert.Equal(t, "secrets", spec.Checks[2].Resource)
	assert.False(t, spec.Checks[2].Allowed)

	// Per-check namespace override
	assert.Equal(t, "kube-system", spec.Checks[3].Namespace)
	assert.False(t, spec.Checks[3].Allowed)
}

// TestParse_RbacValidation_Errors tests error cases for RBAC validation parsing
func TestParse_RbacValidation_Errors(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		errorContains string
	}{
		{
			name: "missing serviceAccount",
			yaml: `
objectives:
  - key: rbac-check
    type: rbac
    spec:
      namespace: challenge-xyz
      checks:
        - verb: list
          resource: pods
          allowed: true
`,
			errorContains: "serviceAccount",
		},
		{
			name: "missing namespace",
			yaml: `
objectives:
  - key: rbac-check
    type: rbac
    spec:
      serviceAccount: my-sa
      checks:
        - verb: list
          resource: pods
          allowed: true
`,
			errorContains: "namespace",
		},
		{
			name: "empty checks",
			yaml: `
objectives:
  - key: rbac-check
    type: rbac
    spec:
      serviceAccount: my-sa
      namespace: challenge-xyz
      checks: []
`,
			errorContains: "at least one check",
		},
		{
			name: "invalid verb",
			yaml: `
objectives:
  - key: rbac-check
    type: rbac
    spec:
      serviceAccount: my-sa
      namespace: challenge-xyz
      checks:
        - verb: exec
          resource: pods
          allowed: true
`,
			errorContains: "invalid verb",
		},
		{
			name: "missing resource",
			yaml: `
objectives:
  - key: rbac-check
    type: rbac
    spec:
      serviceAccount: my-sa
      namespace: challenge-xyz
      checks:
        - verb: list
          allowed: true
`,
			errorContains: "resource is required",
		},
		{
			name: "missing verb",
			yaml: `
objectives:
  - key: rbac-check
    type: rbac
    spec:
      serviceAccount: my-sa
      namespace: challenge-xyz
      checks:
        - resource: pods
          allowed: true
`,
			errorContains: "verb is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}

// TestParse_DnsValidation tests parsing of DNS validation spec
func TestParse_DnsValidation(t *testing.T) {
	input := `
objectives:
  - key: headless-dns
    title: Headless DNS
    description: Headless service DNS must resolve
    order: 1
    type: dns
    spec:
      sourcePod:
        labelSelector:
          app: client
      checks:
        - hostname: "my-service.challenge-xyz.svc.cluster.local"
          resolves: true
        - hostname: "db-0.db-headless.challenge-xyz.svc.cluster.local"
          resolves: true
        - hostname: "restricted.other-ns.svc.cluster.local"
          resolves: false
`

	config, err := Parse([]byte(input))
	require.NoError(t, err)
	require.Len(t, config.Validations, 1)

	v := config.Validations[0]
	assert.Equal(t, "headless-dns", v.Key)
	assert.Equal(t, TypeDns, v.Type)

	spec, ok := v.Spec.(DnsSpec)
	require.True(t, ok, "spec should be DnsSpec")
	assert.Equal(t, "client", spec.SourcePod.LabelSelector["app"])
	require.Len(t, spec.Checks, 3)
	assert.Equal(t, "my-service.challenge-xyz.svc.cluster.local", spec.Checks[0].Hostname)
	assert.True(t, spec.Checks[0].Resolves)
	assert.Equal(t, "db-0.db-headless.challenge-xyz.svc.cluster.local", spec.Checks[1].Hostname)
	assert.True(t, spec.Checks[1].Resolves)
	assert.Equal(t, "restricted.other-ns.svc.cluster.local", spec.Checks[2].Hostname)
	assert.False(t, spec.Checks[2].Resolves)
}

// TestParse_DnsValidation_Errors tests error cases for DNS validation parsing
func TestParse_DnsValidation_Errors(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		errorContains string
	}{
		{
			name: "missing sourcePod",
			yaml: `
objectives:
  - key: dns-check
    type: dns
    spec:
      checks:
        - hostname: "my-svc.ns.svc.cluster.local"
          resolves: true
`,
			errorContains: "sourcePod",
		},
		{
			name: "empty checks",
			yaml: `
objectives:
  - key: dns-check
    type: dns
    spec:
      sourcePod:
        name: my-pod
      checks: []
`,
			errorContains: "at least one check",
		},
		{
			name: "missing hostname",
			yaml: `
objectives:
  - key: dns-check
    type: dns
    spec:
      sourcePod:
        name: my-pod
      checks:
        - resolves: true
`,
			errorContains: "hostname is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}
