package toyaml

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/client9/tojson"
)

func TestFromJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"scalar", `42`, "42\n"},
		{"emptyObject", `{}`, "{}\n"},
		{"emptyArray", `[]`, "[]\n"},
		{"flatMap", `{"a":1,"b":"two","c":true,"d":null}`,
			"a: 1\nb: two\nc: true\nd: null\n"},
		{"nestedMap", `{"a":{"b":{"c":1}}}`,
			"a:\n  b:\n    c: 1\n"},
		{"seqOfScalars", `[1,"two",false]`,
			"- 1\n- two\n- false\n"},
		{"seqUnderKey", `{"list":[1,2]}`,
			"list:\n  - 1\n  - 2\n"},
		{"seqOfMaps", `[{"a":1,"b":2},{"a":3}]`,
			"- a: 1\n  b: 2\n- a: 3\n"},
		{"seqOfSeqs", `[[1,2],[3]]`,
			"-\n  - 1\n  - 2\n-\n  - 3\n"},
		{"emptyNested", `{"a":{},"b":[]}`,
			"a: {}\nb: []\n"},
		{"emptyInSeq", `[{},[],1]`,
			"- {}\n- []\n- 1\n"},
		{"quotedWhenAmbiguous", `{"a":"true","b":"123","c":"","d":"yes"}`,
			"a: 'true'\nb: '123'\nc: ''\nd: 'yes'\n"},
		{"quotedIndicators", `{"a":"- x","b":"x: y","c":"#c","d":" pad "}`,
			"a: '- x'\nb: 'x: y'\nc: '#c'\nd: ' pad '\n"},
		{"numericKey", `{"1":"a","true":"b"}`,
			"'1': a\n'true': b\n"},
		{"blockScalar", `{"text":"line one\nline two\n"}`,
			"text: |\n  line one\n  line two\n"},
		{"blockScalarStrip", `{"text":"line one\nline two"}`,
			"text: |-\n  line one\n  line two\n"},
		{"blockScalarBlankLine", `{"text":"a\n\nb\n"}`,
			"text: |\n  a\n\n  b\n"},
		{"blockScalarInSeq", `["a\nb"]`,
			"- |-\n  a\n  b\n"},
		{"blockLaterLineIndent", `{"t":"a\n  b\n"}`,
			"t: |\n  a\n    b\n"},
		{"blockIndicatedFirstLine", `{"t":"  a\nb\n"}`,
			"t: |2\n    a\n  b\n"},
		{"blockIndicatedStrip", `{"t":"  a\nb"}`,
			"t: |2-\n    a\n  b\n"},
		{"blockIndicatedTab", "{\"t\":\"\\ta\\nb\\n\"}",
			"t: |2\n  \ta\n  b\n"},
		{"blockRejectedTrailingSpace", `{"t":"a \nb\n"}`,
			"t: \"a \\nb\\n\"\n"},
		{"blockRejectedCarriageReturn", `{"t":"a\r\nb\n"}`,
			"t: \"a\\r\\nb\\n\"\n"},
		{"blockOnlyNewlines", `{"t":"\n\n"}`,
			"t: \"\\n\\n\"\n"},
		{"blockKeepTrailingNewlines", `{"t":"a\n\n"}`,
			"t: |+\n  a\n\n"},
		{"blockKeepManyTrailingNewlines", `{"t":"a\nb\n\n\n"}`,
			"t: |+\n  a\n  b\n\n\n"},
		{"unicode", `{"k":"héllo","emoji":"🎉"}`,
			"k: héllo\nemoji: 🎉\n"},
		{"escapes", `{"k":"tab\there"}`,
			"k: \"tab\\there\"\n"},
		{"bignum", `{"num":1e309,"i":123456789012345678901234567890}`,
			"num: 1e309\ni: 123456789012345678901234567890\n"},
		{"yaml11Keywords", `{"n":1,"y":2,"off":3}`,
			"'n': 1\n'y': 2\n'off': 3\n"},
		{"deepMix", `{"a":[{"b":[1,{"c":"d"}]}]}`,
			"a:\n  - b:\n      - 1\n      - c: d\n"},
		{"empty", ``, ""},
		{"whitespaceOnly", "  \n\t", ""},
		{"dateLike", `{"d":"2020-01-02","t":"12:30:00"}`,
			"d: '2020-01-02'\nt: '12:30:00'\n"},
		{"dashWord", `{"a":"-x","b":"a-b"}`,
			"a: -x\nb: a-b\n"},
		// An unnecessary escape in the input does not force a quoted scalar:
		// the string is compared after decoding, not as it was written.
		{"redundantEscape", `{"k":"\u0041\/b"}`, "k: A/b\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FromJSON([]byte(tc.in))
			if err != nil {
				t.Fatalf("FromJSON(%q) error = %v", tc.in, err)
			}
			if string(got) != tc.want {
				t.Errorf("FromJSON(%q)\n got: %q\nwant: %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestRoundTrip converts JSON to YAML, reads it back with tojson.FromYAML, and
// requires the result to be the same document.
func TestRoundTrip(t *testing.T) {
	inputs := []string{
		`{"a":1,"b":[1,2,3],"c":{"d":"e"}}`,
		`[{"name":"one","tags":["x","y"],"on":true},{"name":"two","tags":[]}]`,
		`{"text":"line one\nline two\n","note":"x: y","empty":"","n":null}`,
		`{"deep":{"a":{"b":{"c":[{"d":[[1,2],[3]]}]}}}}`,
		`{"nums":[0,-1,1.5,1e10,-2.5e-3],"strs":["true","0","null","-","#"]}`,
		`{"unicode":"héllo 🎉","quote":"say \"hi\"","tab":"a\tb"}`,
		`[]`,
		`{}`,
		`{"a":{},"b":[],"c":[[]],"d":[{}]}`,
	}
	for _, in := range inputs {
		y, err := FromJSON([]byte(in))
		if err != nil {
			t.Fatalf("FromJSON(%s) error = %v", in, err)
		}
		back, err := tojson.FromYAML(y)
		if err != nil {
			t.Fatalf("FromYAML of\n%s\nerror = %v", y, err)
		}
		if !sameJSON(t, []byte(in), back) {
			t.Errorf("round trip mismatch\n  in:   %s\n  yaml: %s\n  back: %s", in, y, back)
		}
	}
}

func sameJSON(t *testing.T, a, b []byte) bool {
	t.Helper()
	av, err := decodeJSON(a)
	if err != nil {
		t.Fatalf("decoding %s: %v", a, err)
	}
	bv, err := decodeJSON(b)
	if err != nil {
		t.Fatalf("decoding %s: %v", b, err)
	}
	ab, _ := json.Marshal(av)
	bb, _ := json.Marshal(bv)
	return bytes.Equal(ab, bb)
}

// decodeJSON decodes with UseNumber so that numbers this package passes
// through untouched, such as 1e309, survive the comparison instead of
// overflowing float64.
func decodeJSON(b []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// Input that is not JSON is rejected. Most of these are the decoder's
// business now, so the test is that they reach the caller rather than being
// written out as something.
func TestFromJSONErrors(t *testing.T) {
	cases := []string{
		`{"a":`,
		`{"a":0x10}`,
		`{"a":"unterminated}`,
		`[`,
		`{`,
		`{"a"`,
		`{"a" 1}`,
		`[1,2`,
		`{"a":1} extra`,
		`{[1]:2}`,
		`{a:1}`,
		`{"a":1,}`,
		`[1,,2]`,
		`[,1]`,
		`{"a":'b'}`,
		"{\"a\":1} // comment",
		`[1 2]`,
		`{"a":1,"a":2}`, // duplicate names are rejected by the decoder
	}
	for _, in := range cases {
		if out, err := FromJSON([]byte(in)); err == nil {
			t.Errorf("FromJSON(%q) = %q, want error", in, out)
		}
	}
}

// A second top-level value has no YAML rendering here, and is reported as an
// EncodeError rather than as bad JSON, since each value on its own is fine.
func TestFromJSONTrailingValue(t *testing.T) {
	_, err := FromJSON([]byte(`{"a":1} {"b":2}`))
	if !errors.Is(err, ErrTrailingValue) {
		t.Fatalf("FromJSON() error = %v, want %v", err, ErrTrailingValue)
	}
	var ee *EncodeError
	if !errors.As(err, &ee) {
		t.Fatalf("FromJSON() error = %v (%T), want *EncodeError", err, err)
	}
}

// YAML bounds a mapping key written without the "? " indicator at 1024
// Unicode characters, so that a reader's lookahead for the ":" is bounded.
// The count is of the key as written, so quotes and escapes are part of it,
// and it is characters rather than bytes.
func TestLongKey(t *testing.T) {
	key := func(n int, r rune) string {
		b, err := json.Marshal(map[string]any{strings.Repeat(string(r), n): 1})
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		return string(b)
	}
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"plain at the limit", key(maxImplicitKey, 'k'), false},
		{"plain one over", key(maxImplicitKey+1, 'k'), true},
		// three bytes per character, so the byte count is well over the limit
		// while the character count is not
		{"multibyte at the limit", key(maxImplicitKey, 'あ'), false},
		{"multibyte one over", key(maxImplicitKey+1, 'あ'), true},
		// "#" forces quoting, and the two quotes count toward the limit
		{"quoted at the limit", key(maxImplicitKey-2, '#'), false},
		{"quoted one over", key(maxImplicitKey-1, '#'), true},
		// the limit is on keys alone; a value of any length is fine
		{"long value", `{"k":"` + strings.Repeat("v", 4096) + `"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromJSON([]byte(tt.in))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("FromJSON() = %q, want error", got)
				}
				if !errors.Is(err, ErrKeyTooLong) {
					t.Fatalf("FromJSON() error = %v, want %v", err, ErrKeyTooLong)
				}
				var ee *EncodeError
				if !errors.As(err, &ee) {
					t.Fatalf("FromJSON() error = %v (%T), want *EncodeError", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("FromJSON() error = %v", err)
			}
			back, err := tojson.FromYAML(got)
			if err != nil {
				t.Fatalf("FromYAML() error = %v", err)
			}
			if !sameJSON(t, []byte(tt.in), back) {
				t.Errorf("round trip mismatch")
			}
		})
	}
}

// A string that YAML would resolve to a based integer has to be quoted, or it
// reads back as a number. This is the arm of numeric that reaches baseDigits.
func TestPlainSafeBasedInteger(t *testing.T) {
	tests := []struct {
		s     string
		plain bool // may be written as a plain scalar
	}{
		{"0x10", false},
		{"0X1F", false},
		{"0o17", false},
		{"0O17", false},
		{"0b101", false},
		{"0B1_01", false},
		{"-0x10", false},
		{"+0b1", false},
		{"0x_1", false},
		// not based integers: no digits, or a digit the base does not have
		{"0x", true},
		{"0o", true},
		{"0b", true},
		{"0x_", true},
		{"0b102", true},
		{"0o18", true},
		{"0xg", true},
		{"0y10", true},
		// a bare "0" has nothing after it to switch on, so it stays decimal
		{"0", false},
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			if got := plainSafe([]byte(tt.s)); got != tt.plain {
				t.Errorf("plainSafe(%q) = %v, want %v", tt.s, got, tt.plain)
			}
			// whatever the decision, the string must survive the round trip
			in, err := json.Marshal(map[string]string{"k": tt.s})
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			out, err := FromJSON(in)
			if err != nil {
				t.Fatalf("FromJSON() error = %v", err)
			}
			back, err := tojson.FromYAML(out)
			if err != nil {
				t.Fatalf("FromYAML() error = %v\n%s", err, out)
			}
			var got map[string]any
			if err := json.Unmarshal(back, &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if got["k"] != tt.s {
				t.Errorf("round trip = %#v, want %q\nYAML: %s", got["k"], tt.s, out)
			}
		})
	}
}

func TestWideIndent(t *testing.T) {
	// nest deep enough to exercise the writeIndent loop past one span of spaces
	const levels = 24
	in := ""
	for i := 0; i < levels; i++ {
		in += `{"k":`
	}
	in += `"v"`
	for i := 0; i < levels; i++ {
		in += "}"
	}
	got, err := FromJSON([]byte(in))
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}
	want := ""
	for i := 0; i < levels; i++ {
		want += strings.Repeat(" ", i*2) + "k:"
		if i == levels-1 {
			want += " v\n"
		} else {
			want += "\n"
		}
	}
	if string(got) != want {
		t.Errorf("FromJSON() =\n%s\nwant\n%s", got, want)
	}
}

func TestBlockScalarDeepIndent(t *testing.T) {
	in := `{"aaaaaaaaaa":{"bbbbbbbbbb":{"cccccccccc":{"dddddddddd":{"e":"one\ntwo\n"}}}}}`
	got, err := FromJSON([]byte(in))
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}
	back, err := tojson.FromYAML(got)
	if err != nil {
		t.Fatalf("FromYAML() error = %v\n%s", err, got)
	}
	if !sameJSON(t, []byte(in), back) {
		t.Errorf("round trip mismatch: %s", back)
	}
}

// A block scalar's content is indented from the node it hangs off. A sequence
// item's value starts at the dash width, so its block sits two columns in
// whatever the style indent; only a scalar written at its parent's own column,
// which is the top level, takes a full level of indent. The leading-space
// cases also pin the indentation indicator, which counts from the parent's
// column.
func TestBlockScalarIndentColumn(t *testing.T) {
	tests := []struct {
		name   string
		indent int
		in     string
		want   string
	}{
		{"seq item, indent 2", 2, `["one\ntwo\n"]`, "- |\n  one\n  two\n"},
		{"seq item, indent 4", 4, `["one\ntwo\n"]`, "- |\n  one\n  two\n"},
		{"seq item, indent 9", 9, `["one\ntwo\n"]`, "- |\n  one\n  two\n"},
		{"seq item leading space, indent 2", 2, `["  x\ny\n"]`, "- |2\n    x\n  y\n"},
		{"seq item leading space, indent 9", 9, `["  x\ny\n"]`, "- |2\n    x\n  y\n"},
		{"top level, indent 2", 2, `"one\ntwo\n"`, "|\n  one\n  two\n"},
		{"top level, indent 4", 4, `"one\ntwo\n"`, "|\n    one\n    two\n"},
		{"top level leading space, indent 2", 2, `"  x\ny\n"`, "|2\n    x\n  y\n"},
		{"top level leading space, indent 4", 4, `"  x\ny\n"`, "|4\n      x\n    y\n"},
		{"mapping value, indent 4", 4, `{"k":"one\ntwo\n"}`, "k: |\n    one\n    two\n"},
		{"seq under key, indent 4", 4, `{"k":["  x\ny\n"]}`, "k:\n    - |2\n        x\n      y\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromJSONStyle([]byte(tt.in), Style{Indent: tt.indent})
			if err != nil {
				t.Fatalf("FromJSONStyle() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("FromJSONStyle() =\n%q\nwant\n%q", got, tt.want)
			}
			back, err := tojson.FromYAML(got)
			if err != nil {
				t.Fatalf("FromYAML() error = %v\n%s", err, got)
			}
			if !sameJSON(t, []byte(tt.in), back) {
				t.Errorf("round trip mismatch: %s", back)
			}
		})
	}
}

func TestDeepNesting(t *testing.T) {
	src := bytes.Repeat([]byte("["), maxDepth+5)
	src = append(src, bytes.Repeat([]byte("]"), maxDepth+5)...)
	_, err := FromJSON(src)
	if !errors.Is(err, ErrMaxDepth) {
		t.Errorf("FromJSON(deeply nested) error = %v, want %v", err, ErrMaxDepth)
	}
}

// styleMatrix is the spread of output shapes the round-trip tests check.
// None of them may change the document.
var styleMatrix = []Style{
	{},
	{Indent: 1},
	{Indent: 3},
	{Indent: 8},
	{Multiline: Quoted},
	{CompactSequence: true},
	{Indent: 1, CompactSequence: true},
	{Indent: 4, Multiline: Quoted, CompactSequence: true},
	{SpaceMappings: true, SpaceSequences: true},
	{SpaceMappings: true, SpaceSequences: true, SpaceBefore: true},
	{Indent: 4, CompactSequence: true, SpaceMappings: true, SpaceBefore: true, SpaceMaxLevel: 2},
	{Multiline: Quoted, SpaceSequences: true, SpaceBefore: true, SpaceMappingItems: true},
	// Every entry above leaves Quote zero, so they cover the adaptive default;
	// these cover the two settings that name a form outright.
	{Quote: QuoteDouble},
	{Quote: QuoteSingle},
	{Multiline: Quoted, Quote: QuoteSingle},
	{Indent: 4, CompactSequence: true, SpaceMappings: true, Quote: QuoteDouble},
}

// TestCorpus converts every JSON document in testdata to YAML under each
// style, reads it back, and requires the document to survive.
func TestCorpus(t *testing.T) {
	var files []string
	err := filepath.WalkDir("testdata", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(p) == ".json" {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking testdata: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no corpus files found")
	}

	for _, f := range files {
		doc, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		for _, style := range styleMatrix {
			y, err := FromJSONStyle(doc, style)
			if err != nil {
				t.Errorf("%s: FromJSONStyle(%+v): %v", f, style, err)
				continue
			}
			back, err := tojson.FromYAML(y)
			if err != nil {
				t.Errorf("%s: FromYAML of %+v output: %v\n%s", f, style, err, y)
				continue
			}
			if !sameJSON(t, doc, back) {
				t.Errorf("%s: %+v changed the document\n want: %s\n got:  %s\nyaml:\n%s",
					f, style, doc, back, y)
			}
		}
	}
}

// Characters that a YAML reader trims or treats as a line break cannot survive
// a plain scalar, so a string holding one is quoted.
func TestUnicodeWhitespace(t *testing.T) {
	const (
		nel  = "\u0085" // next line
		nbsp = "\u00a0" // no-break space
		idsp = "\u3000" // ideographic space
	)
	cases := []struct{ in, want string }{
		{`"` + nel + `"`, `"\u0085"` + "\n"},
		{`"` + nbsp + `"`, "'" + nbsp + "'\n"},
		{`"a` + nel + `b"`, `"a\u0085b"` + "\n"},
		{`"` + idsp + `x"`, "'" + idsp + "x'\n"},
		{`{"k":"x` + nbsp + `"}`, "k: 'x" + nbsp + "'\n"},
		// DEL and the C1 controls are outside YAML's printable set, and LS is
		// a line break, so all three are escaped even though JSON leaves them
		// raw
		{"\"a\u007fb\"", `"a\u007fb"` + "\n"},
		{"\"a\u0081b\"", `"a\u0081b"` + "\n"},
		{"\"a\u2028b\"", `"a\u2028b"` + "\n"},
		// ordinary non-ASCII text stays plain
		{"\"h\u00e9llo\"", "h\u00e9llo\n"},
		{"\"\u65e5\u672c\u8a9e\"", "\u65e5\u672c\u8a9e\n"},
	}
	for _, tc := range cases {
		got, err := FromJSON([]byte(tc.in))
		if err != nil {
			t.Errorf("FromJSON(%s) error = %v", tc.in, err)
			continue
		}
		if string(got) != tc.want {
			t.Errorf("FromJSON(%s) = %q, want %q", tc.in, got, tc.want)
		}
	}

	// and the document survives the trip back
	for _, in := range []string{
		`"` + nel + `"`,
		`"` + nbsp + `"`,
		`{"k":"a` + nel + `b"}`,
		`["` + nbsp + `"]`,
		`"a\nb` + nel + `c\n"`,
	} {
		y, err := FromJSON([]byte(in))
		if err != nil {
			t.Fatalf("FromJSON(%s) error = %v", in, err)
		}
		back, err := tojson.FromYAML(y)
		if err != nil {
			t.Fatalf("FromYAML(%q) error = %v", y, err)
		}
		if !sameJSON(t, []byte(in), back) {
			t.Errorf("round trip mismatch for %s\n got: %s\nyaml: %q", in, back, y)
		}
	}
}

// A block scalar cannot carry whitespace a reader would trim or treat as a
// line break, so a string holding one is written double-quoted instead.
func TestBlockScalarExoticSpace(t *testing.T) {
	const emsp = "\u2004" // three-per-em space
	cases := []struct{ in, want string }{
		{`"a\nb\n"`, "|\n  a\n  b\n"},
		{`"\n` + emsp + `"`, `"\n` + emsp + `"` + "\n"},
		{`"a\n` + emsp + `b\n"`, `"a\n` + emsp + `b\n"` + "\n"},
	}
	for _, tc := range cases {
		got, err := FromJSON([]byte(tc.in))
		if err != nil {
			t.Errorf("FromJSON(%s) error = %v", tc.in, err)
			continue
		}
		if string(got) != tc.want {
			t.Errorf("FromJSON(%s) = %q, want %q", tc.in, got, tc.want)
		}
		back, err := tojson.FromYAML(got)
		if err != nil {
			t.Errorf("FromYAML(%q) error = %v", got, err)
			continue
		}
		if !sameJSON(t, []byte(tc.in), back) {
			t.Errorf("round trip mismatch for %s: %s", tc.in, back)
		}
	}
}

func TestStyleIndent(t *testing.T) {
	src := []byte(`{"a":{"b":1},"list":[1,{"c":2}],"text":"x\ny\n"}`)
	cases := []struct {
		indent int
		want   string
	}{
		{0, "a:\n  b: 1\nlist:\n  - 1\n  - c: 2\ntext: |\n  x\n  y\n"},
		{1, "a:\n b: 1\nlist:\n - 1\n - c: 2\ntext: |\n x\n y\n"},
		{4, "a:\n    b: 1\nlist:\n    - 1\n    - c: 2\ntext: |\n    x\n    y\n"},
	}
	for _, tc := range cases {
		got, err := FromJSONStyle(src, Style{Indent: tc.indent})
		if err != nil {
			t.Errorf("Indent %d: error = %v", tc.indent, err)
			continue
		}
		if string(got) != tc.want {
			t.Errorf("Indent %d =\n%s\nwant\n%s", tc.indent, got, tc.want)
		}
	}
}

func TestStyleMultiline(t *testing.T) {
	src := []byte(`{"text":"one\ntwo\n","plain":"short"}`)

	got, err := FromJSONStyle(src, Style{Multiline: BlockLiteral})
	if err != nil {
		t.Fatalf("BlockLiteral: %v", err)
	}
	if want := "text: |\n  one\n  two\nplain: short\n"; string(got) != want {
		t.Errorf("BlockLiteral =\n%s\nwant\n%s", got, want)
	}

	got, err = FromJSONStyle(src, Style{Multiline: Quoted})
	if err != nil {
		t.Fatalf("Quoted: %v", err)
	}
	if want := "text: \"one\\ntwo\\n\"\nplain: short\n"; string(got) != want {
		t.Errorf("Quoted =\n%s\nwant\n%s", got, want)
	}
}

func TestStyleCompactSequence(t *testing.T) {
	src := []byte(`{"tags":["a","b"],"nest":{"inner":[1]}}`)

	got, err := FromJSONStyle(src, Style{CompactSequence: true})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	want := "tags:\n- a\n- b\nnest:\n  inner:\n  - 1\n"
	if string(got) != want {
		t.Errorf("CompactSequence =\n%s\nwant\n%s", got, want)
	}

	// a sequence inside a sequence keeps its own indented lines, where the
	// compact form would read as a sibling item
	got, err = FromJSONStyle([]byte(`[[1,2],[3]]`), Style{CompactSequence: true})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if want := "-\n  - 1\n  - 2\n-\n  - 3\n"; string(got) != want {
		t.Errorf("nested sequence =\n%s\nwant\n%s", got, want)
	}
}

func TestStyleErrors(t *testing.T) {
	src := []byte(`{"a":1}`)
	if _, err := FromJSONStyle(src, Style{Indent: -1}); !errors.Is(err, ErrNegativeIndent) {
		t.Errorf("negative indent: error = %v, want %v", err, ErrNegativeIndent)
	}
	if _, err := FromJSONStyle(src, Style{Multiline: 99}); !errors.Is(err, ErrUnknownMultiline) {
		t.Errorf("unknown multiline: error = %v, want %v", err, ErrUnknownMultiline)
	}
	if _, err := FromJSONStyle(src, Style{Quote: 99}); !errors.Is(err, ErrUnknownQuote) {
		t.Errorf("unknown quote: error = %v, want %v", err, ErrUnknownQuote)
	}
	if _, err := FromJSONStyle(src, Style{SpaceMappings: true, SpaceMaxLevel: -1}); !errors.Is(err, ErrNegativeSpaceLevel) {
		t.Errorf("negative space level: error = %v, want %v", err, ErrNegativeSpaceLevel)
	}
}

func TestStyleSpacing(t *testing.T) {
	mapping := `{"a":1,"b":2,"list":["x","z"],"obj":{"k":{"m":1},"p":2},"c":3,"e":[],"f":4}`
	seq := `[{"a":1,"b":2},"s",[1,2],"t"]`
	people := `[{"name":"A","sex":"m"},{"key":"k1"},{"key":"k2"},{"name":"B","sex":"m"}]`
	cases := []struct {
		name  string
		in    string
		style Style
		want  string
	}{
		{"off", mapping, Style{},
			"a: 1\nb: 2\nlist:\n  - x\n  - z\nobj:\n  k:\n    m: 1\n  p: 2\nc: 3\ne: []\nf: 4\n"},
		{"mappings", mapping, Style{SpaceMappings: true},
			"a: 1\nb: 2\nlist:\n  - x\n  - z\n\nobj:\n  k:\n    m: 1\n\n  p: 2\n\nc: 3\ne: []\nf: 4\n"},
		{"mappings before", mapping, Style{SpaceMappings: true, SpaceBefore: true},
			"a: 1\nb: 2\n\nlist:\n  - x\n  - z\n\nobj:\n  k:\n    m: 1\n\n  p: 2\n\nc: 3\ne: []\nf: 4\n"},
		{"mappings top level", mapping, Style{SpaceMappings: true, SpaceMaxLevel: 1},
			"a: 1\nb: 2\nlist:\n  - x\n  - z\n\nobj:\n  k:\n    m: 1\n  p: 2\n\nc: 3\ne: []\nf: 4\n"},
		{"level alone", mapping, Style{SpaceMaxLevel: 1},
			"a: 1\nb: 2\nlist:\n  - x\n  - z\nobj:\n  k:\n    m: 1\n  p: 2\nc: 3\ne: []\nf: 4\n"},
		{"sequences leave mappings", mapping, Style{SpaceSequences: true},
			"a: 1\nb: 2\nlist:\n  - x\n  - z\nobj:\n  k:\n    m: 1\n  p: 2\nc: 3\ne: []\nf: 4\n"},
		{"sequences", seq, Style{SpaceSequences: true},
			"- a: 1\n  b: 2\n\n- s\n-\n  - 1\n  - 2\n\n- t\n"},
		{"sequences before", seq, Style{SpaceSequences: true, SpaceBefore: true},
			"- a: 1\n  b: 2\n\n- s\n\n-\n  - 1\n  - 2\n\n- t\n"},
		{"mappings leave sequences", seq, Style{SpaceMappings: true},
			"- a: 1\n  b: 2\n- s\n-\n  - 1\n  - 2\n- t\n"},
		{"one-line mapping items", people, Style{SpaceSequences: true},
			"- name: A\n  sex: m\n\n- key: k1\n- key: k2\n- name: B\n  sex: m\n"},
		{"one-line mapping items before", people, Style{SpaceSequences: true, SpaceBefore: true},
			"- name: A\n  sex: m\n\n- key: k1\n- key: k2\n\n- name: B\n  sex: m\n"},
		{"mapping items", people, Style{SpaceSequences: true, SpaceMappingItems: true},
			"- name: A\n  sex: m\n\n- key: k1\n\n- key: k2\n\n- name: B\n  sex: m\n"},
		{"mapping items alone", people, Style{SpaceMappingItems: true},
			"- name: A\n  sex: m\n- key: k1\n- key: k2\n- name: B\n  sex: m\n"},
		{"mapping items mixed", `["a",{"k":1},{},"b"]`, Style{SpaceSequences: true, SpaceMappingItems: true},
			"- a\n- k: 1\n\n- {}\n- b\n"},
		{"mapping items before", `["a",{"k":1},{},"b"]`, Style{SpaceSequences: true, SpaceMappingItems: true, SpaceBefore: true},
			"- a\n\n- k: 1\n\n- {}\n- b\n"},
		{"block string", `{"t":"a\nb\n","u":1}`, Style{SpaceMappings: true},
			"t: |\n  a\n  b\n\nu: 1\n"},
		{"quoted string", `{"t":"a\nb\n","u":1}`, Style{SpaceMappings: true, Multiline: Quoted},
			"t: \"a\\nb\\n\"\nu: 1\n"},
		// A blank line after a keep block would be read as part of the string.
		{"keep chomping after", `{"t":"a\n\n","u":1}`, Style{SpaceMappings: true},
			"t: |+\n  a\n\nu: 1\n"},
		{"keep chomping nested", `{"o":{"t":"a\n\n"},"u":1}`, Style{SpaceMappings: true},
			"o:\n  t: |+\n    a\n\nu: 1\n"},
		{"keep chomping before", `{"t":"a\n\n","o":{"k":1}}`, Style{SpaceMappings: true, SpaceBefore: true},
			"t: |+\n  a\n\no:\n  k: 1\n"},
	}
	for _, tc := range cases {
		got, err := FromJSONStyle([]byte(tc.in), tc.style)
		if err != nil {
			t.Errorf("%s: error = %v", tc.name, err)
			continue
		}
		if string(got) != tc.want {
			t.Errorf("%s =\n%q\nwant\n%q", tc.name, got, tc.want)
		}
		back, err := tojson.FromYAML(got)
		if err != nil {
			t.Errorf("%s: FromYAML(%q) error = %v", tc.name, got, err)
			continue
		}
		if !sameJSON(t, []byte(tc.in), back) {
			t.Errorf("%s: round trip mismatch: %s", tc.name, back)
		}
	}
}

// Style changes the shape of the output, never the document.
func TestStyleRoundTrip(t *testing.T) {
	docs := []string{
		`{"a":1,"b":[1,2,3],"c":{"d":"e"}}`,
		`[{"name":"one","tags":["x","y"]},{"name":"two","tags":[]}]`,
		`{"text":"line one\nline two\n","note":"x: y","empty":"","n":null}`,
		`{"deep":{"a":{"b":{"c":[{"d":[[1,2],[3]]}]}}}}`,
		`{"a":{},"b":[],"c":[[]],"d":[{}]}`,
		`{"nums":[0,-1,1.5,1e10],"strs":["true","0","null","-","#"]}`,
	}
	for _, in := range docs {
		for _, style := range styleMatrix {
			y, err := FromJSONStyle([]byte(in), style)
			if err != nil {
				t.Fatalf("FromJSONStyle(%s, %+v) error = %v", in, style, err)
			}
			back, err := tojson.FromYAML(y)
			if err != nil {
				t.Fatalf("FromYAML of %+v output: %v\n%s", style, err, y)
			}
			if !sameJSON(t, []byte(in), back) {
				t.Errorf("style %+v changed the document\n  in:   %s\n  back: %s\n%s",
					style, in, back, y)
			}
		}
	}
}

// A literal block carries a quote, a backslash or a tab as itself. Sending
// those to a quoted scalar was needless, and made ordinary prose come out
// JSON-style.
func TestBlockScalarLiteralContent(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"t":"he said \"hi\"\nnext\n"}`, "t: |\n  he said \"hi\"\n  next\n"},
		{`{"t":"C:\\dir\nnext\n"}`, "t: |\n  C:\\dir\n  next\n"},
		{`{"t":"col1\tcol2\nrow\n"}`, "t: |\n  col1\tcol2\n  row\n"},
		{`{"t":"a\t\"b\"\\c\nsecond\n"}`, "t: |\n  a\t\"b\"\\c\n  second\n"},
		{`{"t":"key: value\nnext\n"}`, "t: |\n  key: value\n  next\n"},
		{`{"t":"# heading\nnext\n"}`, "t: |\n  # heading\n  next\n"},
		{`{"t":"- item\nnext\n"}`, "t: |\n  - item\n  next\n"},
		{`{"t":"---\n...\n"}`, "t: |\n  ---\n  ...\n"},
		// a control character still needs quoting
		{`{"t":"a\u0007b\nc\n"}`, "t: \"a\\u0007b\\nc\\n\"\n"},
		// so does a delete, which YAML excludes from its printable set
		{`{"t":"a\u007fb\nc\n"}`, "t: \"a\\u007fb\\nc\\n\"\n"},
	}
	for _, tc := range cases {
		got, err := FromJSON([]byte(tc.in))
		if err != nil {
			t.Errorf("FromJSON(%s) error = %v", tc.in, err)
			continue
		}
		if string(got) != tc.want {
			t.Errorf("FromJSON(%s) =\n%q\nwant\n%q", tc.in, got, tc.want)
		}
		back, err := tojson.FromYAML(got)
		if err != nil {
			t.Errorf("FromYAML(%q) error = %v", got, err)
			continue
		}
		if !sameJSON(t, []byte(tc.in), back) {
			t.Errorf("round trip mismatch for %s: %s", tc.in, back)
		}
	}
}

// Trailing blank lines are kept with "|+" rather than sending the string to a
// quoted scalar.
func TestBlockScalarKeepChomping(t *testing.T) {
	for _, in := range []string{
		`{"t":"a\n\n"}`,
		`{"t":"a\nb\n\n"}`,
		`{"t":"a\nb\n\n\n\n"}`,
		`{"t":"a\n\nb\n\n"}`,
		`["x\n\n"]`,
		`{"outer":{"t":"a\n\n"},"after":1}`,
		`{"before":1,"t":"a\n\n","list":[1,2],"u":{"v":"b\n\n"},"w":2}`,
		`[["x\n\n"],"y",{"z":"a\n\n"},[1,2]]`,
	} {
		for _, style := range styleMatrix {
			y, err := FromJSONStyle([]byte(in), style)
			if err != nil {
				t.Fatalf("FromJSONStyle(%s, %+v) error = %v", in, style, err)
			}
			back, err := tojson.FromYAML(y)
			if err != nil {
				t.Fatalf("FromYAML(%q) error = %v", y, err)
			}
			if !sameJSON(t, []byte(in), back) {
				t.Errorf("style %+v: round trip mismatch for %s\n got: %s\nyaml: %q", style, in, back, y)
			}
		}
	}
}

// A string that cannot be written plain is quoted, and Quote picks which
// quotes. The single-quoted form escapes nothing but a quote of its own, so it
// is used only where it can hold the string as itself.
func TestQuoteStyle(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		style Style
		want  string
	}{
		// single quotes chosen outright
		{"singleDoubleQuote", `{"k":"say \"hi\""}`, Style{Quote: QuoteSingle},
			"k: 'say \"hi\"'\n"},
		{"singleBackslash", `{"k":"C:\\tmp\\x"}`, Style{Quote: QuoteSingle},
			"k: 'C:\\tmp\\x'\n"},
		{"singleQuoteDoubled", `{"k":"'q'"}`, Style{Quote: QuoteSingle},
			"k: '''q'''\n"},
		{"singleAmbiguous", `{"a":"123","b":"yes","c":"","d":" pad "}`, Style{Quote: QuoteSingle},
			"a: '123'\nb: 'yes'\nc: ''\nd: ' pad '\n"},
		{"singleKey", `{"say \"hi\"":1}`, Style{Quote: QuoteSingle},
			"'say \"hi\"': 1\n"},
		{"singleQuoteKey", `{"'":1}`, Style{Quote: QuoteSingle},
			"'''': 1\n"},
		// whitespace outside ASCII keeps a string out of a plain scalar, but
		// quotes of either kind carry it
		{"singleNoBreakSpace", `{"k":"x\u00a0"}`, Style{Quote: QuoteSingle},
			"k: 'x\u00a0'\n"},

		// what a single-quoted scalar cannot hold falls back to the double form
		{"singleFallbackTab", `{"k":"a\tb"}`, Style{Quote: QuoteSingle},
			"k: \"a\\tb\"\n"},
		{"singleFallbackDelete", `{"k":"a\u007fb"}`, Style{Quote: QuoteSingle},
			"k: \"a\\u007fb\"\n"},
		{"singleFallbackC1", `{"k":"a\u0081b"}`, Style{Quote: QuoteSingle},
			"k: \"a\\u0081b\"\n"},
		{"singleFallbackLineSeparator", `{"k":"a\u2028b"}`, Style{Quote: QuoteSingle},
			"k: \"a\\u2028b\"\n"},
		// Quote is independent of Multiline: a line break rules the single form
		// out, and is invisible to the block path
		{"singleFallbackNewline", `{"k":"a\nb"}`, Style{Quote: QuoteSingle, Multiline: Quoted},
			"k: \"a\\nb\"\n"},
		{"singleLeavesBlockAlone", `{"k":"a\nb"}`, Style{Quote: QuoteSingle},
			"k: |-\n  a\n  b\n"},
		{"singleAfterBlockDeclines", `{"t":"a \nb\n"}`, Style{Quote: QuoteSingle},
			"t: \"a \\nb\\n\"\n"},

		// QuoteDouble is the JSON spelling throughout
		{"doubleAmbiguous", `{"a":"123","b":"yes","c":"","d":" pad "}`, Style{Quote: QuoteDouble},
			"a: \"123\"\nb: \"yes\"\nc: \"\"\nd: \" pad \"\n"},
		{"doubleBackslash", `{"k":"C:\\tmp"}`, Style{Quote: QuoteDouble},
			"k: \"C:\\\\tmp\"\n"},
		{"doubleKey", `{"1":"a"}`, Style{Quote: QuoteDouble},
			"\"1\": a\n"},

		// adaptive: single where it escapes no more, double where it escapes less
		{"adaptiveGainQuote", `{"k":"say \"hi\""}`, Style{Quote: QuoteAdaptive},
			"k: 'say \"hi\"'\n"},
		{"adaptiveGainBackslash", `{"k":"C:\\x"}`, Style{Quote: QuoteAdaptive},
			"k: 'C:\\x'\n"},
		{"adaptiveLossApostrophe", `{"k":"'a'\""}`, Style{Quote: QuoteAdaptive},
			"k: \"'a'\\\"\"\n"},
		// a tie goes to single quotes: the two spellings are the same length,
		// and the single-quoted one carries less punctuation
		{"adaptiveTie", `{"a":"123","b":"a: b","c":""}`, Style{Quote: QuoteAdaptive},
			"a: '123'\nb: 'a: b'\nc: ''\n"},
		{"adaptiveMixed", `{"k":"it's \"x\""}`, Style{Quote: QuoteAdaptive},
			"k: 'it''s \"x\"'\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FromJSONStyle([]byte(tc.in), tc.style)
			if err != nil {
				t.Fatalf("FromJSONStyle(%s, %+v) error = %v", tc.in, tc.style, err)
			}
			if string(got) != tc.want {
				t.Errorf("FromJSONStyle(%s, %+v)\n got: %q\nwant: %q", tc.in, tc.style, got, tc.want)
			}
			back, err := tojson.FromYAML(got)
			if err != nil {
				t.Fatalf("FromYAML(%q) error = %v", got, err)
			}
			if !sameJSON(t, []byte(tc.in), back) {
				t.Errorf("%s changed the document: %s", tc.name, back)
			}
		})
	}
}

// The zero value quotes adaptively, so FromJSON itself picks the lighter form.
func TestQuoteStyleDefault(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"k":"C:\\Users\\x"}`, "k: 'C:\\Users\\x'\n"},
		{`{"k":"say \"hi\""}`, "k: 'say \"hi\"'\n"},
		{`{"k":"123"}`, "k: '123'\n"},
		{`{"k":"'a'\""}`, "k: \"'a'\\\"\"\n"},
	}
	for _, tc := range cases {
		got, err := FromJSON([]byte(tc.in))
		if err != nil {
			t.Fatalf("FromJSON(%s) error = %v", tc.in, err)
		}
		if string(got) != tc.want {
			t.Errorf("FromJSON(%s)\n got: %q\nwant: %q", tc.in, got, tc.want)
		}
	}
}

func TestSingleSafe(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"a", true},
		{"a'b", true},
		{"a\"b\\c", true},
		{"a\u00a0b", true}, // a no-break space is content inside quotes
		{"a\u3000b", true}, // and so is an ideographic space
		{"a\tb", false},    // a tab is turned away by choice, not by necessity
		{"a\nb", false},    // a line break inside the quotes folds to a space
		{"a\rb", false},
		{"a\x00b", false},
		{"a\u007fb", false}, // DEL, outside YAML's printable set
		{"a\u0085b", false}, // NEL, a C1 control and a line break
		{"a\u009fb", false},
		{"a\u2028b", false},
		{"a\u2029b", false},
	}
	for _, tc := range cases {
		if got := singleSafe([]byte(tc.in)); got != tc.want {
			t.Errorf("singleSafe(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSingleGain(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"plain text", 0},
		{"a: b", 0},
		{`say "hi"`, 2},
		{`C:\tmp\x`, 2},
		{"it's", -1},
		{"'''", -3},
		{`it's "x"`, 1},
		{`'"`, 0},
	}
	for _, tc := range cases {
		if got := singleGain([]byte(tc.in)); got != tc.want {
			t.Errorf("singleGain(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
