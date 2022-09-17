package document

import (
	"fmt"
	"io/fs"

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
