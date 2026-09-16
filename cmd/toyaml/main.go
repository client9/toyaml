// toyaml converts a JSON or YAML document from a file or stdin to YAML on
// stdout. Given YAML, it reformats it in the chosen style.
//
// Usage:
//
//	toyaml file.json
//	toyaml file.yaml             # format inferred from extension
//	cat file.json | toyaml
//	cat file.yaml | toyaml -f yaml
//	toyaml -indent 4 file.json
//	toyaml -multiline quoted file.json
//	toyaml -quote double file.json        # or single, or adaptive, the default
//	toyaml -compact-seq file.json
//	toyaml -space-map -space-seq multiline -space-before -space-level 1 file.json
//	toyaml -space-seq mappings file.json  # spaces one-line mappings too
//	toyaml -space-after-key file.json     # opens a nested value with a blank line
//
// YAML is read with github.com/client9/tojson, which accepts a subset of YAML
// and does not keep comments, so reformatting drops them. To convert TOML or a
// JSON variant such as JSON5, convert it to JSON first with the tojson command
// and pipe the result in.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/client9/tojson"
	"github.com/client9/toyaml"
)

// inputFormat returns the format named by the -f flag, or else the one the
// file's extension implies. Stdin and any other extension are JSON.
func inputFormat(flagValue string, args []string) (string, error) {
	name := strings.ToLower(flagValue)
	if name == "" && len(args) > 0 {
		name = strings.TrimPrefix(strings.ToLower(filepath.Ext(args[0])), ".")
	}
	switch name {
	case "yaml", "yml":
		return "yaml", nil
	case "json":
		return "json", nil
	case "":
		return "json", nil
	}
	if flagValue == "" {
		return "json", nil // an unrecognized extension is read as JSON
	}
	return "", fmt.Errorf("unknown input format %q, want json or yaml", flagValue)
}

// convert writes the input, in the given format, as YAML in style.
func convert(format string, input []byte, style toyaml.Style) ([]byte, error) {
	if format == "yaml" {
		j, err := tojson.FromYAML(input)
		if err != nil {
			return nil, err
		}
		input = j
	}
	return toyaml.FromJSONStyle(input, style)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "toyaml: "+format+"\n", args...)
	os.Exit(1)
}

// multilineStyle maps the -multiline flag to the style it names.
func multilineStyle(name string) (toyaml.MultilineStyle, error) {
	switch strings.ToLower(name) {
	case "block":
		return toyaml.BlockLiteral, nil
	case "quoted":
		return toyaml.Quoted, nil
	}
	return 0, fmt.Errorf("unknown multiline style %q, want block or quoted", name)
}

// quoteStyle maps the -quote flag to the style it names.
func quoteStyle(name string) (toyaml.QuoteStyle, error) {
	switch strings.ToLower(name) {
	case "adaptive":
		return toyaml.QuoteAdaptive, nil
	case "double":
		return toyaml.QuoteDouble, nil
	case "single":
		return toyaml.QuoteSingle, nil
	}
	return 0, fmt.Errorf("unknown quote style %q, want adaptive, double or single", name)
}

// sequenceSpacing maps the -space-seq flag to the spacing it names.
func sequenceSpacing(name string) (toyaml.SequenceSpacing, error) {
	switch strings.ToLower(name) {
	case "none":
		return toyaml.SpaceSeqNone, nil
	case "multiline":
		return toyaml.SpaceSeqMultiline, nil
	case "mappings":
		return toyaml.SpaceSeqMappings, nil
	}
	return 0, fmt.Errorf("unknown sequence spacing %q, want none, multiline or mappings", name)
}

// checkSpacing reports a spacing flag that cannot do anything, which is
// -space-before or -space-level with no spacing turned on for it to qualify.
// Both read as qualifiers rather than instructions, so passing one alone is a
// half-written command rather than a request for nothing, and saying so beats
// printing the input back unchanged.
//
// A negative -space-level falls through to the library, whose complaint about
// the value itself is the more useful one.
func checkSpacing(spaceMap bool, seq toyaml.SequenceSpacing, afterKey, before bool, level int) error {
	siblings := spaceMap || seq != toyaml.SpaceSeqNone
	var inert []string
	// -space-before widens the gap between two entries, so it needs spacing
	// between entries; -space-after-key is a different gap and does not count.
	if before && !siblings {
		inert = append(inert, "-space-before")
	}
	// -space-level bounds every kind of blank line, -space-after-key included.
	if level > 0 && !siblings && !afterKey {
		inert = append(inert, "-space-level")
	}
	if inert == nil {
		return nil
	}
	needs := "needs"
	if len(inert) > 1 {
		needs = "need"
	}
	return fmt.Errorf("%s %s -space-map or -space-seq",
		strings.Join(inert, " and "), needs)
}

func readInput(args []string) ([]byte, error) {
	if len(args) == 0 {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(args[0])
}

func main() {
	format := flag.String("f", "", "input format: json or yaml (default from file extension, else json)")
	indent := flag.Int("indent", 0, "spaces per nesting level (default 2)")
	multiline := flag.String("multiline", "block", "how to write multi-line strings: block or quoted")
	quote := flag.String("quote", "adaptive", "quotes around a string that cannot be plain: adaptive, double or single")
	compactSeq := flag.Bool("compact-seq", false, "put a sequence at the indentation of its key")
	spaceMap := flag.Bool("space-map", false, "blank line between mapping entries next to a multi-line value")
	spaceSeq := flag.String("space-seq", "none", "blank line between sequence items: none, multiline, or mappings to count a one-line mapping too")
	spaceAfterKey := flag.Bool("space-after-key", false, "blank line between a mapping key and a nested mapping or sequence")
	spaceBefore := flag.Bool("space-before", false, "also put a blank line before a multi-line value, not only after")
	spaceLevel := flag.Int("space-level", 0, "space only containers nested at most this deep (default every level)")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		v := "(devel)"
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
			v = info.Main.Version
		}
		fmt.Println(v)
		os.Exit(0)
	}

	if flag.NArg() > 1 {
		fatalf("usage: toyaml [-f json|yaml] [-indent n] [-multiline block|quoted] [-quote adaptive|double|single] [-compact-seq] [-space-map] [-space-seq none|multiline|mappings] [-space-after-key] [-space-before] [-space-level n] [file]")
	}

	ml, err := multilineStyle(*multiline)
	if err != nil {
		fatalf("%v", err)
	}
	q, err := quoteStyle(*quote)
	if err != nil {
		fatalf("%v", err)
	}
	seq, err := sequenceSpacing(*spaceSeq)
	if err != nil {
		fatalf("%v", err)
	}
	if err := checkSpacing(*spaceMap, seq, *spaceAfterKey, *spaceBefore, *spaceLevel); err != nil {
		fatalf("%v", err)
	}
	inFormat, err := inputFormat(*format, flag.Args())
	if err != nil {
		fatalf("%v", err)
	}

	input, err := readInput(flag.Args())
	if err != nil {
		fatalf("%v", err)
	}

	out, err := convert(inFormat, input, toyaml.Style{
		Indent:          *indent,
		Multiline:       ml,
		Quote:           q,
		CompactSequence: *compactSeq,
		SpaceMappings:   *spaceMap,
		SpaceSequences:  seq,
		SpaceAfterKey:   *spaceAfterKey,
		SpaceBefore:     *spaceBefore,
		SpaceMaxLevel:   *spaceLevel,
	})
	if err != nil {
		fatalf("%v", err)
	}

	// FromJSONStyle already ends its output with a newline.
	if _, err := os.Stdout.Write(out); err != nil {
		fatalf("writing stdout: %v", err)
	}
}
