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
	parsers: []attributesParserWriter{
		&jsonParserWriter{},
		&htmlAttributesParserWriter{},
	},
	writer: &jsonParserWriter{},
}

// Attributes represents a set of key-value pairs applicable to [Cell]s.
// More: https://docs.runme.dev/configuration/cell-level
type Attributes map[string]string

// WriteAttributes writes [Attributes] to [io.Writer].
func WriteAttributes(w io.Writer, attr Attributes) error {
	return _defaultAttributeParserWriter.Write(w, attr)
}

// parseAttributes parses [Attributes] from raw bytes.
func parseAttributes(raw []byte) (Attributes, error) {
	return _defaultAttributeParserWriter.Parse(raw)
}

type attributesParserWriter interface {
	Parse([]byte) (Attributes, error)
	Write(io.Writer, Attributes) error
}

func newAttributesFromFencedCodeBlock(
	node *ast.FencedCodeBlock,
	source []byte,
) (Attributes, error) {
	attributes := make(map[string]string)

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

func (p *jsonParserWriter) Parse(raw []byte) (Attributes, error) {
	// Parse first to a generic map.
	parsed := make(map[string]interface{})
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, errors.WithStack(err)
	}

	result := make(Attributes, len(parsed))

	// Convert all values to strings.
	for k, v := range parsed {
		if strVal, ok := v.(string); ok {
			result[k] = strVal
		} else {
			if stringified, err := json.Marshal(v); err == nil {
				result[k] = string(stringified)
			}
		}
	}

	return result, nil
}

func (p *jsonParserWriter) Write(w io.Writer, attr Attributes) error {
	// TODO: name at front...
	res, err := json.Marshal(attr)
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
// Deprecated: Use the JSON parser instead.
type htmlAttributesParserWriter struct{}

func (p *htmlAttributesParserWriter) Parse(raw []byte) (Attributes, error) {
	rawAttributes := extractAttributes(raw)
	return p.parseRawAttributes(rawAttributes), nil
}

func (p *htmlAttributesParserWriter) Write(w io.Writer, attr Attributes) error {
	keys := p.getSortedKeys(attr)

	_, _ = w.Write([]byte{'{'})
	i := 0
	for _, k := range keys {
		if i == 0 {
			_, _ = w.Write([]byte{' '})
		}
		v := attr[k]
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

func (*htmlAttributesParserWriter) getSortedKeys(attr Attributes) []string {
	keys := make([]string, 0, len(attr))

	for k := range attr {
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
// is used.
// Writer is used to write the attributes back.
type multiParserWriter struct {
	parsers []attributesParserWriter
	writer  attributesParserWriter
}

func (p *multiParserWriter) Parse(raw []byte) (_ Attributes, finalErr error) {
	for _, parser := range p.parsers {
		attr, err := parser.Parse(raw)
		if err == nil {
			return attr, nil
		}
		finalErr = multierr.Append(finalErr, err)
	}
	return
}

func (p *multiParserWriter) Write(w io.Writer, attr Attributes) error {
	return p.writer.Write(w, attr)
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
