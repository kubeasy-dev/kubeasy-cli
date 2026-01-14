package fieldpath

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_SimpleFields(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"replicas":      int64(3),
			"readyReplicas": int64(3),
			"phase":         "Running",
		},
	}

	tests := []struct {
		name          string
		tokens        []PathToken
		expectedValue interface{}
		expectedFound bool
		expectError   bool
	}{
		{
			name: "simple field access",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "replicas"},
			},
			expectedValue: int64(3),
			expectedFound: true,
		},
		{
			name: "nested field access",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "readyReplicas"},
			},
			expectedValue: int64(3),
			expectedFound: true,
		},
		{
			name: "field not found",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "nonExistent"},
			},
			expectedValue: nil,
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found, err := Resolve(obj, tt.tokens)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedFound, found)
			assert.Equal(t, tt.expectedValue, value)
		})
	}
}

func TestResolve_NestedFields(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"config": map[string]interface{}{
				"settings": map[string]interface{}{
					"enabled": true,
					"value":   "test",
				},
			},
		},
	}

	tests := []struct {
		name          string
		tokens        []PathToken
		expectedValue interface{}
		expectedFound bool
	}{
		{
			name: "deeply nested field",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "config"},
				FieldToken{Name: "settings"},
				FieldToken{Name: "enabled"},
			},
			expectedValue: true,
			expectedFound: true,
		},
		{
			name: "nested map access",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "config"},
				FieldToken{Name: "settings"},
			},
			expectedValue: map[string]interface{}{
				"enabled": true,
				"value":   "test",
			},
			expectedFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found, err := Resolve(obj, tt.tokens)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedFound, found)
			assert.Equal(t, tt.expectedValue, value)
		})
	}
}

func TestResolve_ArrayIndex(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"containerStatuses": []interface{}{
				map[string]interface{}{
					"name":         "container1",
					"restartCount": int64(0),
				},
				map[string]interface{}{
					"name":         "container2",
					"restartCount": int64(2),
				},
				map[string]interface{}{
					"name":         "container3",
					"restartCount": int64(1),
				},
			},
		},
	}

	tests := []struct {
		name          string
		tokens        []PathToken
		expectedValue interface{}
		expectedFound bool
		expectError   bool
		errContains   string
	}{
		{
			name: "array index first element",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "containerStatuses"},
				ArrayIndexToken{Index: 0},
				FieldToken{Name: "name"},
			},
			expectedValue: "container1",
			expectedFound: true,
		},
		{
			name: "array index middle element",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "containerStatuses"},
				ArrayIndexToken{Index: 1},
				FieldToken{Name: "restartCount"},
			},
			expectedValue: int64(2),
			expectedFound: true,
		},
		{
			name: "array index last element",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "containerStatuses"},
				ArrayIndexToken{Index: 2},
			},
			expectedValue: map[string]interface{}{
				"name":         "container3",
				"restartCount": int64(1),
			},
			expectedFound: true,
		},
		{
			name: "array index out of bounds",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "containerStatuses"},
				ArrayIndexToken{Index: 5},
			},
			expectError: true,
			errContains: "out of bounds",
		},
		{
			name: "array index on non-array",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				ArrayIndexToken{Index: 0},
			},
			expectError: true,
			errContains: "expected array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found, err := Resolve(obj, tt.tokens)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedFound, found)
				assert.Equal(t, tt.expectedValue, value)
			}
		})
	}
}

func TestResolve_ArrayFilter(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":   "Ready",
					"status": "True",
					"reason": "AllReady",
				},
				map[string]interface{}{
					"type":   "ContainersReady",
					"status": "True",
					"reason": "AllReady",
				},
				map[string]interface{}{
					"type":   "Initialized",
					"status": "True",
					"reason": "PodCompleted",
				},
			},
		},
	}

	tests := []struct {
		name          string
		tokens        []PathToken
		expectedValue interface{}
		expectedFound bool
		expectError   bool
		errContains   string
	}{
		{
			name: "filter finds first match",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "conditions"},
				ArrayFilterToken{FilterField: "type", FilterValue: "Ready"},
				FieldToken{Name: "status"},
			},
			expectedValue: "True",
			expectedFound: true,
		},
		{
			name: "filter finds middle match",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "conditions"},
				ArrayFilterToken{FilterField: "type", FilterValue: "ContainersReady"},
				FieldToken{Name: "reason"},
			},
			expectedValue: "AllReady",
			expectedFound: true,
		},
		{
			name: "filter finds last match",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "conditions"},
				ArrayFilterToken{FilterField: "type", FilterValue: "Initialized"},
			},
			expectedValue: map[string]interface{}{
				"type":   "Initialized",
				"status": "True",
				"reason": "PodCompleted",
			},
			expectedFound: true,
		},
		{
			name: "filter no match",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "conditions"},
				ArrayFilterToken{FilterField: "type", FilterValue: "NotFound"},
			},
			expectError: true,
			errContains: "no array element found",
		},
		{
			name: "filter on non-array",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				ArrayFilterToken{FilterField: "type", FilterValue: "Ready"},
			},
			expectError: true,
			errContains: "expected array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found, err := Resolve(obj, tt.tokens)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedFound, found)
				assert.Equal(t, tt.expectedValue, value)
			}
		})
	}
}

