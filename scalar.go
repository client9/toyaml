package toyaml

import (
	"unicode"
	"unicode/utf8"
)

// plainSafe reports whether s can be written as a YAML plain scalar and read
// back as the identical string.
func plainSafe(s []byte) bool {
	if len(s) == 0 {
		return false
	}
	if s[0] == ' ' || s[len(s)-1] == ' ' || s[len(s)-1] == ':' {
		return false
	}
	// document markers, which end or start a document when they stand alone
	if len(s) >= 3 && (s[0] == '-' || s[0] == '.') &&
		s[1] == s[0] && s[2] == s[0] && (len(s) == 3 || s[3] == ' ') {
		return false
	}
	switch s[0] {
	case '-', '?', ':':
		if len(s) == 1 || s[1] == ' ' {
			return false
		}
	case ',', '[', ']', '{', '}', '#', '&', '*', '!', '|', '>', '\'', '"', '%', '@', '`':
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf {
			// Non-ASCII whitespace is trimmed or treated as a line break by
			// one reader or another, which a plain scalar cannot survive, and
			// a character outside YAML's printable set has to be escaped.
			r, size := utf8.DecodeRune(s[i:])
			if unicode.IsSpace(r) || nonPrintable(r) {
				return false
			}
			i += size - 1
			continue
		}
		if c < 0x20 || c == 0x7f || c == '\\' || c == '"' {
			return false
		}
		if i+1 == len(s) {
			continue
		}
		if c == ':' && s[i+1] == ' ' {
			return false
		}
		if c == ' ' && s[i+1] == '#' {
			return false
		}
	}
	return !resolvesToNonString(s)
}

// keyword reports whether s, case-folded, is a plain scalar that YAML resolves
// to a bool, null, or a special float.
func keyword(s []byte) bool {
	if len(s) > 5 {
		return false
	}
	var buf [5]byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		buf[i] = c
	}
	switch string(buf[:len(s)]) {
	case "true", "false", "null", "~",
		"y", "n", "yes", "no", "on", "off",
		".inf", ".nan", "-.inf", "+.inf":
		return true
	}
	return false
}

// resolvesToNonString reports whether a YAML reader would turn the plain
// scalar s into something other than a string: a bool, null, a number, or a
// timestamp.
func resolvesToNonString(s []byte) bool {
	if keyword(s) {
		return true
	}
	switch c := s[0]; {
	case '0' <= c && c <= '9', c == '-', c == '+', c == '.':
	default:
		return false
	}
	if numeric(s) {
		return true
	}
	// sexagesimal times and timestamps: digits joined by the punctuation a
	// YAML reader accepts in a date
	digits := false
	for i := 0; i < len(s); i++ {
		switch {
		case '0' <= s[i] && s[i] <= '9':
			digits = true
		case s[i] == '_' || s[i] == ':' || s[i] == '-' || s[i] == '.' ||
			s[i] == '+' || s[i] == 'T' || s[i] == 'Z' || s[i] == ' ':
		default:
			return false
		}
	}
	return digits
}

// numeric reports whether s is a YAML number: a decimal integer or float, with
// optional underscore separators, or a based integer (0x, 0o, 0b). It is done
// by hand rather than through strconv, which allocates on failure.
func numeric(s []byte) bool {
	i := 0
	if s[i] == '+' || s[i] == '-' {
		i++
	}
	if i >= len(s) {
		return false
	}
	if s[i] == '0' && i+1 < len(s) {
		switch s[i+1] {
		case 'x', 'X':
			return baseDigits(s[i+2:], 16)
		case 'o', 'O':
			return baseDigits(s[i+2:], 8)
		case 'b', 'B':
			return baseDigits(s[i+2:], 2)
		}
	}

	digits := false
	for ; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			digits = true
			continue
		}
		if s[i] != '_' {
			break
		}
	}
	if i < len(s) && s[i] == '.' {
		for i++; i < len(s); i++ {
			if s[i] >= '0' && s[i] <= '9' {
				digits = true
				continue
			}
			if s[i] != '_' {
				break
			}
		}
	}
	if !digits {
		return false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		expDigits := false
		for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
			expDigits = true
		}
		if !expDigits {
			return false
		}
	}
	return i == len(s)
}

// baseDigits reports whether s is a non-empty run of digits valid in the given
// base, allowing underscore separators.
func baseDigits(s []byte, base int) bool {
	found := false
	for _, c := range s {
		if c == '_' {
			continue
		}
		v := digitVal(c)
		if v < 0 || v >= base {
			return false
		}
		found = true
	}
	return found
}

// digitVal returns the value of hex digit c, or -1 if c is not one.
func digitVal(c byte) int {
	switch {
	case '0' <= c && c <= '9':
		return int(c - '0')
	case 'a' <= c && c <= 'f':
		return int(c-'a') + 10
	case 'A' <= c && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// nonPrintable reports whether r is outside the set of characters YAML calls
// printable, and so has to be written as an escape rather than as itself.
// The ASCII controls are checked by their callers a byte at a time; this
// covers DEL and the C1 controls above it, which a reader is free to reject.
func nonPrintable(r rune) bool {
	return r >= 0x7f && r <= 0x9f
}
