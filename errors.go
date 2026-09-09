package toyaml

import (
	"errors"
	"fmt"
)

// Style validation fails before any input is read, so these are returned bare
// rather than wrapped in an *EncodeError.
var (
	// ErrNegativeIndent reports a Style.Indent below zero.
	ErrNegativeIndent = errors.New("toyaml: Style.Indent must not be negative")

	// ErrUnknownMultiline reports a Style.Multiline that is not a defined style.
	ErrUnknownMultiline = errors.New("toyaml: Style.Multiline is not a known style")
)

var (
	// ErrMaxDepth reports container nesting past the encoder's limit, which
	// bounds its recursion.
	ErrMaxDepth = errors.New("nesting too deep")

	// ErrKeyTooLong reports an object key longer than a YAML mapping key may
	// reach before its ":".
	ErrKeyTooLong = errors.New("object key too long")

	// ErrTrailingValue reports a second top-level value. A YAML document holds
	// one.
	ErrTrailingValue = errors.New("unexpected value after top-level value")
)

// EncodeError reports valid JSON that this package will not write as YAML.
// It is distinct from *jsontext.SyntacticError, which reports input that is
// not valid JSON, and carries a byte offset in the same currency so the two
// can be reported the same way.
//
// Unwrap it to test for ErrMaxDepth, ErrKeyTooLong, or ErrTrailingValue.
type EncodeError struct {
	// ByteOffset is where the offending value begins in the JSON input.
	ByteOffset int64

	// Err is the underlying error.
	Err error
}

func (e *EncodeError) Error() string {
	return fmt.Sprintf("byte offset %d: %s", e.ByteOffset, e.Err)
}

func (e *EncodeError) Unwrap() error { return e.Err }
