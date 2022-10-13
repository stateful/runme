package document

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/pkg/errors"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type Source struct {
	data []byte
}

func NewSource(data []byte) *Source {
	return &Source{data: data}
}

func NewSourceFromFile(f fs.FS, filename string) (*Source, error) {
	data, err := fs.ReadFile(f, filename)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return NewSource(data), nil
}

func (s *Source) Parse() *ParsedSource {
	return newDefaultParser().Parse(s.data)
}

type ParsedSource struct {
	data []byte
	root ast.Node
}

func (s *ParsedSource) Root() ast.Node {
	return s.root
}

func (s *ParsedSource) Source() []byte {
	return s.data
}

func (s *ParsedSource) Blocks() Blocks {
	var result Blocks

	nameRes := &nameResolver{
		namesCounter: map[string]int{},
		cache:        map[interface{}]string{},
	}

	for c := s.root.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindFencedCodeBlock:
			result = append(result, &CodeBlock{
				source:       s.data,
				inner:        c.(*ast.FencedCodeBlock),
				nameResolver: nameRes,
			})
		default:
			result = append(result, &MarkdownBlock{
				source: s.data,
				inner:  c,
			})
		}
	}

	return result
}

func (s *ParsedSource) CodeBlocks() CodeBlocks {
	var result CodeBlocks

	nameRes := &nameResolver{
		namesCounter: map[string]int{},
		cache:        map[interface{}]string{},
	}

	for c := s.root.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Kind() == ast.KindFencedCodeBlock {
			result = append(result, &CodeBlock{
				source:       s.data,
				inner:        c.(*ast.FencedCodeBlock),
				nameResolver: nameRes,
			})
		}
	}

	return result
}

func getRange(source []byte, n ast.Node, start int, stop int) (string, error) {
	var content strings.Builder
	switch n.Kind() {
	case ast.KindHeading:
		heading := n.(*ast.Heading)
		offset := 1 + heading.Level
		// shield from inital ======= vs ### heading
		if start-offset < 0 {
			offset = 0
		}
		_, _ = content.Write(source[start-offset : stop])
	default:
		_, _ = content.Write(source[start:stop])
	}
	return content.String(), nil
}

func getPrevStart(n ast.Node) int {
	curr := n
	prev := n.PreviousSibling()
	if prev != nil && prev.Lines().Len() > 0 {
		curr = prev
	}
	return curr.Lines().At(0).Stop
}

func getNextStop(n ast.Node) int {
	curr := n
	next := n.NextSibling()
	if next != nil {
		curr = next
	}

	l := curr.Lines().Len()
	if l == 0 {
		return 0
	}

	stop := curr.Lines().At(l - 1).Start

	// add back markdown heading levels
	if curr.Kind() == ast.KindHeading {
		heading := curr.(*ast.Heading)
		// simple math to add back ## incl trailing space
		stop = stop - 1 - heading.Level
	}

	return stop
}

func (s *ParsedSource) SquashedBlocks() (blocks Blocks, err error) {
	nameRes := &nameResolver{
		namesCounter: map[string]int{},
		cache:        map[interface{}]string{},
	}

	lastCodeBlock := s.root
	remainingNode := s.root

	err = ast.Walk(s.root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if n.Kind() != ast.KindFencedCodeBlock || !entering {
			return ast.WalkContinue, nil
		}

		start := getPrevStart(n)

		if lastCodeBlock != nil {
			stop := getNextStop(lastCodeBlock)
			// check for existence of markdown in between code blocks
			if start > stop {
				markdown, _ := getRange(s.data, n, stop, start)
				blocks = append(blocks, &MarkdownBlock{content: markdown})
			}
			lastCodeBlock = n
		}

		blocks = append(blocks, &CodeBlock{
			source:       s.data,
			inner:        n.(*ast.FencedCodeBlock),
			nameResolver: nameRes,
		})

		remainingNode = n.NextSibling()

		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Never encounter a code block, stuck on document node
	if remainingNode == s.root {
		remainingNode = remainingNode.FirstChild()
	}

	// Skip remainingNodes unless it's got lines
	for remainingNode != nil && remainingNode.Lines().Len() == 0 {
		remainingNode = remainingNode.NextSibling()
	}

	if remainingNode != nil {
		start := remainingNode.Lines().At(0).Start
		stop := len(s.data) - 1
		markdown, _ := getRange(s.data, remainingNode, start, stop)
		blocks = append(blocks, &MarkdownBlock{content: markdown})
	}

	// Handle a single code block
	if len(blocks) == 2 {
		if b, ok := blocks[0].(*MarkdownBlock); ok && strings.HasPrefix(b.content, "```") {
			blocks = blocks[1:]
		}
	}

	return
}

type defaultParser struct {
	parser parser.Parser
}

func newDefaultParser() *defaultParser {
	return &defaultParser{parser: goldmark.DefaultParser()}
}

func (p *defaultParser) Parse(data []byte) *ParsedSource {
	root := p.parser.Parse(text.NewReader(data))
	return &ParsedSource{data: data, root: root}
}

type nameResolver struct {
	namesCounter map[string]int
	cache        map[interface{}]string
}

func (r *nameResolver) Get(obj interface{}, name string) string {
	if v, ok := r.cache[obj]; ok {
		return v
	}

	var result string

	r.namesCounter[name]++

	if r.namesCounter[name] == 1 {
		result = name
	} else {
		result = fmt.Sprintf("%s-%d", name, r.namesCounter[name])
	}

	r.cache[obj] = result

	return result
}
