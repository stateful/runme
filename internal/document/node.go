package document

import (
	"bytes"
)

type Node struct {
	children []*Node
	parent   *Node
	item     Block
}

func (n *Node) Children() []*Node {
	return n.children
}

func (n *Node) Index() int {
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

func (n *Node) InsertAt(idx int, item Block) *Node {
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

func (n *Node) Parent() *Node {
	return n.parent
}

func (n *Node) Remove(nodeToRemove *Node) bool {
	if len(n.children) == 0 {
		return false
	}
	idx := -1
	for i := 0; i < len(n.children); i++ {
		if n.children[i] == nodeToRemove {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false
	}
	if idx == 0 {
		n.children = n.children[1:]
	} else if idx == len(n.children)-1 {
		n.children = n.children[:len(n.children)-1]
	} else {
		n.children = append(n.children[0:idx], n.children[idx+1:]...)
	}
	return true
}

func (n *Node) Bytes() []byte {
	var b bytes.Buffer
	for idx, child := range n.children {
		_, _ = b.Write(child.Item().Value())
		if idx != len(n.children)-1 {
			_ = b.WriteByte('\n')
		}
	}
	return b.Bytes()
}

func (n *Node) String() string {
	return string(n.Bytes())
}

func (n *Node) add(item Block) *Node {
	node := &Node{
		item:   item,
		parent: n,
	}
	n.children = append(n.children, node)
	return node
}

func CollectCodeBlocks(node *Node) (result CodeBlocks) {
	collectCodeBlocks(node, &result)
	return
}

func collectCodeBlocks(node *Node, result *CodeBlocks) {
	if node == nil {
		return
	}
	for _, child := range node.children {
		if item, ok := child.Item().(*CodeBlock); ok {
			*result = append(*result, item)
		}
		collectCodeBlocks(child, result)
	}
}

func FindNode(node *Node, fn func(*Node) bool) *Node {
	if node == nil {
		return nil
	}
	for _, child := range node.children {
		if n := FindNode(child, fn); n != nil {
			return n
		}
	}
	if fn(node) {
		return node
	}
	return nil
}
