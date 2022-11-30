package editor

import (
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/md"
)

type Editor struct {
	blocksTree *document.Node
	parsed     *document.ParsedSource
	source     *document.Source
}

func (e *Editor) Deserialize(data []byte) ([]*document.Cell, error) {
	e.source = document.NewSource(data, md.Render)
	e.parsed = e.source.Parse()

	var err error
	e.blocksTree, err = e.parsed.BlocksTree()
	if err != nil {
		return nil, err
	}

	var cells []*document.Cell
	document.ToCells(e.blocksTree, &cells, data)
	return cells, nil
}

func (e *Editor) Serialize(cells []*document.Cell) ([]byte, error) {
	document.SyncCells(e.blocksTree, cells)
	return md.Render(e.parsed.Root(), e.parsed.Source())
}
