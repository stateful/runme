package edit

import (
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/md"
	"github.com/yuin/goldmark/ast"
)

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

func applyCells(node *document.Node, cells []*Cell) {
	blockIds := map[string]bool{}
	// TODO: FindNode is used as iterator. Propose a new function.
	_ = document.FindNode(node, func(n *document.Node) bool {
		if n.Item() != nil {
			blockIds[n.Item().ID()] = false
		}
		return false
	})

	var lastNode *document.Node
	for _, cell := range cells {
		bid, ok := cell.Metadata["_blockId"].(string)
		if !ok || bid == "" {
			if lastNode != nil {
				doc := document.New([]byte(cell.Value), md.Render)
				node, _ := doc.Parse()
				for _, child := range node.Children() {
					idx := lastNode.Index()
					lastNode.Parent().InsertAt(idx+1, child.Item())
				}
			}
			// TODO: it might be the first node so it should be prepended.
			continue
		}
		node := document.FindNode(node, func(n *document.Node) bool {
			return n.Item() != nil && n.Item().ID() == bid
		})
		if node == nil {
			continue
		}
		node.Item().SetValue([]byte(cell.Value))
		// Mark this block as well as all its children as visited.
		blockIds[bid] = true
		_ = document.FindNode(node, func(n *document.Node) bool {
			blockIds[n.Item().ID()] = true
			return false
		})
		lastNode = node
	}

	for bid, visited := range blockIds {
		if !visited {
			node := document.FindNode(node, func(n *document.Node) bool {
				return n.Item() != nil && n.Item().ID() == bid
			})
			if node == nil {
				continue
			}
			node.Item().SetValue(nil)
		}
	}
}

func toCells(node *document.Node, source []byte) (result []*Cell) {
	toCellsRec(node, &result, source)
	return
}

func toCellsRec(node *document.Node, cells *[]*Cell, source []byte) {
	if node == nil {
		return
	}

	for _, child := range node.Children() {
		switch block := child.Item().(type) {
		case *document.InnerBlock:
			switch block.Unwrap().Kind() {
			case ast.KindList:
				nodeWithCode := document.FindNode(child, func(n *document.Node) bool {
					return n.Item().Kind() == document.CodeBlockKind
				})
				if nodeWithCode == nil {
					*cells = append(*cells, &Cell{
						Kind:     MarkupKind,
						Value:    string(block.Value()),
						Metadata: attrsToMetadata(block.Attributes()),
					})
				} else {
					for _, listItemNode := range child.Children() {
						nodeWithCode := document.FindNode(listItemNode, func(n *document.Node) bool {
							return n.Item().Kind() == document.CodeBlockKind
						})
						if nodeWithCode != nil {
							toCellsRec(listItemNode, cells, source)
						} else {
							*cells = append(*cells, &Cell{
								Kind:     MarkupKind,
								Value:    string(listItemNode.Item().Value()),
								Metadata: attrsToMetadata(listItemNode.Item().Attributes()),
							})
						}
					}
				}

			case ast.KindBlockquote:
				nodeWithCode := document.FindNode(child, func(n *document.Node) bool {
					return n.Item().Kind() == document.CodeBlockKind
				})
				if nodeWithCode != nil {
					toCellsRec(child, cells, source)
				} else {
					*cells = append(*cells, &Cell{
						Kind:     MarkupKind,
						Value:    string(block.Value()),
						Metadata: attrsToMetadata(block.Attributes()),
					})
				}
			}

		case *document.CodeBlock:
			*cells = append(*cells, &Cell{
				Kind:     CodeKind,
				Value:    string(block.Value()),
				LangID:   block.Language(),
				Metadata: attrsToMetadata(block.Attributes()),
			})

		case *document.MarkdownBlock:
			*cells = append(*cells, &Cell{
				Kind:     MarkupKind,
				Value:    string(block.Value()),
				Metadata: attrsToMetadata(block.Attributes()),
			})
		}
	}
}

func attrsToMetadata(m map[string]string) map[string]any {
	metadata := make(map[string]any)
	for k, v := range m {
		metadata[k] = v
	}
	return metadata
}
