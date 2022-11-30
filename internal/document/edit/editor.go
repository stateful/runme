package edit

import (
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/md"
)

type Editor struct {
	doc *document.Document
}

func New() *Editor {
	return &Editor{}
}

func (e *Editor) Deserialize(data []byte) (*Notebook, error) {
	e.doc = document.New(data, md.Render)
	node, err := e.doc.Parse()
	if err != nil {
		return nil, err
	}
	return &Notebook{Cells: toCells(node, data)}, nil
}

func (e *Editor) Serialize(notebook *Notebook) ([]byte, error) {
	node, err := e.doc.Parse()
	if err != nil {
		return nil, err
	}
	applyCells(node, notebook.Cells)
	return e.doc.Render()
}
