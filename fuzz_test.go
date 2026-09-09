package toyaml

import (
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"strings"
	"testing"

	"github.com/client9/tojson"
)

// FuzzRoundTrip requires that a document converted to YAML reads back as the
// same document. tojson.FromYAML is the reader, so a failure means the two
// disagree about what was written.
func FuzzRoundTrip(f *testing.F) {
	for _, seed := range []string{
		`{"a":1,"b":[1,2,{"c":"d"}]}`,
		`[[1,2],[],{},null,true]`,
		`{"text":"line one\nline two\n","k":"x: y"}`,
		`"top level string"`,
		`{"nested":{"deep":{"deeper":[{"a":[]}]}}}`,
		`{"n":1e309,"big":123456789012345678901234567890}`,
		`{"pad":"  x\ny\n","tab":"a\tb"}`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, src string) {
		doc := []byte(src)
		if !json.Valid(doc) {
			return
		}
		// A carriage return inside a string does not survive the trip:
		// tojson normalizes CRLF to LF while re-encoding, so the value comes
		// back a byte shorter. That is longstanding behaviour there.
		if v, err := decodeJSON(doc); err != nil || valueHasCR(v) {
			return
		}
		// tojson.FromYAML decodes double-quoted strings with Go string
		// literal rules, which reject surrogate escapes.
		if hasSurrogateEscape(doc) {
			return
		}
		// rotate through the output shapes; the choice follows from the
		// input, so a failure reproduces from the corpus entry alone
		style := styleMatrix[len(doc)%len(styleMatrix)]
		y, err := FromJSONStyle(doc, style)
		if err != nil {
			// Input the decoder rejects, such as a duplicate object name that
			// encoding/json tolerates, and the two documented refusals, are
			// not round-trip failures.
			var se *jsontext.SyntacticError
			if errors.As(err, &se) || errors.Is(err, ErrKeyTooLong) || errors.Is(err, ErrMaxDepth) {
				return
			}
			t.Fatalf("FromJSONStyle(%s, %+v) error: %v", doc, style, err)
		}
		back, err := tojson.FromYAML(y)
		if err != nil {
			t.Fatalf("FromYAML error: %v\nyaml:\n%s", err, y)
		}
		if !sameJSON(t, doc, back) {
			t.Errorf("style %+v changed the document\n want: %s\n got:  %s\nyaml:\n%s",
				style, doc, back, y)
		}
	})
}

// FuzzFromJSON requires that arbitrary input never panics, and that whatever
// comes out of a success reads back as YAML.
func FuzzFromJSON(f *testing.F) {
	for _, seed := range []string{
		``, `{`, `[1,2]`, `"x"`, `{"a":{"b":[null]}}`, "\x00", `{"":""}`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, src string) {
		out, err := FromJSON([]byte(src))
		if err != nil {
			return
		}
		if len(out) == 0 {
			return
		}
		if _, err := tojson.FromYAML(out); err != nil {
			t.Fatalf("FromJSON(%q) produced YAML that does not read back: %v\n%s", src, err, out)
		}
	})
}

// valueHasCR reports whether any string in a decoded JSON document holds a
// carriage return.
func valueHasCR(v any) bool {
	switch t := v.(type) {
	case string:
		return strings.ContainsRune(t, '\r')
	case []any:
		for _, x := range t {
			if valueHasCR(x) {
				return true
			}
		}
	case map[string]any:
		for k, x := range t {
			if strings.ContainsRune(k, '\r') || valueHasCR(x) {
				return true
			}
		}
	}
	return false
}

// hasSurrogateEscape reports whether b holds a \uD800-\uDFFF escape.
func hasSurrogateEscape(b []byte) bool {
	for i := 0; i+3 < len(b); i++ {
		if b[i] != '\\' || b[i+1] != 'u' {
			continue
		}
		if b[i+2] != 'd' && b[i+2] != 'D' {
			continue
		}
		if v := digitVal(b[i+3]); v >= 8 {
			return true
		}
	}
	return false
}
