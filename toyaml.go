package toyaml

// MultilineStyle selects how a string that contains newlines is written.
type MultilineStyle int

const (
	// BlockLiteral prefers a literal block scalar ("|"), and falls back to a
	// double-quoted scalar for the strings a block cannot reproduce exactly,
	// such as one with trailing whitespace on a line.
	BlockLiteral MultilineStyle = iota

	// Quoted always writes a double-quoted scalar, carrying newlines as \n
	// the way JSON does.
	Quoted
)

// Style controls the shape of the output. Its zero value is the default: two
// spaces per level, literal blocks for multi-line strings, and sequences
// indented under their key.
//
// Style never changes the document. Whatever the settings, reading the output
// back yields the same data.
type Style struct {
	// Indent is the number of spaces per nesting level. Zero means two.
	// YAML forbids tabs in indentation, so this is a count of spaces rather
	// than the string that encoding/json takes.
	Indent int

	// Multiline selects how a string containing newlines is written.
	Multiline MultilineStyle

	// CompactSequence puts a sequence at the indentation of the mapping key
	// that introduces it, rather than one level deeper:
	//
	//	tags:          tags:
	//	  - a            vs   - a
	//
	// It applies only to a sequence that is a mapping value. A sequence
	// nested directly in another sequence always gets its own indented lines,
	// where the compact form would read as a sibling item.
	CompactSequence bool
}

// FromJSON converts a JSON document to block-style YAML with the default
// style. Empty input produces empty output.
//
// An object key longer than 1024 characters returns an *EncodeError wrapping
// ErrKeyTooLong: YAML bounds a mapping key written without the "? " indicator
// at that length, and the explicit form is not one most readers accept.
func FromJSON(src []byte) ([]byte, error) {
	return encode(src, Style{})
}

// FromJSONStyle is FromJSON with the output shape given by style.
func FromJSONStyle(src []byte, style Style) ([]byte, error) {
	return encode(src, style)
}
