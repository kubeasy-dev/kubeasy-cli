package fieldpath

// TokenType represents the type of a field path token
type TokenType int

const (
	// TokenField represents a simple field access (e.g., "readyReplicas")
	TokenField TokenType = iota
	// TokenArrayIndex represents array access by index (e.g., "[0]")
	TokenArrayIndex
	// TokenArrayFilter represents array access by filter (e.g., "[type=Ready]")
	TokenArrayFilter
)

// PathToken is the interface implemented by all token types
type PathToken interface {
	Type() TokenType
}

// FieldToken represents a field access in a path
type FieldToken struct {
	Name string
}

// Type returns the token type
func (t FieldToken) Type() TokenType {
	return TokenField
}

// ArrayIndexToken represents an array access by numeric index
type ArrayIndexToken struct {
	Index int
}

// Type returns the token type
func (t ArrayIndexToken) Type() TokenType {
	return TokenArrayIndex
}

// ArrayFilterToken represents an array access by field=value filter
type ArrayFilterToken struct {
	FilterField string
	FilterValue string
}

// Type returns the token type
func (t ArrayFilterToken) Type() TokenType {
	return TokenArrayFilter
}
