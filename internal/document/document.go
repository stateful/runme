package document

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type Document struct {
	astNode      ast.Node
	nameResolver *nameResolver
	node         *Node
	parser       parser.Parser
	renderer     Renderer
	source       []byte
}

func New(source []byte, renderer Renderer) *Document {
	return &Document{
		nameResolver: &nameResolver{
			namesCounter: map[string]int{},
			cache:        map[interface{}]string{},
		},
		parser:   goldmark.DefaultParser(),
		renderer: renderer,
		source:   source,
	}
}

func (d *Document) Parse() (*Node, ast.Node, error) {
	if d.astNode == nil {
		d.astNode = d.parse()
	}

	if d.node == nil {
		node := &Node{}
		if err := d.buildBlocksTree(d.astNode, node); err != nil {
			return nil, nil, errors.WithStack(err)
		}
		d.node = node
	}

	return d.node, d.astNode, nil
}

func (d *Document) parse() ast.Node {
	return d.parser.Parse(text.NewReader(d.source))
}

func (d *Document) buildBlocksTree(parent ast.Node, node *Node) error {
	for astNode := parent.FirstChild(); astNode != nil; astNode = astNode.NextSibling() {
		switch astNode.Kind() {
		case ast.KindFencedCodeBlock:
			block, err := newCodeBlock(
				astNode.(*ast.FencedCodeBlock),
				d.nameResolver,
				d.source,
				d.renderer,
			)
			if err != nil {
				return errors.WithStack(err)
			}
			node.add(block)
		case ast.KindBlockquote, ast.KindList, ast.KindListItem:
			block, err := newInnerBlock(astNode, d.source, d.renderer)
			if err != nil {
				return errors.WithStack(err)
			}
			nNode := node.add(block)
			if err := d.buildBlocksTree(astNode, nNode); err != nil {
				return err
			}
		default:
			block, err := newMarkdownBlock(astNode, d.source, d.renderer)
			if err != nil {
				return errors.WithStack(err)
			}
			node.add(block)
		}
	}
	return nil
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