func TestResolve_ComplexPaths(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"containerStatuses": []interface{}{
				map[string]interface{}{
					"name": "container1",
					"state": map[string]interface{}{
						"waiting": map[string]interface{}{
							"reason":  "ImagePullBackOff",
							"message": "Back-off pulling image",
						},
					},
				},
				map[string]interface{}{
					"name": "container2",
					"state": map[string]interface{}{
						"running": map[string]interface{}{
							"startedAt": "2024-01-01T00:00:00Z",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		tokens        []PathToken
		expectedValue interface{}
		expectedFound bool
	}{
		{
			name: "deeply nested with array index",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "containerStatuses"},
				ArrayIndexToken{Index: 0},
				FieldToken{Name: "state"},
				FieldToken{Name: "waiting"},
				FieldToken{Name: "reason"},
			},
			expectedValue: "ImagePullBackOff",
			expectedFound: true,
		},
		{
			name: "multiple arrays and objects",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "containerStatuses"},
				ArrayIndexToken{Index: 1},
				FieldToken{Name: "state"},
				FieldToken{Name: "running"},
				FieldToken{Name: "startedAt"},
			},
			expectedValue: "2024-01-01T00:00:00Z",
			expectedFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found, err := Resolve(obj, tt.tokens)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedFound, found)
			assert.Equal(t, tt.expectedValue, value)
		})
	}
}

func TestResolve_CaseInsensitivity(t *testing.T) {
	// Test that resolver handles both lowercase (JSON) and uppercase (Go struct) field names
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"readyReplicas": int64(3), // lowercase first letter (JSON)
		},
	}

	tests := []struct {
		name          string
		tokens        []PathToken
		expectedValue interface{}
		expectedFound bool
	}{
		{
			name: "lowercase field name",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "readyReplicas"},
			},
			expectedValue: int64(3),
			expectedFound: true,
		},
		{
			name: "uppercase field name (should find via capitalization)",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "ReadyReplicas"},
			},
			expectedValue: int64(3),
			expectedFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found, err := Resolve(obj, tt.tokens)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedFound, found)
			assert.Equal(t, tt.expectedValue, value)
		})
	}
}

func TestResolve_ErrorCases(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"replicas": int64(3),
		},
	}

	tests := []struct {
		name        string
		tokens      []PathToken
		errContains string
	}{
		{
			name:        "empty tokens",
			tokens:      []PathToken{},
			errContains: "no tokens",
		},
		{
			name: "field access on non-map",
			tokens: []PathToken{
				FieldToken{Name: "status"},
				FieldToken{Name: "replicas"},
				FieldToken{Name: "nested"}, // replicas is int64, not map
			},
			errContains: "expected map",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found, err := Resolve(obj, tt.tokens)
			assert.Error(t, err)
			assert.False(t, found)
			assert.Nil(t, value)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestGet_ConvenienceFunction(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"replicas":      int64(3),
			"readyReplicas": int64(3),
			"containerStatuses": []interface{}{
				map[string]interface{}{
					"name":         "container1",
					"restartCount": int64(0),
				},
			},
			"conditions": []interface{}{
				map[string]interface{}{
					"type":   "Ready",
					"status": "True",
				},
			},
		},
	}

	tests := []struct {
		name          string
		path          string
		expectedValue interface{}
		expectedFound bool
		expectError   bool
	}{
		{
			name:          "simple field",
			path:          "replicas",
			expectedValue: int64(3),
			expectedFound: true,
		},
		{
			name:          "array index",
			path:          "containerStatuses[0].name",
			expectedValue: "container1",
			expectedFound: true,
		},
		{
			name:          "array filter",
			path:          "conditions[type=Ready].status",
			expectedValue: "True",
			expectedFound: true,
		},
		{
			name:        "invalid path",
			path:        "invalid[",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found, err := Get(obj, tt.path)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedFound, found)
				assert.Equal(t, tt.expectedValue, value)
			}
		})
	}
}

func TestGetAvailableFilterValues(t *testing.T) {
	tests := []struct {
		name        string
		slice       []interface{}
		filterField string
		expected    []string
	}{
		{
			name: "multiple unique values",
			slice: []interface{}{
				map[string]interface{}{"type": "Ready"},
				map[string]interface{}{"type": "ContainersReady"},
				map[string]interface{}{"type": "Initialized"},
			},
			filterField: "type",
			expected:    []string{"Ready", "ContainersReady", "Initialized"},
		},
		{
			name: "duplicate values",
			slice: []interface{}{
				map[string]interface{}{"type": "Ready"},
				map[string]interface{}{"type": "Ready"},
			},
			filterField: "type",
			expected:    []string{"Ready"},
		},
		{
			name:        "empty slice",
			slice:       []interface{}{},
			filterField: "type",
			expected:    []string{"none"},
		},
		{
			name: "non-map elements",
			slice: []interface{}{
				"string",
				123,
			},
			filterField: "type",
			expected:    []string{"none"},
		},
		{
			name: "field not present",
			slice: []interface{}{
				map[string]interface{}{"other": "value"},
			},
			filterField: "type",
			expected:    []string{"none"},
		},
		{
			name:        "nil slice",
			slice:       nil,
			filterField: "type",
			expected:    []string{"none"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getAvailableFilterValues(tt.slice, tt.filterField)
			// Check that all expected values are present (order doesn't matter)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "Hello"},
		{"Hello", "Hello"},
		{"h", "H"},
		{"", ""},
		{"hELLO", "HELLO"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := capitalizeFirst(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLowercaseFirst(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello", "hello"},
		{"hello", "hello"},
		{"H", "h"},
		{"", ""},
		{"HELLO", "hELLO"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := lowercaseFirst(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolve_MultipleArrayFilters(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"pods": []interface{}{
				map[string]interface{}{
					"name": "pod1",
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Ready",
							"status": "True",
						},
					},
				},
			},
		},
	}

	tokens := []PathToken{
		FieldToken{Name: "status"},
		FieldToken{Name: "pods"},
		ArrayIndexToken{Index: 0},
		FieldToken{Name: "conditions"},
		ArrayFilterToken{FilterField: "type", FilterValue: "Ready"},
		FieldToken{Name: "status"},
	}

	value, found, err := Resolve(obj, tokens)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "True", value)
}
