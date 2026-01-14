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
  - key: pod-ready
    title: Pod Ready
    description: Pod must be running
    order: 1
    type: status
    spec:
      target:
        kind: Pod
        labelSelector:
          app: test-app
      conditions:
        - type: Ready
          status: "True"
`

	config, err := Parse([]byte(yaml))
	require.NoError(t, err)
	require.Len(t, config.Validations, 1)

	v := config.Validations[0]
	assert.Equal(t, "pod-ready", v.Key)
	assert.Equal(t, "Pod Ready", v.Title)
	assert.Equal(t, TypeStatus, v.Type)

	spec, ok := v.Spec.(StatusSpec)
	require.True(t, ok, "spec should be StatusSpec")
	assert.Equal(t, "Pod", spec.Target.Kind)
	assert.Equal(t, "test-app", spec.Target.LabelSelector["app"])
	require.Len(t, spec.Conditions, 1)
	assert.Equal(t, "Ready", spec.Conditions[0].Type)
	assert.Equal(t, "True", spec.Conditions[0].Status)
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

// TestParse_MetricsValidation tests parsing of metrics validation
func TestParse_MetricsValidation(t *testing.T) {
	yaml := `
objectives:
  - key: replica-count
    type: metrics
    spec:
      target:
        kind: Deployment
        name: web-server
      checks:
        - field: spec.replicas
          operator: "=="
          value: 3
        - field: status.readyReplicas
          operator: ">="
          value: 2
`

	config, err := Parse([]byte(yaml))
	require.NoError(t, err)

	spec, ok := config.Validations[0].Spec.(MetricsSpec)
	require.True(t, ok)
	assert.Equal(t, "Deployment", spec.Target.Kind)
	assert.Equal(t, "web-server", spec.Target.Name)

	require.Len(t, spec.Checks, 2)
	assert.Equal(t, "spec.replicas", spec.Checks[0].Field)
	assert.Equal(t, "==", spec.Checks[0].Operator)
	assert.Equal(t, int64(3), spec.Checks[0].Value)

	assert.Equal(t, "status.readyReplicas", spec.Checks[1].Field)
	assert.Equal(t, ">=", spec.Checks[1].Operator)
	assert.Equal(t, int64(2), spec.Checks[1].Value)
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
    type: status
    spec:
      target:
        name: my-pod
      conditions:
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
	assert.Equal(t, TypeStatus, config.Validations[0].Type)

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
      conditions:
        - type: Ready
          status: "True"
`,
			errorContains: "target must specify either name or labelSelector",
		},
		{
			name: "sourcePod without name or labelSelector",
			yaml: `
objectives:
  - key: test
    type: connectivity
    spec:
      sourcePod:
        container: main
      targets:
        - url: http://test
          expectedStatusCode: 200
`,
			errorContains: "sourcePod must specify either name or labelSelector",
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

// TestValidateSourcePod tests the validateSourcePod function
func TestValidateSourcePod(t *testing.T) {
	tests := []struct {
		name        string
		sourcePod   SourcePod
		expectError bool
	}{
		{
			name:        "valid - with name",
			sourcePod:   SourcePod{Name: "client-pod"},
			expectError: false,
		},
		{
			name:        "valid - with labelSelector",
			sourcePod:   SourcePod{LabelSelector: map[string]string{"role": "client"}},
			expectError: false,
		},
		{
			name:        "invalid - empty",
			sourcePod:   SourcePod{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSourcePod(tt.sourcePod)
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
    type: status
    spec:
      target:
        name: test-pod
      conditions:
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

// TestConstants tests that constants are set correctly
func TestConstants(t *testing.T) {
	assert.Equal(t, "https://raw.githubusercontent.com/kubeasy-dev/challenges/main", ChallengesRepoBaseURL)
	assert.Equal(t, 300, DefaultLogSinceSeconds)
	assert.Equal(t, 300, DefaultEventSinceSeconds)
	assert.Equal(t, 5, DefaultConnectivityTimeoutSeconds)
}
