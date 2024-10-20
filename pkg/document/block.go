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

func (b CodeBlocks) Names() (result []string) {
	for _, block := range b {
		result = append(result, block.Name())
	}
	return result
}

type renderer func(ast.Node, []byte) ([]byte, error)

type CodeBlock struct {
	id            string
	idGenerated   bool
	attributes    map[string]string
	document      *Document
	inner         *ast.FencedCodeBlock
	intro         string // paragraph immediately before the code block
	language      string
	lines         []string // actual code lines
	name          string
	nameGenerated bool
	value         []byte // markdown source
}

var _ Block = (*CodeBlock)(nil)

func newCodeBlock(
	document *Document,
	node *ast.FencedCodeBlock,
	identityResolver identityResolver,
	nameResolver *nameResolver,
	source []byte,
	render renderer,
) (*CodeBlock, error) {
	attributes, err := getAttributes(node, source, DefaultAttributeParser)
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

func (b *CodeBlock) Attributes() map[string]string { return b.attributes }

func (b *CodeBlock) Background() bool {
	val, _ := strconv.ParseBool(b.Attributes()["background"])
	return val
}

func (b *CodeBlock) Tags() []string {
	var superset []string

	categories, ok := b.Attributes()["category"]
	if ok {
		superset = append(superset, strings.Split(categories, ",")...)
	}

	tags, ok := b.Attributes()["tag"]
	if ok {
		superset = append(superset, strings.Split(tags, ",")...)
	}

	return superset
}

func (b *CodeBlock) Clone() *CodeBlock {
	clone := *b

	attributes := make(map[string]string, len(b.attributes))
	for key, value := range b.attributes {
		attributes[key] = value
	}
	clone.attributes = attributes

	lines := make([]string, len(b.lines))
	copy(lines, b.lines)
	clone.lines = lines

	value := make([]byte, len(b.value))
	copy(value, b.value)
	clone.value = value

	return &clone
}

func (b *CodeBlock) Content() []byte {
	value := bytes.Trim(b.value, "\n")
	lines := bytes.Split(value, []byte{'\n'})
	if len(lines) < 2 {
		return b.value
	}
	return bytes.Join(lines[1:len(lines)-1], []byte{'\n'})
}

func (b *CodeBlock) Cwd() string {
	return b.Attributes()["cwd"]
}

func (b *CodeBlock) Document() *Document { return b.document }

func (b *CodeBlock) ExcludeFromRunAll() bool {
	val, err := strconv.ParseBool(b.Attributes()["excludeFromRunAll"])
	if err != nil {
		return false
	}
	return val
}

func (b *CodeBlock) FirstLine() string {
	if len(b.lines) > 0 {
		return b.lines[0]
	}
	return ""
}

func (b *CodeBlock) ID() string { return b.id }

func (b *CodeBlock) Interactive() bool {
	val, _ := strconv.ParseBool(b.Attributes()["interactive"])
	return val
}

// InteractiveLegacy returns true as a default value.
// Deprecated: use Interactive instead, however, keep using
// if you want to align with the VS Code extension.
func (b *CodeBlock) InteractiveLegacy() bool {
	val, err := strconv.ParseBool(b.Attributes()["interactive"])
	if err != nil {
		return true
	}
	return val
}

func (b *CodeBlock) Interpreter() string {
	return b.Attributes()["interpreter"]
}

func (b *CodeBlock) Intro() string { return b.intro }

func (b *CodeBlock) IsUnknown() bool {
	return b.Language() == "" || !executable.IsSupported(b.Language())
}

func (b *CodeBlock) IsUnnamed() bool { return b.nameGenerated }

func (CodeBlock) Kind() BlockKind { return CodeBlockKind }

func (b *CodeBlock) Language() string { return b.language }

func (b *CodeBlock) Lines() []string { return b.lines }

func (b *CodeBlock) Name() string { return b.name }

func (b *CodeBlock) MarshalJSON() ([]byte, error) {
	s := struct {
		Description  string `json:"description"`
		FirstCommand string `json:"first_command"`
		Name         string `json:"name"`
	}{
		Description:  b.Intro(),
		FirstCommand: b.FirstLine(),
		Name:         b.Name(),
	}
	return json.Marshal(s)
}

func (b *CodeBlock) PromptEnvStr() string {
	return b.Attributes()["promptEnv"]
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
		return true
	}
}

func (b *CodeBlock) SetLine(p int, v string) {
	b.lines[p] = v
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

func (b *CodeBlock) Unwrap() ast.Node {
	return b.inner
}

func (b *CodeBlock) Value() []byte {
	return b.value
}

func (b *CodeBlock) SetLines(newLines []string) {
	b.lines = newLines
}

func (b *CodeBlock) PrependLines(newLines []string) {
	b.lines = append(newLines, b.lines...)
}

// TODO(mxs): use guesslang model
func getLanguage(node *ast.FencedCodeBlock, source []byte) string {
	var rawAttrs string
	if node.Info != nil {
		codeBlockInfo := node.Info.Text(source)
		rawAttrs = string(getRawAttributes(codeBlockInfo))
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

var _ Block = (*MarkdownBlock)(nil)

func newMarkdownBlock(
	node ast.Node,
	source []byte,
	render renderer,
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

var _ Block = (*InnerBlock)(nil)

func newInnerBlock(
	node ast.Node,
	source []byte,
	render renderer,
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

func (b *InnerBlock) Unwrap() ast.Node { return b.inner }

func (b *InnerBlock) Value() []byte { return b.value }
