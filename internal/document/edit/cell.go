package edit

import (
	"bytes"
	"log"

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

func applyCells(root *document.Node, cells []*Cell) {
	var blockIds btree.Map[string, bool]
	// TODO: FindNode is used as iterator. Propose a new function.
	_ = document.FindNode(root, func(n *document.Node) bool {
		if root == n {
			return false
		}
		if n.Item() != nil {
			blockIds.Set(n.Item().ID(), false)
		}
		return false
	})
	log.Printf("found blockIDs: %+v", blockIds.Keys())

	var lastNode *document.Node
	for _, cell := range cells {
		log.Printf("processing a cell: %+v", cell)

		bid, ok := cell.Metadata["_blockId"].(string)
		// A new cell is found.
		if !ok || bid == "" {
			log.Printf("new cell discovered")

			doc := document.New([]byte(cell.Value), cmark.Render)
			newNode, _, _ := doc.Parse()

			if lastNode != nil {
				idx := lastNode.Index()
				for j, child := range newNode.Children() {
					lastNode.Parent().InsertAt(idx+j+1, child.Item())
					lastNode = child
				}
			} else {
				for idx, child := range newNode.Children() {
					root.InsertAt(idx, child.Item())
					lastNode = child
				}
			}
			continue
		}
		// Trying to find a node and change its value.
		node := document.FindNode(root, func(n *document.Node) bool {
			return n.Item() != nil && n.Item().ID() == bid
		})
		if node == nil {
			log.Printf("node for blockID %s not found", bid)
			continue
		}
		// If the number of new lines is different,
		// a parser is involved as it might be more
		// drastical change.
		currentLinesCount := bytes.Count(node.Item().Value(), []byte{'\n'})
		newLinesCount := bytes.Count([]byte(cell.Value), []byte{'\n'})
		if currentLinesCount != newLinesCount {
			idx := node.Index()
			parent := node.Parent()

			log.Printf("replacing node at index: %d", idx)

			doc := document.New([]byte(cell.Value), cmark.Render)
			newNode, _, _ := doc.Parse()
			if len(newNode.Children()) != 1 {
				log.Printf("cannot convert single cell to multiple nodes")
				continue
			} else {
				newNode := newNode.Children()[0]
				// TODO: set _blockId to bid in newNode
				parent.InsertAt(idx, newNode.Item())
				lastNode = newNode
				parent.Remove(node)
			}
		} else {
			log.Printf("setting value: %s (old value: %s)", cell.Value, node.Item().Value())
			node.Item().SetValue([]byte(cell.Value))
			lastNode = node
		}

		// Mark this block visisted as well as all its children.
		blockIds.Set(bid, true)
		_ = document.FindNode(node, func(n *document.Node) bool {
			blockIds.Set(n.Item().ID(), true)
			return false
		})
		log.Printf("marked blockID %s as visited", bid)
	}

	// Make inner blocks as visited if all their children are visited.
	blockIds.Reverse(func(id string, visited bool) bool {
		if visited {
			return true
		}
		node := document.FindNode(root, func(n *document.Node) bool {
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
		node := document.FindNode(root, func(n *document.Node) bool {
			return n.Item() != nil && n.Item().ID() == id
		})
		if node != nil {
			log.Printf("removing a node: %s with blockID: %s", node.String(), id)
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
