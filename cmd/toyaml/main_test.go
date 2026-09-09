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
