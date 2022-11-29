package document

import (
	"github.com/stateful/runme/internal/renderer/md"
	"github.com/yuin/goldmark/ast"
)

type Node struct {
	children []*Node
	item     Block
	parent   *Node
}

func (n *Node) add(item Block) *Node {
	node := &Node{
		item:   item,
		parent: n,
	}
	n.children = append(n.children, node)
	return node
}

func (n *Node) index() int {
	if n.parent == nil {
		return -1
	}
	i := 0
	for ; i < len(n.parent.children); i++ {
		if n.parent.children[i] == n {
			break
		}
	}
	return i
}

func (n *Node) insertAt(item Block, idx int) *Node {
	node := &Node{
		item:   item,
		parent: n,
	}
	if idx == len(n.children) {
		n.children = append(n.children, node)
	} else {
		n.children = append(n.children[:idx+1], n.children[idx:]...)
		n.children[idx] = node
	}
	return node
}

func (n *Node) Item() Block {
	return n.item
}

func FindByInner(node *Node, inner ast.Node) *Node {
	if node == nil {
		return nil
	}

	if node.item != nil && node.item.Unwrap() == inner {
		return node
	}

	for _, child := range node.children {
		if n := FindByInner(child, inner); n != nil {
			return n
		}
	}

	return nil
}

func CollectCodeBlocks(node *Node, result *CodeBlocks) {
	if node == nil {
		return
	}

	for _, child := range node.children {
		if item, ok := child.Item().(*CodeBlock); ok {
			*result = append(*result, item)
		}
		CollectCodeBlocks(child, result)
	}
}

func findByBlockKind(node *Node, kind blockKind) *Node {
	if node == nil {
		return nil
	}

	if node.item != nil && node.item.kind() == kind {
		return node
	}

	for _, child := range node.children {
		if n := findByBlockKind(child, kind); n != nil {
			return n
		}
	}

	return nil
}

func ToCells(
	blocksTree *Node,
	cells *[]*Cell,
	source []byte,
) {
	toCells(blocksTree, cells, source)
}

func toCells(blocksTree *Node, cells *[]*Cell, source []byte) {
	if blocksTree == nil {
		return
	}

	for _, child := range blocksTree.children {
		switch block := child.Item().(type) {
		case *InnerBlock:
			switch block.inner.Kind() {
			case ast.KindList:
				if n := findByBlockKind(child, codeBlock); n == nil {
					*cells = append(*cells, &Cell{
						Kind:     MarkupKind,
						Value:    string(block.Value()),
						Metadata: attrsToMetadata(block.Attributes()),
					})
				} else {
					for _, listItemNode := range child.children {
						nodeWithCodeBlock := findByBlockKind(listItemNode, codeBlock)
						if nodeWithCodeBlock != nil {
							toCells(listItemNode, cells, source)
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
				nodeWithCodeBlock := findByBlockKind(child, codeBlock)
				if nodeWithCodeBlock != nil {
					toCells(child, cells, source)
				} else {
					*cells = append(*cells, &Cell{
						Kind:     MarkupKind,
						Value:    string(block.Value()),
						Metadata: attrsToMetadata(block.Attributes()),
					})
				}
			}

		case *CodeBlock:
			*cells = append(*cells, &Cell{
				Kind:     CodeKind,
				Value:    string(block.Value()),
				LangID:   block.Language(),
				Metadata: attrsToMetadata(block.Attributes()),
			})

		case *MarkdownBlock:
			*cells = append(*cells, &Cell{
				Kind:     MarkupKind,
				Value:    string(block.Value()),
				Metadata: attrsToMetadata(block.Attributes()),
			})
		}
	}
}

func findByID(node *Node, id string) *Node {
	if node == nil {
		return nil
	}
	if block := node.Item(); block != nil {
		if bid := block.Attributes()["_blockId"]; bid == id {
			return node
		}
	} else {
		// TODO
	}
	for _, child := range node.children {
		if n := findByID(child, id); n != nil {
			return n
		}
	}
	return nil
}

func iterTree(node *Node, fn func(*Node)) {
	if node == nil {
		return
	}
	for _, child := range node.children {
		iterTree(child, fn)
	}
	fn(node)
}

func SyncCells(blocksTree *Node, cells []*Cell) {
	blockIds := map[string]bool{}
	iterTree(blocksTree, func(n *Node) {
		block := n.Item()
		if block == nil {
			// TODO
		} else {
			blockIds[block.id()] = false
		}
	})

	var lastNode *Node

	for _, cell := range cells {
		bid, ok := cell.Metadata["_blockId"].(string)
		if !ok || bid == "" {
			if lastNode != nil {
				source := NewSource([]byte(cell.Value), md.Render)
				parsed := source.Parse()
				blocksTree, _ := parsed.BlocksTree()
				for _, child := range blocksTree.children {
					idx := lastNode.index()
					lastNode.parent.insertAt(child.item, idx+1)
				}
			}
			continue
		}
		node := findByID(blocksTree, bid)
		if node == nil {
			continue
		}
		node.item.setValue([]byte(cell.Value))
		blockIds[bid] = true
		lastNode = node
	}

	for bid, visited := range blockIds {
		if !visited {
			node := findByID(blocksTree, bid)
			if node == nil {
				continue
			}
			node.item.setValue(nil)
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
