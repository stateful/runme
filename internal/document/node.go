package document

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

func FindNodePreOrder(node *Node, fn func(*Node) bool) *Node {
	if node == nil {
		return nil
	}
	if fn(node) {
		return node
	}
	for _, child := range node.children {
		if n := FindNode(child, fn); n != nil {
			return n
		}
	}
	return nil
}
