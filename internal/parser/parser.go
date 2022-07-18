package parser

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type Parser struct {
	rootNode ast.Node
	src      []byte
}

func New(src []byte) *Parser {
	mdp := goldmark.DefaultParser()
	rootNode := mdp.Parse(text.NewReader(src))

	return &Parser{
		rootNode: rootNode,
		src:      src,
	}
}
