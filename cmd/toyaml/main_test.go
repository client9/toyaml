package main

import (
	"testing"

	"github.com/client9/toyaml"
)

func TestMultilineStyle(t *testing.T) {
	cases := []struct {
		in      string
		want    toyaml.MultilineStyle
		wantErr bool
	}{
		{"block", toyaml.BlockLiteral, false},
		{"BLOCK", toyaml.BlockLiteral, false},
		{"quoted", toyaml.Quoted, false},
		{"Quoted", toyaml.Quoted, false},
		{"", 0, true},
		{"literal", 0, true},
	}
	for _, tc := range cases {
		got, err := multilineStyle(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("multilineStyle(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			continue
		}
		if err == nil && got != tc.want {
			t.Errorf("multilineStyle(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestQuoteStyle(t *testing.T) {
	cases := []struct {
		in      string
		want    toyaml.QuoteStyle
		wantErr bool
	}{
		{"adaptive", toyaml.QuoteAdaptive, false},
		{"ADAPTIVE", toyaml.QuoteAdaptive, false},
		{"double", toyaml.QuoteDouble, false},
		{"Double", toyaml.QuoteDouble, false},
		{"single", toyaml.QuoteSingle, false},
		{"SINGLE", toyaml.QuoteSingle, false},
		{"", 0, true},
		{"literal", 0, true},
	}
	for _, tc := range cases {
		got, err := quoteStyle(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("quoteStyle(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			continue
		}
		if err == nil && got != tc.want {
			t.Errorf("quoteStyle(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// Neither sequence-spacing flag is a no-op on its own, and the wider one wins.
func TestSequenceSpacing(t *testing.T) {
	cases := []struct {
		seq, maps bool
		want      toyaml.SequenceSpacing
	}{
		{false, false, toyaml.SpaceSeqNone},
		{true, false, toyaml.SpaceSeqMultiline},
		{false, true, toyaml.SpaceSeqMappings},
		{true, true, toyaml.SpaceSeqMappings},
	}
	for _, tc := range cases {
		if got := sequenceSpacing(tc.seq, tc.maps); got != tc.want {
			t.Errorf("sequenceSpacing(%v, %v) = %v, want %v", tc.seq, tc.maps, got, tc.want)
		}
	}
}

func TestCheckSpacing(t *testing.T) {
	cases := []struct {
		name     string
		spaceMap bool
		seq      toyaml.SequenceSpacing
		before   bool
		level    int
		wantErr  string
	}{
		{name: "nothing set"},
		{name: "mappings only", spaceMap: true},
		{name: "sequences only", seq: toyaml.SpaceSeqMultiline},
		{name: "before with mappings", spaceMap: true, before: true},
		{name: "before with sequences", seq: toyaml.SpaceSeqMappings, before: true},
		{name: "level with mappings", spaceMap: true, level: 2},
		{name: "before alone", before: true,
			wantErr: "-space-before needs -space-map, -space-seq or -space-seq-maps"},
		{name: "level alone", level: 2,
			wantErr: "-space-level needs -space-map, -space-seq or -space-seq-maps"},
		{name: "both alone", before: true, level: 1,
			wantErr: "-space-before and -space-level need -space-map, -space-seq or -space-seq-maps"},
		// a bad value is the library's complaint to make, not ours
		{name: "negative level alone", level: -1},
	}
	for _, tc := range cases {
		err := checkSpacing(tc.spaceMap, tc.seq, tc.before, tc.level)
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%s: checkSpacing() = %v, want nil", tc.name, err)
		case tc.wantErr != "" && err == nil:
			t.Errorf("%s: checkSpacing() = nil, want %q", tc.name, tc.wantErr)
		case tc.wantErr != "" && err.Error() != tc.wantErr:
			t.Errorf("%s: checkSpacing() = %q, want %q", tc.name, err, tc.wantErr)
		}
	}
}

func TestInputFormat(t *testing.T) {
	cases := []struct {
		flag    string
		args    []string
		want    string
		wantErr bool
	}{
		{"", nil, "json", false},
		{"", []string{"a.json"}, "json", false},
		{"", []string{"a.yaml"}, "yaml", false},
		{"", []string{"a.YML"}, "yaml", false},
		{"", []string{"a.txt"}, "json", false},
		{"", []string{"noext"}, "json", false},
		{"yaml", nil, "yaml", false},
		{"YAML", nil, "yaml", false},
		{"yml", []string{"a.json"}, "yaml", false},
		{"json", []string{"a.yaml"}, "json", false},
		{"toml", nil, "", true},
	}
	for _, tc := range cases {
		got, err := inputFormat(tc.flag, tc.args)
		if (err != nil) != tc.wantErr {
			t.Errorf("inputFormat(%q, %q) error = %v, wantErr %v", tc.flag, tc.args, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("inputFormat(%q, %q) = %q, want %q", tc.flag, tc.args, got, tc.want)
		}
	}
}

func TestConvertYAML(t *testing.T) {
	input, err := readInput([]string{"testdata/sample.yaml"})
	if err != nil {
		t.Fatalf("readInput() error = %v", err)
	}
	got, err := convert("yaml", input, toyaml.Style{Indent: 4, SpaceMappings: true})
	if err != nil {
		t.Fatalf("convert() error = %v", err)
	}
	want := "name: app\nlist:\n    - 1\n    - two\n\nnested:\n    k: v\n"
	if string(got) != want {
		t.Errorf("convert() =\n%q\nwant\n%q", got, want)
	}
}

func TestConvertJSON(t *testing.T) {
	got, err := convert("json", []byte(`{"a":[1]}`), toyaml.Style{})
	if err != nil {
		t.Fatalf("convert() error = %v", err)
	}
	if want := "a:\n  - 1\n"; string(got) != want {
		t.Errorf("convert() = %q, want %q", got, want)
	}
	// JSON stays strict: YAML input is not accepted in JSON mode
	if _, err := convert("json", []byte("a: 1\n"), toyaml.Style{}); err == nil {
		t.Error("convert(json, YAML input) error = nil, want error")
	}
}

// Style.Quote reaches the converter like every other flag.
func TestConvertQuoteStyle(t *testing.T) {
	got, err := convert("json", []byte(`{"k":"say \"hi\""}`), toyaml.Style{Quote: toyaml.QuoteSingle})
	if err != nil {
		t.Fatalf("convert() error = %v", err)
	}
	if want := "k: 'say \"hi\"'\n"; string(got) != want {
		t.Errorf("convert() = %q, want %q", got, want)
	}
}

func TestConvertYAMLError(t *testing.T) {
	if _, err := convert("yaml", []byte("a: [1\n"), toyaml.Style{}); err == nil {
		t.Error("convert(yaml, unterminated flow) error = nil, want error")
	}
}

func TestReadInputFile(t *testing.T) {
	got, err := readInput([]string{"testdata/sample.json"})
	if err != nil {
		t.Fatalf("readInput() error = %v", err)
	}
	if want := "{\"a\":1}\n"; string(got) != want {
		t.Errorf("readInput() = %q, want %q", got, want)
	}
}

func TestReadInputMissingFile(t *testing.T) {
	if _, err := readInput([]string{"testdata/does-not-exist.json"}); err == nil {
		t.Error("readInput() error = nil, want error")
	}
}
