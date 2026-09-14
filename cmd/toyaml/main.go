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
//	toyaml -compact-seq file.json
//	toyaml -space-map -space-seq -space-before -space-level 1 file.json
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
	compactSeq := flag.Bool("compact-seq", false, "put a sequence at the indentation of its key")
	spaceMap := flag.Bool("space-map", false, "blank line between mapping entries next to a multi-line value")
	spaceSeq := flag.Bool("space-seq", false, "blank line between sequence items next to a multi-line value")
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
		fatalf("usage: toyaml [-f json|yaml] [-indent n] [-multiline block|quoted] [-compact-seq] [-space-map] [-space-seq] [-space-before] [-space-level n] [file]")
	}

	ml, err := multilineStyle(*multiline)
	if err != nil {
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
		CompactSequence: *compactSeq,
		SpaceMappings:   *spaceMap,
		SpaceSequences:  *spaceSeq,
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
