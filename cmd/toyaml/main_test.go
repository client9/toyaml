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

func TestSequenceSpacing(t *testing.T) {
	cases := []struct {
		in      string
		want    toyaml.SequenceSpacing
		wantErr bool
	}{
		{"none", toyaml.SpaceSeqNone, false},
		{"None", toyaml.SpaceSeqNone, false},
		{"multiline", toyaml.SpaceSeqMultiline, false},
		{"MULTILINE", toyaml.SpaceSeqMultiline, false},
		{"mappings", toyaml.SpaceSeqMappings, false},
		{"Mappings", toyaml.SpaceSeqMappings, false},
		{"", 0, true},
		{"mapping", 0, true},
		{"true", 0, true},
	}
	for _, tc := range cases {
		got, err := sequenceSpacing(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("sequenceSpacing(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			continue
		}
		if err == nil && got != tc.want {
			t.Errorf("sequenceSpacing(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestCheckSpacing(t *testing.T) {
	cases := []struct {
		name     string
		spaceMap bool
		seq      toyaml.SequenceSpacing
		afterKey bool
		before   bool
		level    int
		wantErr  string
	}{
		{name: "nothing set"},
		{name: "mappings only", spaceMap: true},
		{name: "sequences only", seq: toyaml.SpaceSeqMultiline},
		{name: "after key only", afterKey: true},
		{name: "before with mappings", spaceMap: true, before: true},
		{name: "before with sequences", seq: toyaml.SpaceSeqMappings, before: true},
		{name: "level with mappings", spaceMap: true, level: 2},
		// -space-level bounds -space-after-key too, so it has work to do here
		{name: "level with after key", afterKey: true, level: 2},
		{name: "before alone", before: true,
			wantErr: "-space-before needs -space-map or -space-seq"},
		// -space-before widens the gap between entries; -space-after-key is a
		// different gap and leaves it with nothing to widen
		{name: "before with after key", afterKey: true, before: true,
			wantErr: "-space-before needs -space-map or -space-seq"},
		{name: "level alone", level: 2,
			wantErr: "-space-level needs -space-map or -space-seq"},
		{name: "both alone", before: true, level: 1,
			wantErr: "-space-before and -space-level need -space-map or -space-seq"},
		// a bad value is the library's complaint to make, not ours
		{name: "negative level alone", level: -1},
	}
	for _, tc := range cases {
		err := checkSpacing(tc.spaceMap, tc.seq, tc.afterKey, tc.before, tc.level)
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
