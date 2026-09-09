// toyaml converts a JSON document from a file or stdin to YAML on stdout.
//
// Usage:
//
//	toyaml file.json
//	cat file.json | toyaml
//	toyaml -indent 4 file.json
//	toyaml -multiline quoted file.json
//	toyaml -compact-seq file.json
//
// To convert YAML, TOML, or a JSON variant such as JSON5, convert it to JSON
// first with the tojson command and pipe the result in.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/client9/toyaml"
)

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
	indent := flag.Int("indent", 0, "spaces per nesting level (default 2)")
	multiline := flag.String("multiline", "block", "how to write multi-line strings: block or quoted")
	compactSeq := flag.Bool("compact-seq", false, "put a sequence at the indentation of its key")
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
		fatalf("usage: toyaml [-indent n] [-multiline block|quoted] [-compact-seq] [file]")
	}

	ml, err := multilineStyle(*multiline)
	if err != nil {
		fatalf("%v", err)
	}

	input, err := readInput(flag.Args())
	if err != nil {
		fatalf("%v", err)
	}

	out, err := toyaml.FromJSONStyle(input, toyaml.Style{
		Indent:          *indent,
		Multiline:       ml,
		CompactSequence: *compactSeq,
	})
	if err != nil {
		fatalf("%v", err)
	}

	// FromJSONStyle already ends its output with a newline.
	if _, err := os.Stdout.Write(out); err != nil {
		fatalf("writing stdout: %v", err)
	}
}
