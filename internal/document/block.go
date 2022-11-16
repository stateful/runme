package document

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
)

type NameResolver interface {
	Get(interface{}, string) string
}

type CodeBlock struct {
	extracted    bool // extracted from another block like a list or block quote
	inner        *ast.FencedCodeBlock
	nameResolver NameResolver
	source       []byte

	attributes map[string]string
	content    string
	intro      string
	lines      []string
	start      int
	stop       int
}

func newCodeBlock(source []byte, nameResolver NameResolver, node *ast.FencedCodeBlock) *CodeBlock {
	return &CodeBlock{
		inner:        node,
		nameResolver: nameResolver,
		source:       source,

		start: -1,
		stop:  -1,
	}
}

func (b *CodeBlock) rawAttributes() []byte {
	if b.inner.Info == nil {
		return nil
	}

	segment := b.inner.Info.Segment
	info := segment.Value(b.source)
	start, stop := -1, -1

	for i := 0; i < len(info); i++ {
		if start == -1 && info[i] == '{' && i+1 < len(info) && info[i+1] != '}' {
			start = i + 1
		}
		if stop == -1 && info[i] == '}' {
			stop = i
			break
		}
	}

	if start >= 0 && stop >= 0 {
		return bytes.TrimSpace(info[start:stop])
	}

	return nil
}

