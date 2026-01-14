package fieldpath

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_SimpleFields(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectedTokens []PathToken
	}{
		{
			name: "single field",
			path: "readyReplicas",
			expectedTokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "readyReplicas"},
			},
		},
		{
			name: "nested field",
			path: "phase.current",
			expectedTokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "phase"},
				FieldToken{Name: "current"},
			},
		},
		{
			name: "deeply nested field",
			path: "conditions.lastTransitionTime",
			expectedTokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "conditions"},
				FieldToken{Name: "lastTransitionTime"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := Parse(tt.path)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTokens, tokens)
		})
	}
}

func TestParse_ArrayIndex(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectedTokens []PathToken
	}{
		{
			name: "array index single",
			path: "containerStatuses[0]",
			expectedTokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "containerStatuses"},
				ArrayIndexToken{Index: 0},
			},
		},
		{
			name: "array index with nested field",
			path: "containerStatuses[0].restartCount",
			expectedTokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "containerStatuses"},
				ArrayIndexToken{Index: 0},
				FieldToken{Name: "restartCount"},
			},
		},
		{
			name: "array index with deep nesting",
			path: "containerStatuses[1].state.waiting.reason",
			expectedTokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "containerStatuses"},
				ArrayIndexToken{Index: 1},
				FieldToken{Name: "state"},
				FieldToken{Name: "waiting"},
				FieldToken{Name: "reason"},
			},
		},
		{
			name: "multiple array indices",
			path: "items[0].subItems[2]",
			expectedTokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "items"},
				ArrayIndexToken{Index: 0},
				FieldToken{Name: "subItems"},
				ArrayIndexToken{Index: 2},
			},
		},
		{
			name: "array index chained",
			path: "matrix[0][1]",
			expectedTokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "matrix"},
				ArrayIndexToken{Index: 0},
				ArrayIndexToken{Index: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := Parse(tt.path)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTokens, tokens)
		})
	}
}

func TestParse_ArrayFilter(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectedTokens []PathToken
	}{
		{
			name: "array filter simple",
			path: "conditions[type=Ready]",
			expectedTokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "conditions"},
				ArrayFilterToken{FilterField: "type", FilterValue: "Ready"},
			},
		},
		{
			name: "array filter with nested field",
			path: "conditions[type=Ready].status",
			expectedTokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "conditions"},
				ArrayFilterToken{FilterField: "type", FilterValue: "Ready"},
				FieldToken{Name: "status"},
			},
		},
		{
			name: "array filter with complex value",
			path: "conditions[type=ContainersReady].lastTransitionTime",
			expectedTokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "conditions"},
				ArrayFilterToken{FilterField: "type", FilterValue: "ContainersReady"},
				FieldToken{Name: "lastTransitionTime"},
			},
		},
		{
			name: "array filter with value containing special chars",
			path: "annotations[key=app.kubernetes.io/name].value",
			expectedTokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "annotations"},
				ArrayFilterToken{FilterField: "key", FilterValue: "app.kubernetes.io/name"},
				FieldToken{Name: "value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := Parse(tt.path)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTokens, tokens)
		})
	}
}

func TestParse_ComplexPaths(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectedTokens []PathToken
	}{
		{
			name: "filter then index",
			path: "items[name=test].subItems[0]",
			expectedTokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "items"},
				ArrayFilterToken{FilterField: "name", FilterValue: "test"},
				FieldToken{Name: "subItems"},
				ArrayIndexToken{Index: 0},
			},
		},
		{
			name: "index then filter",
			path: "pods[0].conditions[type=Ready].status",
			expectedTokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "pods"},
				ArrayIndexToken{Index: 0},
				FieldToken{Name: "conditions"},
				ArrayFilterToken{FilterField: "type", FilterValue: "Ready"},
				FieldToken{Name: "status"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := Parse(tt.path)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTokens, tokens)
		})
	}
}

func TestParse_ErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		errContains string
	}{
		{
			name:        "empty path",
			path:        "",
			errContains: "cannot be empty",
		},
		{
			name:        "empty segment",
			path:        "field..nested",
			errContains: "empty segment",
		},
		{
			name:        "invalid field name starting with number",
			path:        "9field",
			errContains: "invalid field name",
		},
		{
			name:        "invalid field name with special chars",
			path:        "field-name",
			errContains: "invalid field name",
		},
		{
			name:        "negative array index",
			path:        "items[-1]",
			errContains: "must be non-negative",
		},
		{
			name:        "invalid array filter missing equals",
			path:        "conditions[Ready]",
			errContains: "invalid array accessor",
		},
		{
			name:        "invalid array filter missing value",
			path:        "conditions[type=]",
			errContains: "invalid array accessor",
		},
		{
			name:        "unclosed bracket",
			path:        "items[0",
			errContains: "invalid field name",
		},
		{
			name:        "empty brackets",
			path:        "items[]",
			errContains: "invalid array accessor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := Parse(tt.path)
			assert.Error(t, err)
			assert.Nil(t, tokens)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestParse_AutoPrefixing(t *testing.T) {
	// Verify that all paths are automatically prefixed with "status."
	tests := []struct {
		name       string
		path       string
		firstToken FieldToken
	}{
		{
			name:       "simple field gets status prefix",
			path:       "replicas",
			firstToken: FieldToken{Name: "status"},
		},
		{
			name:       "nested field gets status prefix",
			path:       "phase.current",
			firstToken: FieldToken{Name: "status"},
		},
		{
			name:       "array access gets status prefix",
			path:       "containerStatuses[0].name",
			firstToken: FieldToken{Name: "status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := Parse(tt.path)
			require.NoError(t, err)
			require.NotEmpty(t, tokens)
			assert.Equal(t, tt.firstToken, tokens[0])
		})
	}
}

func TestParseSegment(t *testing.T) {
	tests := []struct {
		name           string
		segment        string
		expectedTokens []PathToken
	}{
		{
			name:    "simple field",
			segment: "field",
			expectedTokens: []PathToken{
				FieldToken{Name: "field"},
			},
		},
		{
			name:    "field with single index",
			segment: "field[0]",
			expectedTokens: []PathToken{
				FieldToken{Name: "field"},
				ArrayIndexToken{Index: 0},
			},
		},
		{
			name:    "field with multiple indices",
			segment: "field[0][1]",
			expectedTokens: []PathToken{
				FieldToken{Name: "field"},
				ArrayIndexToken{Index: 0},
				ArrayIndexToken{Index: 1},
			},
		},
		{
			name:    "field with filter",
			segment: "field[key=value]",
			expectedTokens: []PathToken{
				FieldToken{Name: "field"},
				ArrayFilterToken{FilterField: "key", FilterValue: "value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := parseSegment(tt.segment, 0, "test")
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTokens, tokens)
		})
	}
}

func TestIsValidFieldName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"field", true},
		{"Field", true},
		{"field123", true},
		{"field123ABC", true},
		{"123field", false},
		{"field-name", false},
		{"field_name", false},
		{"field.name", false},
		{"", false},
		{"field name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidFieldName(tt.name)
			assert.Equal(t, tt.expected, result)
		})
	}
}
