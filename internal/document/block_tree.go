package document

import "github.com/yuin/goldmark/ast"

type Node struct {
	children []*Node
	value    Block
}

func newNode(value Block) *Node {
	return &Node{
		value: value,
	}
}

func (n *Node) Value() Block {
	return n.value
}

func (n *Node) Add(value Block) *Node {
	node := &Node{
		value: value,
	}
	n.children = append(n.children, node)
	return node
}

func FindByInner(node *Node, inner ast.Node) *Node {
	if node == nil {
		return nil
	}

	if node.value != nil && node.value.Unwrap() == inner {
		return node
	}

	if len(node.children) > 0 {
		for _, child := range node.children {
			if n := FindByInner(child, inner); n != nil {
				return n
			}
		}
	}

	return nil
}
