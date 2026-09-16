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

// QuoteStyle selects the quotes put around a string that cannot be written
// plain. It applies only where the choice is free: a single-quoted scalar
// escapes nothing but a quote of its own, so a string holding a line break, or
// a character YAML has no literal spelling for, is double-quoted whatever the
// setting.
type QuoteStyle int

const (
	// QuoteAdaptive writes whichever form escapes less, and single quotes on a
	// tie, since the two spellings are then the same length and the lighter one
	// reads better. So a string carrying a backslash or a double quote comes out
	// single-quoted, and one carrying an apostrophe double-quoted.
	QuoteAdaptive QuoteStyle = iota

	// QuoteDouble writes the JSON spelling of the string, which is already
	// valid inside a YAML double-quoted scalar.
	QuoteDouble

	// QuoteSingle writes a single-quoted scalar wherever one can hold the string
	// as itself, doubling any quote in it. Nothing else is escaped, so a
	// backslash and a double quote come out literal.
	QuoteSingle
)

// SequenceSpacing selects which items of a sequence are set off from their
// neighbours by a blank line. It is one value rather than a switch and a
// modifier because the two are not independent: there is no measurement to
// adjust while the spacing is off, and a setting that silently does nothing is
// a setting that reads as broken.
type SequenceSpacing int

const (
	// SpaceSeqNone leaves the items of a sequence packed together.
	SpaceSeqNone SequenceSpacing = iota

	// SpaceSeqMultiline sets off an item that spans more than one line, which
	// is a nested mapping or sequence, a block string, or a mapping with more
	// than one entry.
	SpaceSeqMultiline

	// SpaceSeqMappings also counts a mapping that fits on the dash's line, so
	// a list of mappings is spaced evenly however many entries each has:
	//
	//	- name: a
	//	  role: admin
	//
	//	- name: b
	//
	//	- name: c
	//
	// An empty mapping is "- {}" and never counts, and neither does a scalar.
	SpaceSeqMappings
)

// Style controls the shape of the output. Its zero value is the default: two
// spaces per level, literal blocks for multi-line strings, sequences indented
// under their key, and quotes chosen to escape least.
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

	// Quote selects the quotes put around a string that is neither plain nor a
	// block scalar. It is independent of Multiline: a single-quoted scalar
	// written across lines folds its breaks into spaces, so a string with a
	// newline in it is double-quoted even under Multiline == Quoted.
	Quote QuoteStyle

	// CompactSequence puts a sequence at the indentation of the mapping key
	// that introduces it, rather than one level deeper:
	//
	//	tags:          tags:
	//	  - a     vs   - a
	//
	// It applies only to a sequence that is a mapping value. A sequence
	// nested directly in another sequence always gets its own indented lines,
	// where the compact form would read as a sibling item.
	CompactSequence bool

	// SpaceMappings puts a blank line between two entries of a mapping when
	// the first spans more than one line, which sets off a nested mapping,
	// sequence or block string from the key that follows it:
	//
	//	list:
	//	  - a
	//	  - b
	//
	//	name: app
	//
	// An empty {} or [] fits on one line and does not count. No blank line
	// follows a string written with keep chomping ("|+"), where a reader would
	// take it as part of the string.
	SpaceMappings bool

	// SpaceSequences is SpaceMappings for the items of a sequence, and says
	// which items count as worth setting off.
	SpaceSequences SequenceSpacing

	// SpaceBefore also puts a blank line ahead of an entry that spans more
	// than one line, so it is set off from the entry before it as well as the
	// one after. It applies wherever SpaceMappings or SpaceSequences does.
	SpaceBefore bool

	// SpaceMaxLevel limits spacing to containers nested at most this deep,
	// counting the top-level container as 1. Zero means every level. It has no
	// effect unless SpaceMappings or SpaceSequences asks for a blank line.
	SpaceMaxLevel int
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
