package edit

import (
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/cmark"
)

type Editor struct{}

func New() *Editor {
	return &Editor{}
}

func (e *Editor) Deserialize(data []byte) (*Notebook, error) {
	doc := document.New(data, cmark.Render)
	node, _, err := doc.Parse()
	if err != nil {
		return nil, err
	}
	return &Notebook{Cells: toCells(node, data)}, nil
}

func (e *Editor) Serialize(notebook *Notebook) ([]byte, error) {
	result := serializeCells(notebook.Cells)
	return result, nil
}
