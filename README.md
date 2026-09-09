# toyaml

Directly converts JSON documents to YAML. Standard library only.

[![Go Reference](https://pkg.go.dev/badge/github.com/client9/toyaml.svg)](https://pkg.go.dev/github.com/client9/toyaml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

`toyaml` takes JSON bytes and writes block-style YAML bytes. There is no
reflection and no intermediate data structure: tokens come from
`encoding/json/jsontext` and YAML goes straight into a byte slice.

It is the outbound half of [tojson](https://github.com/client9/tojson), which
converts YAML, TOML, JSON variants, and front matter into JSON. Together they
convert between any two of those formats, with JSON in the middle.

## Requirements

Go 1.27 or later, for `encoding/json/jsontext`.

The package itself uses only the standard library. `go.mod` requires
[tojson](https://github.com/client9/tojson), but only the tests import it, to
read the YAML back and confirm the document survived the trip. It is never
compiled into anything that imports `toyaml`.

## Quick Start

```bash
go get github.com/client9/toyaml
```

```go
package main

import (
	"fmt"
	"log"

	"github.com/client9/toyaml"
)

func main() {
	out, err := toyaml.FromJSON([]byte(`{"name":"widget","tags":["a","b"]}`))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s", out)
}
```

```yaml
name: widget
tags:
  - a
  - b
```

## Style

`FromJSONStyle` takes a `Style` controlling the shape of the output. Style never
changes the document: whatever the settings, reading the output back yields the
same data.

| Field | Meaning | Default |
| --- | --- | --- |
| `Indent` | spaces per nesting level | 2 |
| `Multiline` | `BlockLiteral` or `Quoted` for strings containing newlines | `BlockLiteral` |
| `CompactSequence` | put a sequence at the indentation of its key | false |

```go
out, err := toyaml.FromJSONStyle(src, toyaml.Style{Indent: 4, CompactSequence: true})
```

## How values are written

- **Strings** are written as plain scalars where that reads back unchanged, as
  literal blocks (`|`) where they contain newlines, and as double-quoted
  scalars otherwise. A string a YAML reader would resolve to a bool, null, a
  number, or a timestamp is quoted, so `"0x10"` and `"yes"` come back as
  strings.
- **Numbers** pass through as written, with no evaluation, so a value too large
  for `float64` keeps its digits.
- **Empty containers** are written in flow form, `{}` and `[]`, since block
  form has no way to spell them.
- **A sequence nested directly in a sequence** gets the `-` on its own line
  rather than the compact `- - 1`. Both are legal; the split form states the
  nested sequence's indentation instead of implying it.

## What it rejects

Input that is not valid JSON is reported as `*jsontext.SyntacticError`, which
carries a byte offset and a JSON pointer. That includes duplicate object member
names: a YAML mapping with a repeated key is not a document that reads back as
what was written.

Valid JSON with no YAML rendering here is reported as `*EncodeError`, wrapping
one of `ErrKeyTooLong`, `ErrMaxDepth`, or `ErrTrailingValue`. Test for those
with `errors.Is`.

An object key may not exceed 1024 characters, which is as far as YAML lets a
mapping key reach before its `:`. Nesting is capped at 200 levels.

## Command line

```bash
go install github.com/client9/toyaml/cmd/toyaml@latest
```

```bash
toyaml file.json
cat file.json | toyaml
toyaml -indent 4 file.json
toyaml -multiline quoted file.json
toyaml -compact-seq file.json
```

To convert something that is not JSON, convert it to JSON first:

```bash
tojson -raw config.toml | toyaml
```

## License

MIT
