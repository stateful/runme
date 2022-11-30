package edit

import (
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/md"
)

type Editor struct {
	doc  *document.Document
	node *document.Node
}

func New() *Editor {
	return &Editor{}
}

func (e *Editor) Deserialize(data []byte) ([]*document.Cell, error) {
	e.doc = document.NewDocument(data, md.Render)
	var err error
	e.node, err = e.doc.Parse()
	if err != nil {
		return nil, err
	}
	return document.ToCells(e.node, data), nil
}

func (e *Editor) Serialize(cells []*document.Cell) ([]byte, error) {
	document.ApplyCells(e.node, cells)
	return e.doc.Render()
}
