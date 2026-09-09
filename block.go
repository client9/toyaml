package toyaml

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

// blockContent reports whether a literal block scalar can hold s as itself.
// Block content is literal text, so it carries a quote, a backslash or a tab
// without escaping. What it cannot represent is a character YAML excludes from
// its printable set, which has no literal spelling, or whitespace outside
// ASCII, which a reader trims or treats as a line break. A tab is content, not
// indentation, so it qualifies; a line that opens or closes with one is turned
// away later.
func blockContent(s []byte) bool {
	newline := false
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '\n':
				newline = true
			case c == '\t':
			case c < 0x20 || c == 0x7f:
				return false
			}
			i++
			continue
		}
		r, size := utf8.DecodeRune(s[i:])
		if unicode.IsSpace(r) || nonPrintable(r) {
			return false
		}
		i += size
	}
	return newline
}

// emitBlockScalar writes s as a literal block scalar ("|") when that is both
// possible and an improvement, reporting whether it did so. parent is the
// indentation of the node this scalar hangs off, which is what an explicit
// indentation indicator counts from; indent is the column the content is
// written at.
func (e *encoder) emitBlockScalar(s []byte, indent, parent int) bool {
	if !blockContent(s) {
		return false
	}
	// Block content must be indented deeper than its parent node. A caller
	// that has already moved in — a mapping value, or a sequence item, whose
	// content starts at the dash width — has satisfied that. Only a value
	// written at its parent's own column, which is the top level, has not.
	if indent <= parent {
		indent = parent + e.style.Indent
	}

	// Chomping. One trailing newline is the default, "clip" (|). None is
	// "strip" (|-). More than one is "keep" (|+), which also holds on to the
	// blank lines at the end.
	body := s
	trailing := 0
	for len(body) > 0 && body[len(body)-1] == '\n' {
		body = body[:len(body)-1]
		trailing++
	}
	if len(body) == 0 {
		return false // nothing but line breaks
	}
	chomp := ""
	switch trailing {
	case 0:
		chomp = "-"
	case 1:
	default:
		chomp = "+"
	}

	// A reader with no indentation indicator to go on takes the block's
	// indentation from its first non-empty line, so leading whitespace there
	// would be read as indentation and lost. Only that line calls for an
	// indicator: once it has fixed the indentation, whitespace opening a line
	// below it is content, since every line is written at the block
	// indentation plus whatever it carries of its own. Trailing whitespace has
	// no such remedy: it does not survive a round trip.
	indicator := ""
	firstNonEmpty := true
	for rest := body; ; {
		ln, more := nextLine(&rest)
		if len(ln) > 0 {
			if firstNonEmpty && (ln[0] == ' ' || ln[0] == '\t') {
				// The indicator counts from the parent's indentation and is a
				// single digit, so a deeply nested block gives up here.
				rel := indent - parent
				if rel < 1 || rel > 9 {
					return false
				}
				indicator = string([]byte{'0' + byte(rel)})
			}
			firstNonEmpty = false
			if c := ln[len(ln)-1]; c == ' ' || c == '\t' {
				return false
			}
		}
		if !more {
			break
		}
	}

	e.out = append(e.out, '|')
	e.out = append(e.out, indicator...)
	e.out = append(e.out, chomp...)
	for rest := body; ; {
		ln, more := nextLine(&rest)
		e.out = append(e.out, '\n')
		if len(ln) > 0 {
			e.writeIndent(indent)
			e.out = append(e.out, ln...)
		}
		if !more {
			break
		}
	}
	// Under keep, the last content line accounts for one trailing newline and
	// each blank line after it for one more.
	for i := 1; i < trailing; i++ {
		e.out = append(e.out, '\n')
	}
	return true
}

// nextLine splits the leading newline-terminated line off of *rest, reporting
// whether any further lines remain.
func nextLine(rest *[]byte) (line []byte, more bool) {
	if i := bytes.IndexByte(*rest, '\n'); i >= 0 {
		line, *rest = (*rest)[:i], (*rest)[i+1:]
		return line, true
	}
	line, *rest = *rest, nil
	return line, false
}
