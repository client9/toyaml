package toyaml

import (
	"bytes"
	"encoding/json/jsontext"
	"fmt"
	"io"
	"slices"
	"unicode/utf8"
)

// maxDepth bounds container nesting so that malformed or hostile input cannot
// drive the recursive emitter into a stack overflow.
const maxDepth = 200

// defaultIndent is the number of spaces per nesting level that Style's zero
// value asks for.
const defaultIndent = 2

// dashWidth is the width of the "- " that opens a sequence item, and so the
// column its value starts at. It is a property of YAML, not of the style.
const dashWidth = 2

// maxImplicitKey is how far a mapping key may reach before its ":", in Unicode
// characters, which YAML bounds so that a reader's lookahead is bounded too. A
// longer key is legal only in the explicit "? key" form, which most readers do
// not accept, so the encoder rejects it instead.
const maxImplicitKey = 1024

var spaces = []byte("                                ")

// encoder walks the JSON token stream and writes block-style YAML.
type encoder struct {
	style   Style
	dec     *jsontext.Decoder
	out     []byte
	str     []byte // decoded string value, reused across values
	scratch []byte // quoted scalar under construction, reused
	depth   int
}

func encode(src []byte, style Style) ([]byte, error) {
	if style.Indent < 0 {
		return nil, ErrNegativeIndent
	}
	if style.Indent == 0 {
		style.Indent = defaultIndent
	}
	switch style.Multiline {
	case BlockLiteral, Quoted:
	default:
		return nil, ErrUnknownMultiline
	}
	switch style.Quote {
	case QuoteAdaptive, QuoteDouble, QuoteSingle:
	default:
		return nil, ErrUnknownQuote
	}
	switch style.SpaceSequences {
	case SpaceSeqNone, SpaceSeqMultiline, SpaceSeqMappings:
	default:
		return nil, ErrUnknownSequenceSpacing
	}
	if style.SpaceMaxLevel < 0 {
		return nil, ErrNegativeSpaceLevel
	}

	e := &encoder{
		style: style,
		dec:   jsontext.NewDecoder(bytes.NewReader(src)),
		out:   make([]byte, 0, len(src)+len(src)/4),
	}

	if e.dec.PeekKind() == 0 {
		// Either the input is empty, which converts to empty output, or it
		// does not begin a value at all. Reading surfaces which.
		_, err := e.dec.ReadToken()
		if err == io.EOF {
			return e.out, nil
		}
		return nil, err
	}
	if err := e.emitValue(0, 0); err != nil {
		return nil, err
	}
	off := e.dec.InputOffset()
	if _, err := e.dec.ReadToken(); err != io.EOF {
		if err != nil {
			return nil, err
		}
		return nil, &EncodeError{ByteOffset: off, Err: ErrTrailingValue}
	}
	e.out = append(e.out, '\n')
	return e.out, nil
}

func (e *encoder) writeIndent(n int) {
	for n > len(spaces) {
		e.out = append(e.out, spaces...)
		n -= len(spaces)
	}
	e.out = append(e.out, spaces[:n]...)
}

// emitValue reads the next value and writes it at the current cursor position.
// The caller has already written whatever prefix belongs on this line ("- ",
// "key: ", or nothing). indent is the column that continuation lines of this
// value start at, and parent the indentation of the node it hangs off.
func (e *encoder) emitValue(indent, parent int) error {
	switch k := e.dec.PeekKind(); k {
	case '{', '[':
		if _, err := e.dec.ReadToken(); err != nil {
			return err
		}
		return e.emitContainer(k, indent)
	}
	return e.emitScalar(indent, parent)
}

// emitContainer writes a container whose opening delimiter has already been
// read. It is the one place nesting is counted, so every path into a mapping
// or a sequence passes through it.
func (e *encoder) emitContainer(k jsontext.Kind, indent int) error {
	if e.depth++; e.depth > maxDepth {
		return &EncodeError{ByteOffset: e.dec.InputOffset(), Err: ErrMaxDepth}
	}
	defer func() { e.depth-- }()

	if k == '{' {
		return e.emitMapping(indent)
	}
	return e.emitSequence(indent)
}

