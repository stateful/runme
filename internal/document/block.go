package document

import (
	"bytes"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"

	"github.com/stateful/runme/v3/internal/executable"
	"github.com/stateful/runme/v3/internal/shell"
)

type BlockKind int

const (
	InnerBlockKind BlockKind = iota + 1
	CodeBlockKind
	MarkdownBlockKind
)

type Block interface {
	Kind() BlockKind
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

type Renderer func(ast.Node, []byte) ([]byte, error)

type CodeBlock struct {
	id            string
	idGenerated   bool
	attributes    map[string]string
	document      *Document
	inner         *ast.FencedCodeBlock
	intro         string
	language      string
	lines         []string
	name          string
	nameGenerated bool
	value         []byte
}

func newCodeBlock(
	document *Document,
	node *ast.FencedCodeBlock,
	identityResolver identityResolver,
	nameResolver *nameResolver,
	source []byte,
	render Renderer,
) (*CodeBlock, error) {
	attributes, err := getAttributes(node, source, DefaultDocumentParser)
	if err != nil {
		return nil, err
	}

	id, hasID := identityResolver.GetCellID(node, attributes)

	name, hasName := getName(node, source, nameResolver, attributes)

	value, err := render(node, source)
	if err != nil {
		return nil, err
	}

	return &CodeBlock{
		id:            id,
		idGenerated:   !hasID,
		attributes:    attributes,
		document:      document,
		inner:         node,
		intro:         getIntro(node, source),
		language:      getLanguage(node, source),
		lines:         getLines(node, source),
		name:          name,
		nameGenerated: !hasName,
		value:         value,
	}, nil
}

func (b *CodeBlock) Clone() *CodeBlock {
	attributes := make(map[string]string, len(b.attributes))
	for key, value := range b.attributes {
		attributes[key] = value
	}

	lines := make([]string, len(b.lines))
	copy(lines, b.lines)

	value := make([]byte, len(b.value))
	copy(value, b.value)

	return &CodeBlock{
		id:            b.id,
		idGenerated:   b.idGenerated,
		attributes:    attributes,
		intro:         b.intro,
		language:      b.language,
		lines:         lines,
		name:          b.name,
		nameGenerated: b.nameGenerated,
		value:         value,
	}
}

func (b *CodeBlock) MarshalJSON() ([]byte, error) {
	s := struct {
		Name         string `json:"name"`
		FirstCommand string `json:"first_command"`
		Description  string `json:"description"`
	}{
		Name:         b.Name(),
		FirstCommand: b.Lines()[0],
		Description:  b.Intro(),
	}

	return json.Marshal(s)
}

func (b *CodeBlock) Attributes() map[string]string { return b.attributes }

func (b *CodeBlock) Document() *Document { return b.document }

func (b *CodeBlock) Interactive() bool {
	val, err := strconv.ParseBool(b.Attributes()["interactive"])
	if err != nil {
		return true
	}
	return val
}

func (b *CodeBlock) Background() bool {
	val, _ := strconv.ParseBool(b.Attributes()["background"])
	return val
}

func (CodeBlock) Kind() BlockKind { return CodeBlockKind }

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

func (b *CodeBlock) ID() string {
	return b.id
}

func (b *CodeBlock) Name() string {
	return b.name
}

func (b *CodeBlock) IsUnnamed() bool {
	return b.nameGenerated
}

func (b *CodeBlock) IsUnknown() bool {
	return b.Language() == "" || !executable.IsSupported(b.Language())
}

func (b *CodeBlock) Unwrap() ast.Node {
	return b.inner
}

func (b *CodeBlock) Value() []byte {
	return b.value
}

func (b *CodeBlock) SetLine(p int, v string) {
	b.lines[p] = v
}

func (b *CodeBlock) Categories() []string {
	cat, ok := b.Attributes()["category"]
	if !ok {
		return nil
	}
	return strings.Split(cat, ",")
}

func (b *CodeBlock) Cwd() string {
	return b.Attributes()["cwd"]
}

func (b *CodeBlock) Interpreter() string {
	return b.Attributes()["interpreter"]
}

func (b *CodeBlock) PromptEnv() bool {
	val, ok := b.Attributes()["promptEnv"]
	if !ok {
		return true
	}

	// todo(sebastian): integration with ResolveProgram
	switch strings.ToLower(val) {
	case "false", "no", "0":
		return false
	default:
		// use default return
	}

	return true
}

func (b *CodeBlock) ExcludeFromRunAll() bool {
	val, err := strconv.ParseBool(b.Attributes()["excludeFromRunAll"])
	if err != nil {
		return false
	}

	return val
}

type TextRange struct {
	Start int
	End   int
}

func (b *CodeBlock) TextRange() (textRange TextRange) {
	node := b.inner

	textRange.Start = math.MaxInt

	// merge line ranges
	for i := 0; i < node.Lines().Len(); i++ {
		line := node.Lines().At(i)
		textRange.Start = min(textRange.Start, line.Start)
		textRange.End = max(textRange.Start, line.Stop)
	}

	return
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
		return bytes.TrimSpace(source[start-1 : stop+1])
	}

	return nil
}

