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

var _defaultAttributeParserWriter = &multiParserWriter{
	parserWriters: []parserWriterEntry{
		{format: "json", writer: &jsonParserWriter{}},
		{format: "html", writer: &htmlAttributesParserWriter{}},
	},
}

// interface to represent a store of attribute key-value pairs.
type AttributeStore interface {
	Items() map[string]string
	Format() string
	Clone() AttributeStore
	Equal(other AttributeStore) bool
}

// attributes impl AttributeStore containing a set of key-value pairs applicable to [Cell]s.
// More: https://docs.runme.dev/configuration/cell-level
type attributes struct {
	format string
	items  map[string]string
}

func (a *attributes) Clone() AttributeStore {
	clone := &attributes{
		format: a.format,
		items:  make(map[string]string, len(a.items)),
	}
	for k, v := range a.items {
		clone.items[k] = v
	}
	return clone
}

func (a *attributes) Equal(other AttributeStore) bool {
	otherAttributes, ok := other.(*attributes)
	if !ok {
		return false
	}

	if a.format != otherAttributes.format {
		return false
	}

	if len(a.items) != len(otherAttributes.items) {
		return false
	}

	for k, v := range a.items {
		if otherValue, exists := otherAttributes.items[k]; !exists || v != otherValue {
			return false
		}
	}

	return true
}

func (a *attributes) Items() map[string]string {
	return a.items
}

func (a *attributes) Format() string {
	return a.format
}

func NewAttributes(items map[string]string) (AttributeStore, error) {
	const defaultWriterFormat = "json"
	return _defaultAttributeParserWriter.NewAttributesWithFormat(items, defaultWriterFormat)
}

func NewAttributesWithFormat(items map[string]string, format string) (AttributeStore, error) {
	return _defaultAttributeParserWriter.NewAttributesWithFormat(items, format)
}

// WriteAttributes writes [AttributeStore] to [io.Writer].
func WriteAttributes(w io.Writer, attr AttributeStore) error {
	return _defaultAttributeParserWriter.Write(w, attr)
}

// parseAttributes parses [AttributeStore] from raw bytes.
func parseAttributes(raw []byte) (AttributeStore, error) {
	return _defaultAttributeParserWriter.Parse(raw)
}

type attributesParserWriter interface {
	Parse([]byte) (AttributeStore, error)
	Write(io.Writer, AttributeStore) error
}

func newAttributesFromFencedCodeBlock(
	node *ast.FencedCodeBlock,
	source []byte,
) (AttributeStore, error) {
	attributes, err := NewAttributes(nil)
	if err != nil {
		return nil, err
	}
	if node.Info == nil {
		return attributes, nil
	}

	content := node.Info.Value(source)

	rawAttributes := extractAttributes(content)
	if len(rawAttributes) > 0 {
		var err error
		attributes, err = parseAttributes(rawAttributes)
		if err != nil {
			return nil, err
		}
	}

	return attributes, nil
}

// jsonParserWriter parses all values as strings.
//
// The correct format is as follows:
//
//	{ key: "value", hello: "world", string_value: "2" }
type jsonParserWriter struct{}

func (p *jsonParserWriter) Parse(raw []byte) (AttributeStore, error) {
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

	return NewAttributesWithFormat(attrMap, "json")
}

func (p *jsonParserWriter) Write(w io.Writer, attr AttributeStore) error {
	// TODO: name at front...
	res, err := json.Marshal(attr.Items())
	if err != nil {
		return errors.WithStack(err)
	}
	_, err = w.Write(bytes.TrimSpace(res))
	return errors.WithStack(err)
}

// htmlAttributesParserWriter parses and writes options as HTML attributes.
//
// For example:
//
//	{ key=value hello=world string_value=2 }
//
// HTML attributes is for compatability; prefer the JSON parser instead.
// note this parser will never error, it returns an empty map for invalid data
type htmlAttributesParserWriter struct{}

func (p *htmlAttributesParserWriter) Parse(raw []byte) (AttributeStore, error) {
	rawAttributes := extractAttributes(raw)
	attrMap := p.parseRawAttributes(rawAttributes)
	return NewAttributesWithFormat(attrMap, "html")
}

func (p *htmlAttributesParserWriter) Write(w io.Writer, attr AttributeStore) error {
	keys := p.getSortedKeys(attr)

	_, _ = w.Write([]byte{'{'})
	i := 0
	for _, k := range keys {
		if i == 0 {
			_, _ = w.Write([]byte{' '})
		}
		v := attr.Items()[k]
		_, _ = w.Write([]byte(fmt.Sprintf("%s=%s ", k, v)))
		i++
	}
	_, _ = w.Write([]byte{'}'})

	return nil
}

func (*htmlAttributesParserWriter) parseRawAttributes(raw []byte) map[string]string {
	items := bytes.Split(raw, []byte{' '})
	if len(items) == 0 {
		return nil
	}

	result := make(map[string]string)

	for _, item := range items {
		if !bytes.Contains(item, []byte{'='}) {
			continue
		}
		kv := bytes.Split(item, []byte{'='})
		result[string(kv[0])] = string(kv[1])
	}

	return result
}

func (*htmlAttributesParserWriter) getSortedKeys(attr AttributeStore) []string {
	keys := make([]string, 0, len(attr.Items()))

	for k := range attr.Items() {
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

// multiParserWriter parses attributes using the provided parsers
// in the order they are provided. If a parser fails, the next one
// is used. It will retain the attribute format of the first successful
// parser for writing.
type multiParserWriter struct {
	parserWriters []parserWriterEntry
}

// required to unlike map maintain order of keys
type parserWriterEntry struct {
	format string
	writer attributesParserWriter
}

func (p *multiParserWriter) getWriterParser(format string) (attributesParserWriter, error) {
	for _, entry := range p.parserWriters {
		if entry.format == format {
			return entry.writer, nil
		}
	}
	return nil, errors.Errorf("no parserWriter for format: %s", format)
}

func (p *multiParserWriter) NewAttributesWithFormat(items map[string]string, format string) (AttributeStore, error) {
	if items == nil {
		items = make(map[string]string)
	}

	_, err := p.getWriterParser(format)
	if err != nil {
		return nil, err
	}

	return &attributes{
		format: format,
		items:  items,
	}, nil
}

func (p *multiParserWriter) Parse(raw []byte) (_ AttributeStore, finalErr error) {
	for _, entry := range p.parserWriters {
		parser, err := p.getWriterParser(entry.format)
		if err != nil {
			return nil, err
		}
		attr, err := parser.Parse(raw)
		if err == nil {
			return attr, nil
		}
		finalErr = multierr.Append(finalErr, err)
	}
	return
}

func (p *multiParserWriter) Write(w io.Writer, attr AttributeStore) error {
	writer, err := p.getWriterParser(attr.Format())
	if err != nil {
		return err
	}

	return writer.Write(w, attr)
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
