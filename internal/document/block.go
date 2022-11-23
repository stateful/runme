package document

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/stateful/runme/internal/renderer/md"
	"github.com/yuin/goldmark/ast"
)

type NameResolver interface {
	Get(interface{}, string) string
}

type CodeBlock struct {
	inner           *ast.FencedCodeBlock
	isExtra         bool     // true for a block created for exracted blocks
	parent          ast.Node // for extracted blocks
	previousSibling ast.Node // for extracted blocks
	nameResolver    NameResolver

	attributes map[string]string
	intro      string
	language   string
	lines      []string
	name       string
	value      []byte
}

func newCodeBlock(source []byte, nameResolver NameResolver, node *ast.FencedCodeBlock) *CodeBlock {
	name := getName(node, source, nameResolver)

	attributes := make(map[string]string)
	if node.Info != nil {
		attributes = parseRawAttributes(rawAttributes(node.Info.Text(source)))
	}
	attributes["name"] = name

	value, _ := md.Render(node, source)

	return &CodeBlock{
		inner:        node,
		nameResolver: nameResolver,
		attributes:   attributes,
		intro:        getIntro(node, source),
		language:     getLanguage(node, source),
		lines:        getLines(node, source),
		name:         name,
		value:        value,
	}
}

func (b *CodeBlock) Attributes() map[string]string {
	return b.attributes
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
	inner           ast.Node
	isExtra         bool     // true for a block created for exracted blocks
	parent          ast.Node // for extracted blocks
	previousSibling ast.Node // for extracted blocks
	source          []byte

	value []byte
}

func newMarkdownBlock(source []byte, node ast.Node) *MarkdownBlock {
	value, _ := md.Render(node, source)
	return &MarkdownBlock{
		inner:  node,
		source: source,
		value:  value,
	}
}

func (b *MarkdownBlock) Unwrap() ast.Node {
	return b.inner
}

func (b *MarkdownBlock) Value() []byte {
	return b.value
}

type Block interface {
	Unwrap() ast.Node
	Value() []byte
}

type Blocks []Block

func (b Blocks) CodeBlocks() (result CodeBlocks) {
	for _, block := range b {
		if v, ok := block.(*CodeBlock); ok {
			result = append(result, v)
		}
	}
	return
}

type CodeBlocks []*CodeBlock

func (b CodeBlocks) Lookup(name string) *CodeBlock {
	for _, block := range b {
		if block.name == name {
			return block
		}
	}
	return nil
}

func (b CodeBlocks) Names() (result []string) {
	for _, block := range b {
		result = append(result, block.name)
	}
	return result
}
