package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTypedValue_String(t *testing.T) {
	tv, err := NewTypedValue("hello")
	require.NoError(t, err)
	assert.Equal(t, ValueString, tv.Kind)
	assert.Equal(t, "hello", tv.StringVal)
}

func TestNewTypedValue_Int(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int64
	}{
		{"int", int(42), 42},
		{"int32", int32(42), 42},
		{"int64", int64(42), 42},
		{"float64 whole number", float64(42.0), 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := NewTypedValue(tt.input)
			require.NoError(t, err)
			assert.Equal(t, ValueInt, tv.Kind)
			assert.Equal(t, tt.expected, tv.IntVal)
		})
	}
}

func TestNewTypedValue_Float(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
	}{
		{"float32", float32(3.14), 3.14},
		{"float64", float64(3.14159), 3.14159},
		{"float64 with fractional part", float64(42.5), 42.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := NewTypedValue(tt.input)
			require.NoError(t, err)
			assert.Equal(t, ValueFloat, tv.Kind)
			assert.InDelta(t, tt.expected, tv.FloatVal, 0.001)
		})
	}
}

func TestNewTypedValue_Bool(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected bool
	}{
		{"true", true, true},
		{"false", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := NewTypedValue(tt.input)
			require.NoError(t, err)
			assert.Equal(t, ValueBool, tv.Kind)
			assert.Equal(t, tt.expected, tv.BoolVal)
		})
	}
}

func TestNewTypedValue_Nil(t *testing.T) {
	tv, err := NewTypedValue(nil)
	assert.Error(t, err)
	assert.Nil(t, tv)
	assert.Contains(t, err.Error(), "nil")
}

func TestNewTypedValue_UnsupportedType(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
	}{
		{"slice", []int{1, 2, 3}},
		{"map", map[string]string{"a": "b"}},
		{"struct", struct{ Name string }{"test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := NewTypedValue(tt.input)
			assert.Error(t, err)
			assert.Nil(t, tv)
			assert.Contains(t, err.Error(), "unsupported value type")
		})
	}
}

func TestTypedValue_String(t *testing.T) {
	tests := []struct {
		name     string
		tv       *TypedValue
		expected string
	}{
		{
			name:     "string value",
			tv:       &TypedValue{Kind: ValueString, StringVal: "test"},
			expected: `"test"`,
		},
		{
			name:     "int value",
			tv:       &TypedValue{Kind: ValueInt, IntVal: 42},
			expected: "42",
		},
		{
			name:     "bool true",
			tv:       &TypedValue{Kind: ValueBool, BoolVal: true},
			expected: "true",
		},
		{
			name:     "bool false",
			tv:       &TypedValue{Kind: ValueBool, BoolVal: false},
			expected: "false",
		},
		{
			name:     "float value",
			tv:       &TypedValue{Kind: ValueFloat, FloatVal: 3.14},
			expected: "3.14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.tv.String())
		})
	}
}