// takeEmpty reports whether the container just opened closes immediately,
// consuming the closing delimiter when it does.
func (e *encoder) takeEmpty(k jsontext.Kind) (bool, error) {
	if e.dec.PeekKind() != closerFor(k) {
		return false, nil
	}
	if _, err := e.dec.ReadToken(); err != nil {
		return false, err
	}
	return true, nil
}

func closerFor(k jsontext.Kind) jsontext.Kind {
	if k == '{' {
		return '}'
	}
	return ']'
}

// writeFlowEmpty writes the flow form of an empty container, which is the one
// place this encoder does not use block style: block form has no way to spell
// an empty collection.
func (e *encoder) writeFlowEmpty(k jsontext.Kind) {
	if k == '{' {
		e.out = append(e.out, '{', '}')
		return
	}
	e.out = append(e.out, '[', ']')
}

func (e *encoder) emitMapping(indent int) error {
	sp := e.newSpacer(e.style.SpaceMappings)
	for first := true; ; first = false {
		if e.dec.PeekKind() == '}' {
			if _, err := e.dec.ReadToken(); err != nil {
				return err
			}
			if first {
				e.writeFlowEmpty('{')
			}
			return nil
		}
		if !first {
			sp.separate(e, indent)
		}

		start := len(e.out)
		if err := e.emitKey(); err != nil {
			return err
		}
		e.out = append(e.out, ':')
		if err := e.emitNested(indent); err != nil {
			return err
		}
		sp.written(e, start, false)
	}
}

func (e *encoder) emitSequence(indent int) error {
	sp := e.newSpacer(e.style.SpaceSequences != SpaceSeqNone)
	for first := true; ; first = false {
		if e.dec.PeekKind() == ']' {
			if _, err := e.dec.ReadToken(); err != nil {
				return err
			}
			if first {
				e.writeFlowEmpty('[')
			}
			return nil
		}
		if !first {
			sp.separate(e, indent)
		}

		start := len(e.out)
		mapItem := e.style.SpaceSequences == SpaceSeqMappings && e.dec.PeekKind() == '{'
		if err := e.emitSeqItem(indent); err != nil {
			return err
		}
		// A mapping with one scalar entry fits on the dash's line, and counts
		// as multi-line only when asked, so that a list of mappings is spaced
		// evenly. An empty one is "- {}" and never counts.
		sp.written(e, start, mapItem && string(e.out[start:]) != "- {}")
	}
}

// spacer places the blank lines between the entries of one container.
//
// Whether an entry spans more than one line is known only once it has been
// written, since telling a block string from a quoted one takes decoding it.
// So a blank line owed after an entry is written with the next separator, and
// one owed before an entry is inserted behind it at the separator's offset.
// That insertion copies only the entry just written.
type spacer struct {
	on    bool // this container's entries are spaced
	multi bool // the entry just written spans more than one line
	sep   int  // offset of the line break ahead of the current entry, -1 for the first
}

// withinSpaceLevel reports whether the container being written now is shallow
// enough for SpaceMaxLevel to allow a blank line inside it. The top-level
// container is level 1.
func (e *encoder) withinSpaceLevel() bool {
	return e.style.SpaceMaxLevel == 0 || e.depth <= e.style.SpaceMaxLevel
}

func (e *encoder) newSpacer(kind bool) spacer {
	on := kind && e.withinSpaceLevel()
	return spacer{on: on, sep: -1}
}

// separate writes the line break ahead of an entry other than the first.
func (s *spacer) separate(e *encoder, indent int) {
	s.sep = len(e.out)
	if s.multi && e.blankable(s.sep) {
		e.out = append(e.out, '\n')
	}
	e.out = append(e.out, '\n')
	e.writeIndent(indent)
}

// written records the entry that begins at offset start and has just been
// written in full. force counts it as multi-line whatever its length.
func (s *spacer) written(e *encoder, start int, force bool) {
	if !s.on {
		return
	}
	multi := force || bytes.IndexByte(e.out[start:], '\n') >= 0
	if multi && e.style.SpaceBefore && s.sep >= 0 && !s.multi && e.blankable(s.sep) {
		e.out = slices.Insert(e.out, s.sep, '\n')
	}
	s.multi = multi
}