func (b *CodeBlock) parseAttributes(raw []byte) map[string]string {
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

// Attributes returns code block attributes detected in the first line.
// They are of a form: "sh { attr=value }".
func (b *CodeBlock) Attributes() map[string]string {
	if b.attributes == nil {
		b.attributes = b.parseAttributes(b.rawAttributes())
	}
	return b.attributes
}

// Content returns unaltered snippet as a single blob of text.
func (b *CodeBlock) Content() string {
	if b.content == "" {
		start, stop := getContentRange(b.source, b.inner)

		var buf strings.Builder

		_, _ = buf.Write(b.source[start:b.inner.Lines().At(0).Start])

		for i := 0; i < b.inner.Lines().Len(); i++ {
			line := b.inner.Lines().At(i)
			_, _ = buf.Write(line.Value(b.source))
		}

		_, _ = buf.Write(b.source[b.inner.Lines().At(b.inner.Lines().Len()-1).Stop:stop])

		b.content = buf.String()
		b.start = start
		b.stop = stop
	}
	return b.content
}

func (b *CodeBlock) SetExtracted(val bool) {
	b.extracted = val
}

func (b *CodeBlock) Start() int {
	if b.start == -1 {
		_ = b.Content()
	}
	return b.start
}

func (b *CodeBlock) Stop() int {
	if b.stop == -1 {
		_ = b.Content()
	}
	return b.stop
}

// Executable returns an identifier of a program to execute the block.
func (b *CodeBlock) Executable() string {
	if lang := string(b.inner.Language(b.source)); lang != "" {
		return lang
	}
	return ""
}

var replaceEndingRe = regexp.MustCompile(`([^a-z0-9!?\.])$`)

func normalizeIntro(s string) string {
	return replaceEndingRe.ReplaceAllString(s, ".")
}

// Intro returns a normalized description of the code block
// based on the preceding paragraph.
func (b *CodeBlock) Intro() string {
	if b.intro == "" {
		if prevNode := b.inner.PreviousSibling(); prevNode != nil {
			b.intro = normalizeIntro(string(prevNode.Text(b.source)))
		}
	}
	return b.intro
}

// Line returns a normalized code block line at index.
func (b *CodeBlock) Line(idx int) string {
	lines := b.getLines()
	if idx >= len(lines) {
		return ""
	}
	return lines[idx]
}

// LineCount returns the number of code block lines.
func (b *CodeBlock) LineCount() int {
	return len(b.getLines())
}

func normalizeLine(s string) string {
	return strings.TrimSpace(strings.TrimLeft(s, "$"))
}

func (b *CodeBlock) getLines() []string {
	if b.lines == nil {
		var result []string
		for i := 0; i < b.inner.Lines().Len(); i++ {
			line := b.inner.Lines().At(i)
			result = append(result, normalizeLine(string(line.Value(b.source))))
		}
		b.lines = result
	}
	return b.lines
}

// Lines returns all code block lines, normalized.
func (b *CodeBlock) Lines() (result []string) {
	return b.getLines()
}

func (b *CodeBlock) MapLines(fn func(string) (string, error)) error {
	var result []string
	for _, line := range b.getLines() {
		v, err := fn(line)
		if err != nil {
			return err
		}
		result = append(result, v)
	}
	b.lines = result
	return nil
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

// Name returns a code block name.
func (b *CodeBlock) Name() string {
	var name string
	if n, ok := b.Attributes()["name"]; ok && n != "" {
		name = n
	} else {
		name = sanitizeName(b.Line(0))
	}
	return b.nameResolver.Get(b, name)
}

func (b *CodeBlock) MarshalJSON() ([]byte, error) {
	type codeBlock struct {
		Attributes map[string]string `json:"attributes"`
		Name       string            `json:"name"`
		Editable   bool              `json:"editable"`
		Executable string            `json:"executable"`
		Lines      []string          `json:"lines"`
		Source     string            `json:"source"`
		Type       string            `json:"type"`
	}

	block := codeBlock{
		Attributes: b.Attributes(),
		Name:       b.Name(),
		Editable:   !b.extracted,
		Executable: b.Executable(),
		Lines:      b.Lines(),
		Source:     b.Content(),
		Type:       "code",
	}

	return json.Marshal(block)
}

type MarkdownBlock struct {
	extracted bool // extracted from another block like a list or block quote
	inner     ast.Node
	source    []byte

	content string
	start   int
	stop    int
}

func newMarkdownBlock(source []byte, node ast.Node) *MarkdownBlock {
	return &MarkdownBlock{
		inner:  node,
		source: source,

		start: -1,
		stop:  -1,
	}
}

func findFirstBlock(node ast.Node) ast.Node {
	if !node.HasChildren() || node.FirstChild().Type() != ast.TypeBlock {
		return node
	}
	node = node.FirstChild()
	for node.FirstChild() != nil && node.FirstChild().Type() == ast.TypeBlock {
		node = node.FirstChild()
	}
	return node
}

func findLastBlock(node ast.Node) ast.Node {
	if !node.HasChildren() || node.LastChild().Type() != ast.TypeBlock {
		return node
	}
	node = node.LastChild()
	for node.LastChild() != nil && node.LastChild().Type() == ast.TypeBlock {
		node = node.LastChild()
	}
	return node
}

func peekLine(b []byte) []byte {
	idx := bytes.IndexByte(b, '\n')
	if idx == -1 {
		return b
	}
	return b[:idx]
}

func (b *MarkdownBlock) Content() string {
	if b.content == "" {
		switch b.inner.Kind() {
		case ast.KindHeading:
			start := b.inner.Lines().At(0).Start
			if start > 0 {
				if idx := bytes.LastIndexByte(b.source[0:start], '\n'); idx != -1 {
					start = idx + 1
				} else {
					start = 0
				}
			}

			stop := b.inner.Lines().At(b.inner.Lines().Len() - 1).Stop
			if idx := bytes.IndexByte(b.source[stop:], '\n'); idx != -1 {
				stop = stop + idx
			}

			if stop+1 < len(b.source) {
				if line := peekLine(b.source[stop+1:]); bytes.ContainsAny(line, "=-") {
					stop += len(line) + 1
				}
			}

			b.content = string(b.source[start:stop])
			b.start = start
			b.stop = stop
		case ast.KindList, ast.KindBlockquote, ast.KindParagraph:
			firstChild := findFirstBlock(b.inner)
			lastChild := findLastBlock(b.inner)

			start := firstChild.Lines().At(0).Start
			if start > 0 {
				if idx := bytes.LastIndexByte(b.source[0:start], '\n'); idx != -1 {
					start = idx + 1
				} else {
					start = 0
				}
			}

			stop := lastChild.Lines().At(lastChild.Lines().Len() - 1).Stop
			if idx := bytes.IndexByte(b.source[stop:], '\n'); idx != -1 {
				stop = stop + idx
			}

			b.content = string(b.source[start:stop])
			b.start = start
			b.stop = stop
		case ast.KindThematicBreak:
			previousBlock := b.inner.PreviousSibling()
			for previousBlock != nil && !hasLine(previousBlock) {
				previousBlock.PreviousSibling()
				panic(1)
			}

			start := 0
			if previousBlock != nil {
				_, start = getContentRange(b.source, previousBlock)
				start++
			}
			for ch := b.source[start]; ch == '\n'; ch = b.source[start] {
				start++
			}

			nextBlock := b.inner.NextSibling()
			for nextBlock != nil && !hasLine(nextBlock) {
				nextBlock = nextBlock.NextSibling()
			}

			stop := len(b.source) - 1
			if nextBlock != nil {
				stop, _ = getContentRange(b.source, nextBlock)
				stop--
			}
			for ch := b.source[stop]; ch == '\n'; ch = b.source[stop] {
				stop--
			}

			b.content = string(b.source[start : stop+1])
			b.start = start
			b.stop = stop
		default:
			var buf strings.Builder
			for i := 0; i < b.inner.Lines().Len(); i++ {
				line := b.inner.Lines().At(i)
				value := line.Value(b.source)
				_, _ = buf.Write(value)
			}
			b.content = buf.String()
			b.start = b.inner.Lines().At(0).Start
			b.stop = b.inner.Lines().At(b.inner.Lines().Len() - 1).Stop
		}
	}
	return b.content
}

func (b *MarkdownBlock) SetExtracted(val bool) {
	b.extracted = val
}

func (b *MarkdownBlock) Start() int {
	if b.start == -1 {
		_ = b.Content()
	}
	return b.start
}

func (b *MarkdownBlock) Stop() int {
	if b.stop == -1 {
		_ = b.Content()
	}
	return b.stop
}

func (b *MarkdownBlock) MarshalJSON() ([]byte, error) {
	type markdownBlock struct {
		Editable bool   `json:"editable"`
		Source   string `json:"source"`
		Type     string `json:"type"`
	}

	block := markdownBlock{
		Editable: !b.extracted,
		Source:   b.Content(),
		Type:     "markdown",
	}

	return json.Marshal(block)
}

type Block interface {
	json.Marshaler
	Content() string
	Start() int
	Stop() int
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

func getContentRange(source []byte, node ast.Node) (int, int) {
	switch node.Kind() {
	case ast.KindFencedCodeBlock:
		start := node.Lines().At(0).Start

		// Find first back tick.
		for start > 0 {
			ch := source[start]
			if ch == '`' {
				break
			}
			start--
		}
		// Go back until the end of back ticks.
		for start > 0 {
			ch := source[start]
			if ch != '`' {
				start++
				break
			}
			start--
		}

		stop := node.Lines().At(node.Lines().Len() - 1).Stop

		// The source has no end line.
		if stop == len(source) {
			stop--
		} else {
			for stop < len(source) {
				ch := source[stop]
				if ch == '\n' {
					break
				}
				stop++
			}
		}

		return start, stop
	default:
		return node.Lines().At(0).Start, node.Lines().At(node.Lines().Len() - 1).Stop
	}
}
