package edit

import (
	"bytes"

	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/cmark"
	"github.com/tidwall/btree"
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
	var blockIds btree.Map[string, bool]
	// TODO: FindNode is used as iterator. Propose a new function.
	_ = document.FindNode(node, func(n *document.Node) bool {
		if node == n {
			return false
		}
		if n.Item() != nil {
			blockIds.Set(n.Item().ID(), false)
		}
		return false
	})

	var lastNode *document.Node
	for _, cell := range cells {
		bid, ok := cell.Metadata["_blockId"].(string)
		// A new cell is found.
		if !ok || bid == "" {
			doc := document.New([]byte(cell.Value), cmark.Render)
			newNode, _, _ := doc.Parse()

			if lastNode != nil {
				for _, child := range newNode.Children() {
					idx := lastNode.Index()
					lastNode.Parent().InsertAt(idx+1, child.Item())
					lastNode = child
				}
			} else {
				doc := document.New([]byte(cell.Value), cmark.Render)
				newNode, _, _ := doc.Parse()
				for idx, child := range newNode.Children() {
					node.InsertAt(idx, child.Item())
					lastNode = child
				}
			}
			continue
		}
		// Trying to find a node and change its value.
		node := document.FindNode(node, func(n *document.Node) bool {
			return n.Item() != nil && n.Item().ID() == bid
		})
		if node == nil {
			continue
		}
		// If the number of new lines is different,
		// a parser is involved as it might be more
		// drastical change.
		currentLinesCount := bytes.Count(node.Item().Value(), []byte{'\n'})
		newLinesCount := bytes.Count([]byte(cell.Value), []byte{'\n'})
		if currentLinesCount != newLinesCount {
			idx := node.Index()

			doc := document.New([]byte(cell.Value), cmark.Render)
			newNode, _, _ := doc.Parse()
			for _, child := range newNode.Children() {
				node.Parent().InsertAt(idx, child.Item())
				lastNode = child
			}

			node.Parent().Remove(node)
		} else {
			node.Item().SetValue([]byte(cell.Value))
			lastNode = node
		}

		// Mark this block visisted as well as all its children.
		blockIds.Set(bid, true)
		_ = document.FindNode(node, func(n *document.Node) bool {
			blockIds.Set(n.Item().ID(), true)
			return false
		})
	}

	// Make inner blocks as visited if all their children are visited.
	blockIds.Reverse(func(id string, visited bool) bool {
		if visited {
			return true
		}
		node := document.FindNode(node, func(n *document.Node) bool {
			return n.Item().ID() == id
		})
		if len(node.Children()) == 0 {
			return true
		}
		notVisitedChild := document.FindNode(node, func(n *document.Node) bool {
			visited, found := blockIds.Get(n.Item().ID())
			return n != node && found && !visited
		})
		if notVisitedChild == nil {
			blockIds.Set(id, true)
		}
		return true
	})

	blockIds.Scan(func(id string, visited bool) bool {
		if visited {
			return true
		}
		node := document.FindNode(node, func(n *document.Node) bool {
			return n.Item() != nil && n.Item().ID() == id
		})
		if node != nil {
			node.Parent().Remove(node)
		}
		return true
	})
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
