package document

import (
	"bytes"
	"fmt"
	"io"
	"reflect"

	"go.uber.org/multierr"
	"golang.org/x/exp/slices"

	"github.com/pelletier/go-toml"
)

type Attributes map[string]string

type attributeParser interface {
	ParseAttributes(raw []byte) (Attributes, error)
	WriteAttributes(attr Attributes, w io.Writer) error
}

// Original attribute language used by runme prior to v1.3.0
//
// Only supports strings, and does not support spaces. Example:
//
// { key=value hello=world string_value=2 }
//
// Pioneered by Adam Babik
type babikMLParser struct{}

func (p *babikMLParser) ParseAttributes(raw []byte) (Attributes, error) {
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

func sortedAttrs(attr Attributes) []string {
	keys := make([]string, 0, len(attr))

	for k := range attr {
		keys = append(keys, k)
	}

	// Sort attributes by key, however, keep the element
	// with the key "name" in front.
	slices.SortFunc(keys, func(a, b string) bool {
		if a == "name" {
			return true
		}
		if b == "name" {
			return false
		}
		return a < b
	})

	if len(keys) == 0 {
		return nil
	}

	return keys
}

func (*babikMLParser) WriteAttributes(attr Attributes, w io.Writer) error {
	keys := sortedAttrs(attr)

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

// "Inline" toml parser
//
// Example:
//
// { key = "value", hello = "world", string_value="2" }
type tomlParser struct{}

func (p *tomlParser) ParseAttributes(raw []byte) (Attributes, error) {
	bytes := []byte("attr=")
	bytes = append(bytes, raw...)

	root := make(map[string](map[string]interface{}))

	if err := toml.Unmarshal(bytes, &root); err != nil {
		return nil, err
	}

	parsedAttr := root["attr"]

	attr := make(Attributes, len(parsedAttr))

	for k, v := range parsedAttr {
		kind := reflect.TypeOf(v).Kind()

		// primitive type
		if kind < reflect.Array || kind == reflect.String {
			attr[k] = fmt.Sprintf("%v", v)
		}
	}

	return attr, nil
}

func (p *tomlParser) WriteAttributes(attr Attributes, w io.Writer) error {
	res, err := toml.Marshal(attr)
	if err != nil {
		return err
	}

	res = bytes.TrimSpace(res)

	lines := bytes.Split(res, []byte{'\n'})

	// Sort attributes by key, however, keep the element
	// with the key "name" in front.
	slices.SortFunc(lines, func(a, b []byte) bool {
		if bytes.HasPrefix(a, []byte("name")) {
			return true
		}
		if bytes.HasPrefix(b, []byte("name")) {
			return false
		}
		return string(a) < string(b)
	})

	_, _ = w.Write([]byte{'{', ' '})
	_, _ = w.Write(bytes.Join(
		lines, []byte{',', ' '},
	))
	_, _ = w.Write([]byte{' ', '}'})

	return nil
}

// Tries to parse attributes in sequence, until it finds a non-failing parser
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

func (p *failoverAttributeParser) ParseAttributes(raw []byte) (attr Attributes, finalErr error) {
	for _, parser := range p.parsers {
		attr, err := parser.ParseAttributes(raw)

		if err == nil {
			return attr, nil
		}

		finalErr = multierr.Append(finalErr, err)
	}

	return
}

func (p *failoverAttributeParser) WriteAttributes(attr Attributes, w io.Writer) error {
	return p.writer.WriteAttributes(attr, w)
}

var DefaultDocumentParser = newFailoverAttributeParser(
	[]attributeParser{
		&tomlParser{},
		&babikMLParser{},
	},
	&tomlParser{},
)
