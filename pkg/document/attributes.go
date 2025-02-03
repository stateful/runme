package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/pkg/errors"
	"github.com/yuin/goldmark/ast"
	"go.uber.org/multierr"
)

// Attributes stores the attributes of a code block along with the format.
// More: https://docs.runme.dev/configuration/cell-level
type Attributes struct {
	Format string
	Items  map[string]string
}

// NewAttributes creates a new [Attributes] instance.
// Check out the [NewAttributesWithFormat] function for more options.
func NewAttributes(items map[string]string) *Attributes {
	return NewAttributesWithFormat(items, "json")
}

// NewAttributesWithFormat creates a new [Attributes] instance with the specified format.
func NewAttributesWithFormat(items map[string]string, format string) *Attributes {
	if items == nil {
		items = make(map[string]string)
	}
	return &Attributes{
		Format: format,
		Items:  items,
	}
}

func (a *Attributes) Clone() *Attributes {
	clone := Attributes{
		Format: a.Format,
		Items:  make(map[string]string, len(a.Items)),
	}
	for k, v := range a.Items {
		clone.Items[k] = v
	}
	return &clone
}

func (a *Attributes) Equal(other Attributes) bool {
	if a.Format != other.Format {
		return false
	}

	if len(a.Items) != len(other.Items) {
		return false
	}

	for k, v := range a.Items {
		if otherValue, exists := other.Items[k]; !exists || v != otherValue {
			return false
		}
	}

	return true
}

func newAttributesFromFencedCodeBlock(
	node *ast.FencedCodeBlock,
	source []byte,
) (*Attributes, error) {
	attr := NewAttributes(nil)

	if node.Info == nil {
		return attr, nil
	}

	content := node.Info.Value(source)

	rawAttributes := extractAttributes(content)
	if len(rawAttributes) == 0 {
		return attr, nil
	}
	return parseAttributes(rawAttributes)
}

type attrParserWriter interface {
	Parse([]byte) (*Attributes, error)
	Write(io.Writer, *Attributes) error
}

var _defaultAttrParserWriter = &multiParserWriter{
	items: []struct {
		format       attrFormat
		parserWriter attrParserWriter
	}{
		// The order is important here. The first parsers should be those
		// that are the most strict. The goal is to keep the attributes
		// in the same format and use fallback only when necessary.
		{format: "json", parserWriter: &jsonAttrParserWriter{}},
		{format: "html", parserWriter: &htmlAttrParserWriter{}},
	},
}

// WriteAttributes writes [Attributes] to [io.Writer].
func WriteAttributes(w io.Writer, attr *Attributes) error {
	return _defaultAttrParserWriter.Write(w, attr)
}

// parseAttributes parses [Attributes] from raw bytes.
func parseAttributes(raw []byte) (*Attributes, error) {
	return _defaultAttrParserWriter.Parse(raw)
}

// jsonAttrParserWriter parses all values as strings.
//
// The correct format is as follows:
//
//	{ key: "value", hello: "world", string_value: "2" }
type jsonAttrParserWriter struct{}

func (p *jsonAttrParserWriter) Parse(raw []byte) (*Attributes, error) {
	// Parse first to a generic map.
	parsed := make(map[string]interface{})
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, errors.WithStack(err)
	}

	attrMap := make(map[string]string, len(parsed))

	// Convert all values to strings.
	for k, v := range parsed {
		if strVal, ok := v.(string); ok {
			attrMap[k] = strVal
		} else {
			if stringified, err := json.Marshal(v); err == nil {
				attrMap[k] = string(stringified)
			}
		}
	}

	return NewAttributesWithFormat(attrMap, "json"), nil
}

func (p *jsonAttrParserWriter) Write(w io.Writer, attr *Attributes) error {
	// TODO: name at front...
	res, err := json.Marshal(attr.Items)
	if err != nil {
		return errors.WithStack(err)
	}
	_, err = w.Write(bytes.TrimSpace(res))
	return errors.WithStack(err)
}

