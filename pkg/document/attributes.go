package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"
	"go.uber.org/multierr"
)

type Attributes map[string]string

type attributeParser interface {
	Parse(raw []byte) (Attributes, error)
	Write(attr Attributes, w io.Writer) error
}

func getRawAttributes(source []byte) []byte {
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

func getAttributes(node *ast.FencedCodeBlock, source []byte, parser attributeParser) (Attributes, error) {
	attributes := make(map[string]string)

	if node.Info != nil {
		codeBlockInfo := node.Info.Text(source)
		rawAttrs := getRawAttributes(codeBlockInfo)

		if len(bytes.TrimSpace(rawAttrs)) > 0 {
			attr, err := parser.Parse(rawAttrs)
			if err != nil {
				return nil, err
			}

			attributes = attr
		}
	}
	return attributes, nil
}

// Original attribute language used by runme prior to v1.3.0
//
// Only supports strings, and does not support spaces. Example:
//
// { key=value hello=world string_value=2 }
//
// Pioneered by Adam Babik
type babikMLParser struct{}

func (p *babikMLParser) Parse(raw []byte) (Attributes, error) {
	return p.parseRawAttributes(p.rawAttributes(raw)), nil
}

func (*babikMLParser) parseRawAttributes(source []byte) map[string]string {
	items := bytes.Split(source, []byte{' '})
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

func (*babikMLParser) rawAttributes(source []byte) []byte {
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
		return bytes.TrimSpace(source[start:stop])
	}

	return nil
}

func (*babikMLParser) sortedAttrs(attr Attributes) []string {
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

func (p *babikMLParser) Write(attr Attributes, w io.Writer) error {
	keys := p.sortedAttrs(attr)

	_, _ = w.Write([]byte{'{', ' '})
	i := 0
	for _, k := range keys {
		v := attr[k]
		_, _ = w.Write([]byte(fmt.Sprintf("%s=%v", k, v)))
		i++
		if i < len(keys) {
			_, _ = w.Write([]byte{' '})
		}
	}
	_, _ = w.Write([]byte{' ', '}'})

	return nil
}

// JSON parser
//
// Example:
//
// { key: "value", hello: "world", string_value: "2" }
type jsonParser struct{}

func (p *jsonParser) Parse(raw []byte) (Attributes, error) {
	bytes := raw

	parsedAttr := make(map[string]interface{})

	if err := json.Unmarshal(bytes, &parsedAttr); err != nil {
		return nil, err
	}

	attr := make(Attributes, len(parsedAttr))

	for k, v := range parsedAttr {
		if strVal, ok := v.(string); ok {
			attr[k] = strVal
		} else {
			if stringified, err := json.Marshal(v); err == nil {
				attr[k] = string(stringified)
			}
		}
	}

	return attr, nil
}

func (p *jsonParser) Write(attr Attributes, w io.Writer) error {
	// TODO: name at front...
	res, err := json.Marshal(attr)
	if err != nil {
		return err
	}

	res = bytes.TrimSpace(res)

	_, _ = w.Write(res)

	return nil
}

// failoverAttributeParser tries to parse attributes using one of the provided ordered parsers
// until it finds a non-failing one.
// Attributes are written using the provided writer.
type failoverAttributeParser struct {
	parsers []attributeParser
	writer  attributeParser
}

func newFailoverAttributeParser(parsers []attributeParser, writer attributeParser) *failoverAttributeParser {
	return &failoverAttributeParser{
		parsers,
		writer,
	}
}

func (p *failoverAttributeParser) Parse(raw []byte) (attr Attributes, finalErr error) {
	for _, parser := range p.parsers {
		attr, err := parser.Parse(raw)

		if err == nil {
			return attr, nil
		}

		finalErr = multierr.Append(finalErr, err)
	}

	return
}

func (p *failoverAttributeParser) Write(attr Attributes, w io.Writer) error {
	return p.writer.Write(attr, w)
}