func getAttributes(node *ast.FencedCodeBlock, source []byte, parser attributeParser) (Attributes, error) {
	attributes := make(map[string]string)

	if node.Info != nil {
		codeBlockInfo := node.Info.Text(source)
		rawAttrs := rawAttributes(codeBlockInfo)

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

// TODO(mxs): use guesslang model
func getLanguage(node *ast.FencedCodeBlock, source []byte) string {
	var rawAttrs string
	if node.Info != nil {
		codeBlockInfo := node.Info.Text(source)
		rawAttrs = string(rawAttributes(codeBlockInfo))
	}

	// If the language is the same as the raw attributes,
	// it means Goldmark is not aware of our internal language segment usage.
	if lang := string(node.Language(source)); lang != "" && !strings.HasPrefix(rawAttrs, lang) {
		return lang
	}
	return ""
}

var replaceEndingRe = regexp.MustCompile(`([^a-z0-9!?\.])$`)

func normalizeIntro(s string) string {
	return replaceEndingRe.ReplaceAllString(s, ".")
}

func getIntro(node *ast.FencedCodeBlock, source []byte) string {
	if prevNode := node.PreviousSibling(); prevNode != nil && prevNode.Kind() == ast.KindParagraph {
		return normalizeIntro(string(prevNode.Text(source)))
	}
	return ""
}

func normalizeLine(s string) string {
	return strings.TrimRightFunc(strings.TrimLeft(s, "$"), unicode.IsSpace)
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

func getName(node *ast.FencedCodeBlock, source []byte, nameResolver *nameResolver, attributes Attributes) (string, bool) {
	hasName := false

	var name string
	if n, ok := attributes["name"]; ok && n != "" {
		name = n
		hasName = true
	} else {
		lines := getLines(node, source)
		if len(lines) > 0 {
			// TODO(mxs): only do this in sh-like commands
			name = sanitizeName(shell.TryGetNonCommentLine(lines))
		}
	}
	return nameResolver.Get(node, name), hasName
}

type MarkdownBlock struct {
	inner ast.Node
	value []byte
}

func newMarkdownBlock(
	node ast.Node,
	source []byte,
	render Renderer,
) (*MarkdownBlock, error) {
	value, err := render(node, source)
	if err != nil {
		return nil, err
	}
	return &MarkdownBlock{
		inner: node,
		value: value,
	}, nil
}

func (MarkdownBlock) Kind() BlockKind { return MarkdownBlockKind }

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
	inner ast.Node
	value []byte
}

func newInnerBlock(
	node ast.Node,
	source []byte,
	render Renderer,
) (*InnerBlock, error) {
	value, err := render(node, source)
	if err != nil {
		return nil, err
	}
	return &InnerBlock{
		inner: node,
		value: value,
	}, nil
}

func (InnerBlock) Kind() BlockKind { return InnerBlockKind }

func (b *InnerBlock) Unwrap() ast.Node {
	return b.inner
}

func (b *InnerBlock) Value() []byte {
	return b.value
}

func (b *CodeBlock) GetBlock() *CodeBlock {
	return b
}

func (b *CodeBlock) GetFileRel() string {
	return ""
}

func (b *CodeBlock) GetFile() string {
	return ""
}

func (b *CodeBlock) GetFrontmatter() Frontmatter {
	return Frontmatter{}
}
