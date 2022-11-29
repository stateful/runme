package document

import (
	"github.com/stateful/runme/internal/renderer/md"
	"github.com/yuin/goldmark/ast"
)

type Node struct {
	children []*Node
	value    Block
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
		if item, ok := child.Value().(*CodeBlock); ok {
			*result = append(*result, item)
		}
		CollectCodeBlocks(child, result)
	}
}

func ToCells(
	blocksTree *Node,
	cells *[]*Cell,
	source []byte,
) {
	toCells(blocksTree, cells, source)
}

type innerBlockProcessor struct {
	node   *Node
	source []byte

	cells           []*Cell
	foundCodeBlock  bool
	inListItemBlock bool
	renderer        *md.Renderer
}

func (p *innerBlockProcessor) callback(astNode ast.Node, entering bool) (ast.WalkStatus, bool) {
	if astNode.Kind() == ast.KindListItem {
		p.inListItemBlock = entering
		if !entering {
			p.foundCodeBlock = false // reset after leaving a list item
		}
	} else if astNode.Kind() == ast.KindFencedCodeBlock {
		if !entering {
			return ast.WalkSkipChildren, true
		}

		p.foundCodeBlock = true

		value, _ := p.renderer.BufferBytes()
		p.cells = append(
			p.cells,
			&Cell{
				Kind:     MarkupKind,
				Value:    string(value),
				Metadata: map[string]any{},
			},
		)

		codeBlock := FindByInner(p.node, astNode).Value().(*CodeBlock)
		metadata := make(map[string]any)
		for k, v := range codeBlock.Attributes() {
			metadata[k] = v
		}
		p.cells = append(
			p.cells,
			&Cell{
				Kind:     CodeKind,
				Value:    string(codeBlock.Value()),
				LangID:   codeBlock.Language(),
				Metadata: metadata,
			},
		)

		return ast.WalkSkipChildren, true
	} else if p.inListItemBlock && p.foundCodeBlock && !entering {
		// All blocks after the fenced code block within the list item
		// should be renderer as separate cells.
		value, _ := p.renderer.RawBufferBytes()
		if len(value) > 0 {
			p.cells = append(
				p.cells,
				&Cell{
					Kind:     MarkupKind,
					Value:    string(value),
					Metadata: map[string]any{},
				},
			)
		}
	}
	return ast.WalkContinue, false
}

func (p *innerBlockProcessor) Process(cells []*Cell) []*Cell {
	p.cells = cells
	p.foundCodeBlock = false
	p.inListItemBlock = false
	p.renderer = new(md.Renderer)

	leftover, _ := p.renderer.Render(
		p.node.Value().Unwrap(),
		p.source,
		func(ast.Node) ([]byte, bool) { return nil, false },
		p.callback,
	)
	if len(leftover) > 0 {
		p.cells = append(p.cells, &Cell{
			Kind:     MarkupKind,
			Value:    string(leftover),
			Metadata: map[string]any{},
		})
	}

	return p.cells
}

func toCells(blocksTree *Node, cells *[]*Cell, source []byte) {
	if blocksTree == nil {
		return
	}

	for _, child := range blocksTree.children {
		switch block := child.Value().(type) {
		case *InnerBlock:
			r := &innerBlockProcessor{
				node:   child,
				source: source,
			}
			*cells = r.Process(*cells)

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
				Metadata: attrsToMetadata(block.attributes),
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
