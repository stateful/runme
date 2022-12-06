package edit

import (
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/cmark"
)

type Editor struct {
	doc *document.Document
}

func New() *Editor {
	return &Editor{}
}

func (e *Editor) Deserialize(data []byte) (*Notebook, error) {
	e.doc = document.New(data, cmark.Render)
	node, _, err := e.doc.Parse()
	if err != nil {
		return nil, err
	}
	return &Notebook{Cells: toCells(node, data)}, nil
}

func (e *Editor) Serialize(notebook *Notebook) ([]byte, error) {
	node, _, err := e.doc.Parse()
	if err != nil {
		return nil, err
	}

	applyCells(node, notebook.Cells)

	result := node.Bytes()
	e.doc = document.New(result, cmark.Render)
	_, _, err = e.doc.Parse()
	if err != nil {
		return nil, err
	}
	return result, nil
}
