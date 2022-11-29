package document

import (
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type ParsedSource struct {
	data     []byte
	renderer Renderer
	root     ast.Node
}

func (s *ParsedSource) Root() ast.Node {
	return s.root
}

func (s *ParsedSource) Source() []byte {
	return s.data
}

func (s *ParsedSource) buildBlocksTree(nameRes *nameResolver, parent ast.Node, node *Node) error {
	for astNode := parent.FirstChild(); astNode != nil; astNode = astNode.NextSibling() {
		switch astNode.Kind() {
		case ast.KindFencedCodeBlock:
			block, err := newCodeBlock(astNode.(*ast.FencedCodeBlock), nameRes, s.data, s.renderer)
			if err != nil {
				return err
			}
			node.add(block)
		case ast.KindBlockquote, ast.KindList, ast.KindListItem:
			block, err := newInnerBlock(astNode, s.data, s.renderer)
			if err != nil {
				return err
			}
			nNode := node.add(block)
			s.buildBlocksTree(nameRes, astNode, nNode)
		default:
			block, err := newMarkdownBlock(astNode, s.data, s.renderer)
			if err != nil {
				return err
			}
			node.add(block)
		}
	}
	return nil
}

func (s *ParsedSource) BlocksTree() (*Node, error) {
	nameRes := &nameResolver{
		namesCounter: map[string]int{},
		cache:        map[interface{}]string{},
	}
	tree := &Node{}
	if err := s.buildBlocksTree(nameRes, s.root, tree); err != nil {
		return nil, err
	}
	return tree, nil
}

type defaultParser struct {
	parser parser.Parser
}

func newDefaultParser() *defaultParser {
	return &defaultParser{parser: goldmark.DefaultParser()}
}

func (p *defaultParser) Parse(data []byte, r Renderer) *ParsedSource {
	root := p.parser.Parse(text.NewReader(data))
	return &ParsedSource{data: data, renderer: r, root: root}
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
