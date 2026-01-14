package validation

import (
	"fmt"
	"math"
)

// ValueKind represents the type of a TypedValue
type ValueKind int

const (
	// ValueString represents a string value
	ValueString ValueKind = iota
	// ValueInt represents an int64 value
	ValueInt
	// ValueBool represents a boolean value
	ValueBool
	// ValueFloat represents a float64 value
	ValueFloat
)

// String returns a human-readable representation of the ValueKind
func (k ValueKind) String() string {
	switch k {
	case ValueString:
		return "string"
	case ValueInt:
		return "int"
	case ValueBool:
		return "bool"
	case ValueFloat:
		return "float"
	default:
		return "unknown"
	}
}

// TypedValue represents a value with its type information
// This enables type-safe comparisons between values from YAML and Kubernetes objects
type TypedValue struct {
	Kind      ValueKind
	StringVal string
	IntVal    int64
	BoolVal   bool
	FloatVal  float64
}

// NewTypedValue converts an interface{} value (typically from YAML or unstructured K8s objects)
// to a TypedValue. Returns an error if the value type is not supported.
func NewTypedValue(v interface{}) (*TypedValue, error) {
	if v == nil {
		return nil, fmt.Errorf("cannot create TypedValue from nil")
	}

	tv := &TypedValue{}

	switch val := v.(type) {
	case string:
		tv.Kind = ValueString
		tv.StringVal = val
	case int:
		tv.Kind = ValueInt
		tv.IntVal = int64(val)
	case int32:
		tv.Kind = ValueInt
		tv.IntVal = int64(val)
	case int64:
		tv.Kind = ValueInt
		tv.IntVal = val
	case float32:
		tv.Kind = ValueFloat
		tv.FloatVal = float64(val)
	case float64:
		// Check if this is actually an integer (YAML sometimes parses ints as floats)
		if val == math.Trunc(val) && val >= math.MinInt64 && val <= math.MaxInt64 {
			tv.Kind = ValueInt
			tv.IntVal = int64(val)
		} else {
			tv.Kind = ValueFloat
			tv.FloatVal = val
		}
	case bool:
		tv.Kind = ValueBool
		tv.BoolVal = val
	default:
		return nil, fmt.Errorf("unsupported value type: %T", v)
	}

	return tv, nil
}

// String returns a human-readable representation of the TypedValue
func (tv *TypedValue) String() string {
	switch tv.Kind {
	case ValueString:
		return fmt.Sprintf("%q", tv.StringVal)
	case ValueInt:
		return fmt.Sprintf("%d", tv.IntVal)
	case ValueBool:
		return fmt.Sprintf("%t", tv.BoolVal)
	case ValueFloat:
		return fmt.Sprintf("%g", tv.FloatVal)
	default:
		return "<unknown>"
	}
}

// Compare compares this TypedValue with another using the given operator.
// Supported operators: "==", "!=", ">", "<", ">=", "<="
// String and bool types only support "==" and "!="
// Numeric types (int, float) support all operators and allow int/float coercion
func (tv *TypedValue) Compare(operator string, other *TypedValue) (bool, error) {
	if other == nil {
		return false, fmt.Errorf("cannot compare with nil value")
	}

	// Handle numeric coercion: int can be compared with float
	if tv.isNumeric() && other.isNumeric() {
		return tv.compareNumeric(operator, other)
	}

	// For non-numeric types, kinds must match
	if tv.Kind != other.Kind {
		return false, fmt.Errorf("type mismatch: cannot compare %s with %s", tv.Kind, other.Kind)
	}

	switch tv.Kind {
	case ValueString:
		return tv.compareString(operator, other)
	case ValueBool:
		return tv.compareBool(operator, other)
	default:
		return false, fmt.Errorf("unsupported type for comparison: %s", tv.Kind)
	}
}

// isNumeric returns true if the TypedValue is a numeric type (int or float)
func (tv *TypedValue) isNumeric() bool {
	return tv.Kind == ValueInt || tv.Kind == ValueFloat
}

// compareString compares two string values
// Only supports "==" and "!=" operators
func (tv *TypedValue) compareString(operator string, other *TypedValue) (bool, error) {
	switch operator {
	case "==", "=":
		return tv.StringVal == other.StringVal, nil
	case "!=":
		return tv.StringVal != other.StringVal, nil
	default:
		return false, fmt.Errorf("operator %q not supported for string comparison (use == or !=)", operator)
	}
}

// compareBool compares two boolean values
// Only supports "==" and "!=" operators
func (tv *TypedValue) compareBool(operator string, other *TypedValue) (bool, error) {
	switch operator {
	case "==", "=":
		return tv.BoolVal == other.BoolVal, nil
	case "!=":
		return tv.BoolVal != other.BoolVal, nil
	default:
		return false, fmt.Errorf("operator %q not supported for boolean comparison (use == or !=)", operator)
	}
}

// compareNumeric compares two numeric values (int64 or float64)
// Supports all comparison operators: "==", "!=", ">", "<", ">=", "<="
// Handles int/float coercion by converting to float64 for comparison
//
// Note on float equality: This uses direct float comparison (==) which can be
// problematic for computed values due to floating-point precision. However, this
// is acceptable for Kubernetes validation because values come from YAML specs or
// resource status fields (not calculations). If epsilon-based comparison becomes
// necessary, it can be added here without changing the public API.
func (tv *TypedValue) compareNumeric(operator string, other *TypedValue) (bool, error) {
	// Convert both to float64 for comparison
	var a, b float64

	if tv.Kind == ValueFloat {
		a = tv.FloatVal
	} else {
		a = float64(tv.IntVal)
	}

	if other.Kind == ValueFloat {
		b = other.FloatVal
	} else {
		b = float64(other.IntVal)
	}

	switch operator {
	case "==", "=":
		return a == b, nil
	case "!=":
		return a != b, nil
	case ">":
		return a > b, nil
	case "<":
		return a < b, nil
	case ">=":
		return a >= b, nil
	case "<=":
		return a <= b, nil
	default:
		return false, fmt.Errorf("invalid operator %q (valid: ==, !=, >, <, >=, <=)", operator)
	}
}

// Value returns the underlying value as an interface{}
func (tv *TypedValue) Value() interface{} {
	switch tv.Kind {
	case ValueString:
		return tv.StringVal
	case ValueInt:
		return tv.IntVal
	case ValueBool:
		return tv.BoolVal
	case ValueFloat:
		return tv.FloatVal
	default:
		return nil
	}
}