// htmlAttrParserWriter parses and writes options as HTML attributes.
//
// For example:
//
//	{ key=value hello=world string_value=2 }
//
// HTML attributes is for compatability; prefer the JSON parser instead.
// note this parser will never error, it returns an empty map for invalid data
type htmlAttrParserWriter struct{}

func (p *htmlAttrParserWriter) Parse(raw []byte) (*Attributes, error) {
	rawAttr := extractAttributes(raw)
	rawAttr = bytes.Trim(rawAttr, "{}")
	attrMap := p.parseRawAttributes(rawAttr)
	return NewAttributesWithFormat(attrMap, "html"), nil
}

func (p *htmlAttrParserWriter) Write(w io.Writer, attr *Attributes) error {
	keys := p.getSortedKeys(attr)

	_, _ = w.Write([]byte{'{'})
	i := 0
	for _, k := range keys {
		if i == 0 {
			_, _ = w.Write([]byte{' '})
		}
		v := attr.Items[k]
		_, _ = w.Write([]byte(fmt.Sprintf("%s=%s ", k, v)))
		i++
	}
	_, _ = w.Write([]byte{'}'})

	return nil
}

func (*htmlAttrParserWriter) parseRawAttributes(raw []byte) map[string]string {
	items := bytes.Split(raw, []byte{' '})
	if len(items) == 0 {
		return nil
	}

	result := make(map[string]string)

	for _, item := range items {
		fragments := bytes.SplitN(item, []byte{'='}, 2)
		key, value := string(fragments[0]), ""
		if key == "" {
			continue
		}

		if len(fragments) == 2 {
			value = string(fragments[1])
		}

		result[key] = value
	}

	return result
}

func (*htmlAttrParserWriter) getSortedKeys(attr *Attributes) []string {
	keys := make([]string, 0, len(attr.Items))

	for k := range attr.Items {
		keys = append(keys, k)
	}

	if len(keys) == 0 {
		return nil
	}

	// Sort attributes by key, however, keep the element
	// with the key "name" in front.
	slices.SortFunc(keys, func(a, b string) int {
		if a == "name" {
			return -1
		}
		if b == "name" {
			return 1
		}
		return strings.Compare(a, b)
	})

	return keys
}

type attrFormat string

// multiParserWriter parses attributes using the provided parsers
// in the order they are provided. If a parser fails, the next one
// is used. It will retain the attribute format of the first successful
// parser for writing.
type multiParserWriter struct {
	items []struct {
		format       attrFormat
		parserWriter attrParserWriter
	}
}

var _ attrParserWriter = (*multiParserWriter)(nil)

func (p *multiParserWriter) getByFormat(format attrFormat) (attrParserWriter, bool) {
	for _, item := range p.items {
		if item.format == format {
			return item.parserWriter, true
		}
	}
	return nil, false
}

func (p *multiParserWriter) Parse(raw []byte) (_ *Attributes, finalErr error) {
	for _, item := range p.items {
		attr, err := item.parserWriter.Parse(raw)
		if err == nil {
			return attr, nil
		}
		finalErr = multierr.Append(finalErr, err)
	}
	return
}

func (p *multiParserWriter) Write(w io.Writer, attr *Attributes) error {
	parserWriter, ok := p.getByFormat(attrFormat(attr.Format))
	if !ok {
		return errors.Errorf("writer for format %q not found", attr.Format)
	}
	return parserWriter.Write(w, attr)
}

// extractAttributes extracts attributes from the source
// by finding the first `{` and last `}` characters.
func extractAttributes(source []byte) []byte {
	start, stop := -1, -1

	for i := 0; i < len(source); i++ {
		if start == -1 && source[i] == '{' && i+1 < len(source) && source[i+1] != '}' {
			start = i + 1
		}
		if stop == -1 && source[i] == '}' {
			stop = i
			break
		}
	}

	if start >= 0 && stop >= 0 {
		return bytes.TrimSpace(source[start-1 : stop+1])
	}

	return nil
}
