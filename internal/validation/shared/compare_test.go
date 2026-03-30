package shared_test

import (
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetNestedInt64(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		fields   []string
		expected int64
		found    bool
		wantErr  bool
	}{
		{
			name:     "int64 value",
			obj:      map[string]interface{}{"status": map[string]interface{}{"readyReplicas": int64(5)}},
			fields:   []string{"status", "readyReplicas"},
			expected: 5,
			found:    true,
		},
		{
			name:     "int32 value",
			obj:      map[string]interface{}{"status": map[string]interface{}{"readyReplicas": int32(3)}},
			fields:   []string{"status", "readyReplicas"},
			expected: 3,
			found:    true,
		},
		{
			name:     "int value",
			obj:      map[string]interface{}{"count": 10},
			fields:   []string{"count"},
			expected: 10,
			found:    true,
		},
		{
			name:     "float64 value",
			obj:      map[string]interface{}{"value": float64(7.5)},
			fields:   []string{"value"},
			expected: 7,
			found:    true,
		},
		{
			name:   "field not found",
			obj:    map[string]interface{}{"other": "value"},
			fields: []string{"missing"},
			found:  false,
		},
		{
			name:    "invalid type",
			obj:     map[string]interface{}{"value": "string"},
			fields:  []string{"value"},
			found:   true,
			wantErr: true,
		},
		{
			name:     "nested int value",
			obj:      map[string]interface{}{"status": map[string]interface{}{"count": int32(10)}},
			fields:   []string{"status", "count"},
			expected: 10,
			found:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found, err := shared.GetNestedInt64(tt.obj, tt.fields...)
			if tt.wantErr {
				assert.Error(t, err)
			} else if tt.found {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, value)
			}
			assert.Equal(t, tt.found, found)
		})
	}
}

func TestCompareValues(t *testing.T) {
	tests := []struct {
		name     string
		actual   int64
		operator string
		expected int64
		want     bool
	}{
		{"equal ==", 5, "==", 5, true},
		{"equal =", 5, "=", 5, true},
		{"not equal !=", 5, "!=", 3, true},
		{"greater than >", 10, ">", 5, true},
		{"less than <", 3, "<", 5, true},
		{"greater or equal >=", 5, ">=", 5, true},
		{"less or equal <=", 5, "<=", 5, true},
		{"equal fails", 5, "==", 3, false},
		{"greater fails", 3, ">", 5, false},
		{"unknown operator returns false", 5, "unknown", 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shared.CompareValues(tt.actual, tt.operator, tt.expected)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestCompareTypedValues(t *testing.T) {
	tests := []struct {
		name     string
		actual   interface{}
		operator string
		expected interface{}
		want     bool
		wantErr  bool
	}{
		{"string equal", "hello", "==", "hello", true, false},
		{"string not equal", "hello", "!=", "world", true, false},
		{"string operator mismatch", "hello", "==", 42, false, true},
		{"bool equal true", true, "==", true, true, false},
		{"bool not equal", true, "!=", false, true, false},
		{"bool unsupported operator", true, ">", false, false, true},
		{"int64 greater", int64(10), ">", int64(5), true, false},
		{"float64 less", float64(3.0), "<", float64(5.0), true, false},
		{"nil actual", nil, "==", "hello", false, true},
		{"unsupported type", []string{"a"}, "==", "a", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := shared.CompareTypedValues(tt.actual, tt.operator, tt.expected)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}