// blankable reports whether a blank line may go at offset i, which a line
// break follows. Output only ends a line there after a string with keep
// chomping ("|+"), whose trailing blank lines are content, and a reader would
// count one more.
func (e *encoder) blankable(i int) bool {
	return e.out[i-1] != '\n'
}

// emitSeqItem writes one item of a sequence indented at indent, dash included.
func (e *encoder) emitSeqItem(indent int) error {
	k := e.dec.PeekKind()
	if k != '{' && k != '[' {
		// An item written after "- " begins two columns in, whatever the
		// configured indent, because that is the width of the marker. Its
		// continuation lines have to line up with it.
		e.out = append(e.out, '-', ' ')
		return e.emitScalar(indent+dashWidth, indent)
	}
	if _, err := e.dec.ReadToken(); err != nil {
		return err
	}
	empty, err := e.takeEmpty(k)
	if err != nil {
		return err
	}
	if empty {
		e.out = append(e.out, '-', ' ')
		e.writeFlowEmpty(k)
		return nil
	}

	child := indent + dashWidth
	if k == '[' {
		// A nested sequence goes on its own indented lines, so it takes a
		// real indentation level. The compact "- - 1" form is equivalent, but
		// this one states the nested sequence's indentation outright instead
		// of implying it.
		child = indent + e.style.Indent
		e.out = append(e.out, '-', '\n')
		e.writeIndent(child)
	} else {
		e.out = append(e.out, '-', ' ')
	}
	return e.emitContainer(k, child)
}

// emitNested writes a mapping value after the ":" has been written. parent is
// the indentation of the key. Containers move to their own lines; scalars stay
// on the key's line.
func (e *encoder) emitNested(parent int) error {
	k := e.dec.PeekKind()
	if k != '{' && k != '[' {
		e.out = append(e.out, ' ')
		return e.emitScalar(parent+e.style.Indent, parent)
	}
	if _, err := e.dec.ReadToken(); err != nil {
		return err
	}
	empty, err := e.takeEmpty(k)
	if err != nil {
		return err
	}
	if empty {
		e.out = append(e.out, ' ')
		e.writeFlowEmpty(k)
		return nil
	}

	child := parent + e.style.Indent
	if k == '[' && e.style.CompactSequence {
		// a sequence may sit at the indentation of its own key
		child = parent
	}
	e.out = append(e.out, '\n')
	// Only a nested container reaches here: a scalar, a block scalar among
	// them, went out on the key's own line above, which is what keeps a blank
	// line from being read as a block string's first line.
	if e.style.SpaceAfterKey && e.withinSpaceLevel() {
		e.out = append(e.out, '\n')
	}
	e.writeIndent(child)
	return e.emitContainer(k, child)
}

func (e *encoder) emitKey() error {
	off := e.dec.InputOffset()
	v, err := e.dec.ReadValue()
	if err != nil {
		return err
	}
	// jsontext guarantees an object name is a string.
	if err := e.decodeString(v); err != nil {
		return err
	}
	start := len(e.out)
	if err := e.writeMaybePlain(e.str); err != nil {
		return err
	}
	// The limit is on the key as written, quotes and escapes included, since
	// that is what a reader counts.
	if n := utf8.RuneCount(e.out[start:]); n > maxImplicitKey {
		return &EncodeError{
			ByteOffset: off,
			Err:        fmt.Errorf("%w: %d characters exceeds the %d a YAML mapping key may span", ErrKeyTooLong, n, maxImplicitKey),
		}
	}
	return nil
}

func (e *encoder) emitScalar(indent, parent int) error {
	v, err := e.dec.ReadValue()
	if err != nil {
		return err
	}
	if v.Kind() != '"' {
		// null, true, false and numbers are spelled the same in both formats,
		// and a number is written through without evaluation so that one too
		// large for float64 keeps its digits.
		e.out = append(e.out, v...)
		return nil
	}
	if err := e.decodeString(v); err != nil {
		return err
	}
	if e.style.Multiline == BlockLiteral && e.emitBlockScalar(e.str, indent, parent) {
		return nil
	}
	return e.writeMaybePlain(e.str)
}

