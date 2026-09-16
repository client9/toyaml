// Package toyaml converts JSON documents to block-style YAML.
//
// It reads standard JSON only, using encoding/json/jsontext for tokenizing,
// and writes YAML directly to a byte slice with no reflection and no
// intermediate data structure.
//
//	out, err := toyaml.FromJSON(src)
//
// FromJSONStyle takes a Style to set the indent width, whether multi-line
// strings use literal blocks or JSON-style quoting, which quotes a quoted
// scalar gets, whether sequences are indented under their key, and where blank
// lines set off multi-line values. Style never changes the document. Whatever
// the settings, reading the output back yields the same data.
//
// Strings are written as plain scalars where that round-trips, as literal
// blocks ("|") where they contain newlines, and as quoted scalars otherwise.
// Which quotes a quoted scalar gets is Style.Quote: by default whichever of
// the two escapes less, which puts single quotes around a string carrying a
// backslash or a double quote and double quotes around one carrying an
// apostrophe. Numbers are written through as they appear in the input, so a
// value too large for float64 keeps its digits.
//
// To convert YAML, TOML, or a JSON variant such as JSON5 to YAML, convert it
// to JSON first with github.com/client9/tojson and pass the result here.
//
// Input that is not valid JSON is reported as *jsontext.SyntacticError, which
// carries a byte offset and a JSON pointer. A document that is valid JSON but
// has no YAML rendering this package will write is reported as *EncodeError.
package toyaml