func TestTypedValue_Compare_StringEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		operator string
		expected bool
	}{
		{"equal strings ==", "hello", "hello", "==", true},
		{"equal strings =", "hello", "hello", "=", true},
		{"unequal strings ==", "hello", "world", "==", false},
		{"unequal strings !=", "hello", "world", "!=", true},
		{"equal strings !=", "hello", "hello", "!=", false},
		{"case sensitive", "Hello", "hello", "==", false},
		{"empty strings", "", "", "==", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tvA, _ := NewTypedValue(tt.a)
			tvB, _ := NewTypedValue(tt.b)

			result, err := tvA.Compare(tt.operator, tvB)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypedValue_Compare_StringInvalidOperator(t *testing.T) {
	tvA, _ := NewTypedValue("hello")
	tvB, _ := NewTypedValue("world")

	invalidOperators := []string{">", "<", ">=", "<="}

	for _, op := range invalidOperators {
		t.Run(op, func(t *testing.T) {
			result, err := tvA.Compare(op, tvB)
			assert.Error(t, err)
			assert.False(t, result)
			assert.Contains(t, err.Error(), "not supported for string")
		})
	}
}

func TestTypedValue_Compare_IntComparison(t *testing.T) {
	tests := []struct {
		name     string
		a        int64
		b        int64
		operator string
		expected bool
	}{
		// Equal operator
		{"5 == 5", 5, 5, "==", true},
		{"5 = 5", 5, 5, "=", true},
		{"5 == 3", 5, 3, "==", false},

		// Not equal operator
		{"5 != 3", 5, 3, "!=", true},
		{"5 != 5", 5, 5, "!=", false},

		// Greater than
		{"5 > 3", 5, 3, ">", true},
		{"3 > 5", 3, 5, ">", false},
		{"5 > 5", 5, 5, ">", false},

		// Less than
		{"3 < 5", 3, 5, "<", true},
		{"5 < 3", 5, 3, "<", false},
		{"5 < 5", 5, 5, "<", false},

		// Greater than or equal
		{"5 >= 3", 5, 3, ">=", true},
		{"5 >= 5", 5, 5, ">=", true},
		{"3 >= 5", 3, 5, ">=", false},

		// Less than or equal
		{"3 <= 5", 3, 5, "<=", true},
		{"5 <= 5", 5, 5, "<=", true},
		{"5 <= 3", 5, 3, "<=", false},

		// Edge cases
		{"zero == zero", 0, 0, "==", true},
		{"negative comparison", -5, 5, "<", true},
		{"large numbers", 9223372036854775807, 0, ">", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tvA, _ := NewTypedValue(tt.a)
			tvB, _ := NewTypedValue(tt.b)

			result, err := tvA.Compare(tt.operator, tvB)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypedValue_Compare_FloatComparison(t *testing.T) {
	tests := []struct {
		name     string
		a        float64
		b        float64
		operator string
		expected bool
	}{
		{"3.14 == 3.14", 3.14, 3.14, "==", true},
		{"3.14 != 2.71", 3.14, 2.71, "!=", true},
		{"3.14 > 2.71", 3.14, 2.71, ">", true},
		{"2.71 < 3.14", 2.71, 3.14, "<", true},
		{"3.14 >= 3.14", 3.14, 3.14, ">=", true},
		{"2.71 <= 3.14", 2.71, 3.14, "<=", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tvA, _ := NewTypedValue(tt.a)
			tvB, _ := NewTypedValue(tt.b)

			result, err := tvA.Compare(tt.operator, tvB)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypedValue_Compare_BoolComparison(t *testing.T) {
	tests := []struct {
		name     string
		a        bool
		b        bool
		operator string
		expected bool
	}{
		{"true == true", true, true, "==", true},
		{"false == false", false, false, "==", true},
		{"true == false", true, false, "==", false},
		{"true != false", true, false, "!=", true},
		{"true != true", true, true, "!=", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tvA, _ := NewTypedValue(tt.a)
			tvB, _ := NewTypedValue(tt.b)

			result, err := tvA.Compare(tt.operator, tvB)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypedValue_Compare_BoolInvalidOperator(t *testing.T) {
	tvA, _ := NewTypedValue(true)
	tvB, _ := NewTypedValue(false)

	invalidOperators := []string{">", "<", ">=", "<="}

	for _, op := range invalidOperators {
		t.Run(op, func(t *testing.T) {
			result, err := tvA.Compare(op, tvB)
			assert.Error(t, err)
			assert.False(t, result)
			assert.Contains(t, err.Error(), "not supported for boolean")
		})
	}
}

func TestTypedValue_Compare_IntFloatCoercion(t *testing.T) {
	tests := []struct {
		name     string
		intVal   int64
		floatVal float64
		operator string
		expected bool
	}{
		{"int 5 == float 5.0", 5, 5.0, "==", true},
		{"int 5 < float 5.5", 5, 5.5, "<", true},
		{"int 6 > float 5.5", 6, 5.5, ">", true},
		{"int 5 != float 5.5", 5, 5.5, "!=", true},
		{"int 5 >= float 4.9", 5, 4.9, ">=", true},
		{"int 5 <= float 5.1", 5, 5.1, "<=", true},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (int first)", func(t *testing.T) {
			tvInt, _ := NewTypedValue(tt.intVal)
			tvFloat, _ := NewTypedValue(tt.floatVal)

			result, err := tvInt.Compare(tt.operator, tvFloat)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})

		// Also test with float as first operand
		t.Run(tt.name+" (float first)", func(t *testing.T) {
			tvInt, _ := NewTypedValue(tt.intVal)
			tvFloat, _ := NewTypedValue(tt.floatVal)

			// Reverse the comparison (and operator if needed)
			reverseOp := reverseOperator(tt.operator)
			result, err := tvFloat.Compare(reverseOp, tvInt)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func reverseOperator(op string) string {
	switch op {
	case ">":
		return "<"
	case "<":
		return ">"
	case ">=":
		return "<="
	case "<=":
		return ">="
	default:
		return op // ==, != are symmetric
	}
}

func TestTypedValue_Compare_TypeMismatch(t *testing.T) {
	tests := []struct {
		name    string
		aType   interface{}
		bType   interface{}
		errText string
	}{
		{"string vs bool", "hello", true, "type mismatch"},
		{"bool vs string", true, "hello", "type mismatch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tvA, _ := NewTypedValue(tt.aType)
			tvB, _ := NewTypedValue(tt.bType)

			result, err := tvA.Compare("==", tvB)
			assert.Error(t, err)
			assert.False(t, result)
			assert.Contains(t, err.Error(), tt.errText)
		})
	}
}

func TestTypedValue_Compare_NilOther(t *testing.T) {
	tv, _ := NewTypedValue("hello")

	result, err := tv.Compare("==", nil)
	assert.Error(t, err)
	assert.False(t, result)
	assert.Contains(t, err.Error(), "nil")
}

func TestTypedValue_Compare_InvalidOperator(t *testing.T) {
	tvA, _ := NewTypedValue(5)
	tvB, _ := NewTypedValue(3)

	result, err := tvA.Compare("~=", tvB)
	assert.Error(t, err)
	assert.False(t, result)
	assert.Contains(t, err.Error(), "invalid operator")
}

func TestTypedValue_Value(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"string", "hello", "hello"},
		{"int", int64(42), int64(42)},
		{"bool", true, true},
		{"float", float64(3.14), float64(3.14)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, _ := NewTypedValue(tt.input)
			assert.Equal(t, tt.expected, tv.Value())
		})
	}
}

func TestValueKind_String(t *testing.T) {
	tests := []struct {
		kind     ValueKind
		expected string
	}{
		{ValueString, "string"},
		{ValueInt, "int"},
		{ValueBool, "bool"},
		{ValueFloat, "float"},
		{ValueKind(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.kind.String())
		})
	}
}

func TestTypedValue_isNumeric(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"int is numeric", int64(42), true},
		{"float is numeric", float64(3.14), true},
		{"string is not numeric", "hello", false},
		{"bool is not numeric", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, _ := NewTypedValue(tt.input)
			assert.Equal(t, tt.expected, tv.isNumeric())
		})
	}
}

// Test realistic Kubernetes status field scenarios
func TestTypedValue_KubernetesScenarios(t *testing.T) {
	t.Run("Pod condition status comparison", func(t *testing.T) {
		// Pod conditions use string values "True", "False", "Unknown"
		actual, _ := NewTypedValue("True")
		expected, _ := NewTypedValue("True")

		result, err := actual.Compare("==", expected)
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("Deployment replicas comparison", func(t *testing.T) {
		// Replicas are int32 in Kubernetes but often come as int64 from YAML
		actual, _ := NewTypedValue(int32(3))
		expected, _ := NewTypedValue(int64(3))

		result, err := actual.Compare(">=", expected)
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("Container restart count check", func(t *testing.T) {
		// restartCount should be less than threshold
		actual, _ := NewTypedValue(int32(2))
		threshold, _ := NewTypedValue(int64(5))

		result, err := actual.Compare("<", threshold)
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("Float comparison from YAML", func(t *testing.T) {
		// YAML sometimes parses whole numbers as float64
		actual, _ := NewTypedValue(float64(3.0))
		expected, _ := NewTypedValue(int64(3))

		result, err := actual.Compare("==", expected)
		require.NoError(t, err)
		assert.True(t, result)
	})
}
