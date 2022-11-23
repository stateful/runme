package document

type CellKind int

const (
	MarkupKind CellKind = iota + 1
	CodeKind
)

// Cell resembles NotebookCellData from VS Code.
// https://github.com/microsoft/vscode/blob/085c409898bbc89c83409f6a394e73130b932add/src/vscode-dts/vscode.d.ts#L13715
type Cell struct {
	Kind     CellKind       `json:"kind"`
	Value    string         `json:"value"`
	LangID   string         `json:"languageId"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Notebook resembles NotebookData form VS Code.
// https://github.com/microsoft/vscode/blob/085c409898bbc89c83409f6a394e73130b932add/src/vscode-dts/vscode.d.ts#L13767
type Notebook struct {
	Cells    []*Cell        `json:"cells"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func BlocksToCells(blocks Blocks) []*Cell {
	cells := make([]*Cell, 0, len(blocks))
	for _, block := range blocks {
		switch b := block.(type) {
		case *MarkdownBlock:
			cells = append(cells, &Cell{
				Kind:     MarkupKind,
				Value:    string(b.value),
				Metadata: make(map[string]any),
			})
		case *CodeBlock:
			metadata := make(map[string]any)
			for k, v := range b.attributes {
				metadata[k] = v
			}
			metadata["name"] = b.name

			cells = append(cells, &Cell{
				Kind:     CodeKind,
				LangID:   b.language,
				Value:    string(b.value),
				Metadata: metadata,
			})
		}
	}
	return cells
}

// func CellsToBlocks(blocks Blocks, cells []*Cell) (result Blocks) {
// 	for _, cell := range cells {
// 		blockID, ok := cell.Metadata["_blockID"].(string)
// 		if !ok {
// 			// This is a new cell. We skip this case for now.
// 			continue
// 		}

// 		var block Block
// 		for _, block = range blocks {
// 			if block.ID() == blockID {
// 				break
// 			}
// 		}

// 		if block == nil {
// 			panic("block not found")
// 		}

// 		switch cell.Kind {
// 		case MarkupKind:
// 			codeBlock := block.(*CodeBlock)
// 			codeBlock.inner.Set

// 		case CodeKind:
// 		}
// 	}
// 	return
// }
