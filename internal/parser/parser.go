package parser

import (
	"io"

	"github.com/pkg/errors"
	"github.com/stateful/rdme/internal/renderer"
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

func (p *Parser) Render(w io.Writer) error {
	mdr := goldmark.New(goldmark.WithRenderer(renderer.NewJSON(p.src)))
	err := mdr.Renderer().Render(w, p.src, p.rootNode)
	if err != nil {
		return errors.Wrapf(err, "error rendering to json doc")
	}

	return nil
}
