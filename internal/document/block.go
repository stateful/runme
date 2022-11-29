package document

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/rs/xid"
	"github.com/yuin/goldmark/ast"
)

type blockKind int

const (
	innerBlock blockKind = iota + 1
	codeBlock
	markdownBlock
)

type Block interface {
	kind() blockKind
	id() string
	setValue([]byte)

	Unwrap() ast.Node
	Value() []byte
}

type Blocks []Block

type CodeBlocks []*CodeBlock

func (b CodeBlocks) Lookup(name string) *CodeBlock {
	for _, block := range b {
		if block.Name() == name {
			return block
		}
	}
	return nil
}

func (b CodeBlocks) Names() (result []string) {
	for _, block := range b {
		result = append(result, block.Name())
	}
	return result
}

type NameResolver interface {
	Get(interface{}, string) string
}

type Renderer func(ast.Node, []byte) ([]byte, error)

type CodeBlock struct {
	attributes map[string]string
	inner      *ast.FencedCodeBlock
	intro      string
	language   string
	lines      []string
	name       string
	value      []byte
}

func newCodeBlock(
	node *ast.FencedCodeBlock,
	nameResolver NameResolver,
	source []byte,
	render Renderer,
) (*CodeBlock, error) {
	name := getName(node, source, nameResolver)

	attributes := make(map[string]string)
	if node.Info != nil {
		attributes = parseRawAttributes(rawAttributes(node.Info.Text(source)))
	}
	attributes["_blockId"] = getID()
	attributes["name"] = name

	value, err := render(node, source)
	if err != nil {
		return nil, err
	}

	return &CodeBlock{
		attributes: attributes,
		inner:      node,
		intro:      getIntro(node, source),
		language:   getLanguage(node, source),
		lines:      getLines(node, source),
		name:       name,
		value:      value,
	}, nil
}

func (CodeBlock) kind() blockKind { return codeBlock }

func (b *CodeBlock) id() string { return b.attributes["_blockId"] }

func (b *CodeBlock) setValue(value []byte) { b.value = value }

func (b *CodeBlock) Attributes() map[string]string {
	return b.attributes
}

func (b *CodeBlock) Content() []byte {
	value := bytes.Trim(b.value, "\n")
	lines := bytes.Split(value, []byte{'\n'})
	if len(lines) < 2 {
		return b.value
	}
	return bytes.Join(lines[1:len(lines)-1], []byte{'\n'})
}

func (b *CodeBlock) Intro() string {
	return b.intro
}

func (b *CodeBlock) Language() string {
	return b.language
}

func (b *CodeBlock) Lines() []string {
	return b.lines
}

func (b *CodeBlock) Name() string {
	return b.name
}

func (b *CodeBlock) Unwrap() ast.Node {
	return b.inner
}

func (b *CodeBlock) Value() []byte {
	return b.value
}

func rawAttributes(source []byte) []byte {
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

func parseRawAttributes(source []byte) map[string]string {
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

func getLanguage(node *ast.FencedCodeBlock, source []byte) string {
	if lang := string(node.Language(source)); lang != "" {
		return lang
	}
	return ""
}

var replaceEndingRe = regexp.MustCompile(`([^a-z0-9!?\.])$`)

func normalizeIntro(s string) string {
	return replaceEndingRe.ReplaceAllString(s, ".")
}

func getIntro(node *ast.FencedCodeBlock, source []byte) string {
	if prevNode := node.PreviousSibling(); prevNode != nil {
		return normalizeIntro(string(prevNode.Text(source)))
	}
	return ""
}

func normalizeLine(s string) string {
	return strings.TrimSpace(strings.TrimLeft(s, "$"))
}

func getLines(node *ast.FencedCodeBlock, source []byte) []string {
	var result []string
	for i := 0; i < node.Lines().Len(); i++ {
		line := node.Lines().At(i)
		result = append(result, normalizeLine(string(line.Value(source))))
	}
	return result
}

func sanitizeName(s string) string {
	// Handle cases when the first line is defining a variable.
	if idx := strings.Index(s, "="); idx > 0 {
		return sanitizeName(s[:idx])
	}

	limit := len(s)
	if limit > 32 {
		limit = 32
	}
	s = s[0:limit]

	fragments := strings.Split(s, " ")
	if len(fragments) > 1 {
		s = strings.Join(fragments[:2], " ")
	} else {
		s = fragments[0]
	}

	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r == ' ' && b.Len() > 0 {
			_, _ = b.WriteRune('-')
		} else if r >= '0' && r <= '9' || r >= 'a' && r <= 'z' {
			_, _ = b.WriteRune(r)
		}
	}
	return b.String()
}

func getName(node *ast.FencedCodeBlock, source []byte, nameResolver NameResolver) string {
	attributes := make(map[string]string)
	if node.Info != nil {
		attributes = parseRawAttributes(rawAttributes(node.Info.Text(source)))
	}

	var name string
	if n, ok := attributes["name"]; ok && n != "" {
		name = n
	} else {
		lines := getLines(node, source)
		if len(lines) > 0 {
			name = sanitizeName(lines[0])
		}
	}
	return nameResolver.Get(node, name)
}

type MarkdownBlock struct {
	attributes map[string]string
	inner      ast.Node
	value      []byte
}

func newMarkdownBlock(
	node ast.Node,
	source []byte,
	render Renderer,
) (*MarkdownBlock, error) {
	attributes := map[string]string{"_blockId": getID()}
	value, err := render(node, source)
	if err != nil {
		return nil, err
	}
	return &MarkdownBlock{
		attributes: attributes,
		inner:      node,
		value:      value,
	}, nil
}

func (MarkdownBlock) kind() blockKind { return markdownBlock }

func (b *MarkdownBlock) id() string { return b.attributes["_blockId"] }

func (b *MarkdownBlock) setValue(value []byte) { b.value = value }

func (b *MarkdownBlock) Attributes() map[string]string { return b.attributes }

func (b *MarkdownBlock) Unwrap() ast.Node {
	return b.inner
}

func (b *MarkdownBlock) Value() []byte {
	return b.value
}

// InnerBlock represents a non-leaf block.
// It helps to handle nested fenced code blocks
// for block quotes and list items.
type InnerBlock struct {
	attributes map[string]string
	inner      ast.Node
	value      []byte
}

func newInnerBlock(
	node ast.Node,
	source []byte,
	render Renderer,
) (*InnerBlock, error) {
	attributes := map[string]string{"_blockId": getID()}
	value, err := render(node, source)
	if err != nil {
		return nil, err
	}
	return &InnerBlock{
		attributes: attributes,
		inner:      node,
		value:      value,
	}, nil
}

func (InnerBlock) kind() blockKind { return innerBlock }

func (b *InnerBlock) id() string { return b.attributes["_blockId"] }

func (b *InnerBlock) setValue(value []byte) { b.value = value }

func (b *InnerBlock) Unwrap() ast.Node {
	return b.inner
}

func (b *InnerBlock) Value() []byte {
	return b.value
}

func getID() string {
	return xid.New().String()
}