// decodeString unquotes the JSON string v into e.str, which the caller must
// use before the next read from the decoder.
func (e *encoder) decodeString(v jsontext.Value) error {
	s, err := jsontext.AppendUnquote(e.str[:0], v)
	if err != nil {
		return err
	}
	e.str = s
	return nil
}

// writeMaybePlain writes s as a YAML plain scalar when that round-trips, and
// as a double-quoted scalar otherwise.
func (e *encoder) writeMaybePlain(s []byte) error {
	if plainSafe(s) {
		e.out = append(e.out, s...)
		return nil
	}
	return e.writeQuoted(s)
}

// writeQuoted writes s as a quoted scalar in the configured style. Single
// quotes are used only where they can hold s as itself; the double-quoted form,
// which can escape anything, is the fallback and what every other setting lands
// on. Under QuoteAdaptive the gain is checked first, since it is the cheaper
// pass and a string with nothing to gain is the common case.
func (e *encoder) writeQuoted(s []byte) error {
	switch e.style.Quote {
	case QuoteAdaptive:
		if singleGain(s) >= 0 && singleSafe(s) {
			e.writeSingleQuoted(s)
			return nil
		}
	case QuoteSingle:
		if singleSafe(s) {
			e.writeSingleQuoted(s)
			return nil
		}
	}
	return e.writeDoubleQuoted(s)
}

// writeSingleQuoted writes s as a single-quoted scalar. That form has one
// escape, a doubled quote, and carries everything else as itself, so the body is
// copied in runs straight into e.out rather than built somewhere first. Copying
// verbatim is safe because decodeString has already rejected the invalid UTF-8
// that writeDoubleQuoted is spared by AppendQuote.
func (e *encoder) writeSingleQuoted(s []byte) {
	e.out = append(e.out, '\'')
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '\'' {
			continue
		}
		e.out = append(e.out, s[start:i+1]...) // the quote itself
		e.out = append(e.out, '\'')            // and again, which is the escape
		start = i + 1
	}
	e.out = append(e.out, s[start:]...)
	e.out = append(e.out, '\'')
}

// writeDoubleQuoted writes s as a double-quoted scalar. JSON's escaping of a string
// is valid inside a YAML double-quoted scalar, so the body comes straight from
// jsontext, with one addition. JSON escapes the C0 controls and stops there,
// while YAML also has no literal spelling for DEL and the C1 controls, and
// treats three characters above ASCII as line breaks. Left raw, a break ends
// the line even inside quotes and comes back folded into a space.
func (e *encoder) writeDoubleQuoted(s []byte) error {
	q, err := jsontext.AppendQuote(e.scratch[:0], s)
	if err != nil {
		return err
	}
	e.scratch = q

	start := 0
	for i := 0; i < len(q); {
		esc, size := yamlEscape(q[i:])
		if size == 0 {
			i++
			continue
		}
		e.out = append(e.out, q[start:i]...)
		e.out = append(e.out, esc...)
		i += size
		start = i
	}
	e.out = append(e.out, q[start:]...)
	return nil
}

// c1Escape holds the escape for each of U+0080 through U+009F, NEL (U+0085)
// among them, so that writing one costs no allocation.
var c1Escape [0x20]string

func init() {
	const hex = "0123456789abcdef"
	for i := range c1Escape {
		r := 0x80 + i
		c1Escape[i] = string([]byte{'\\', 'u', '0', '0', hex[r>>4], hex[r&0xf]})
	}
}

// yamlEscape returns the escape for the character at the start of b, and how
// many bytes that character occupies, or a zero size if b starts with one that
// needs no escape beyond what JSON already gives it. It covers DEL, the C1
// controls, and the line separators LS (U+2028) and PS (U+2029); the C0
// controls, \n and \r among them, reach here already escaped by jsontext.
func yamlEscape(b []byte) (string, int) {
	if b[0] == 0x7f {
		return "\\u007f", 1
	}
	if b[0] == 0xc2 && len(b) >= 2 && b[1] >= 0x80 && b[1] <= 0x9f {
		return c1Escape[b[1]-0x80], 2
	}
	if len(b) >= 3 && b[0] == 0xe2 && b[1] == 0x80 {
		switch b[2] {
		case 0xa8:
			return "\\u2028", 3
		case 0xa9:
			return "\\u2029", 3
		}
	}
	return "", 0
}
