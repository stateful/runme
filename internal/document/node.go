package document

import (
	"bytes"
	"strings"

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
	node *Node,
	cells *[]*Cell,
	source []byte,
) {
	toCells(node, cells, source)
}

func toCells(node *Node, cells *[]*Cell, source []byte) {
	if node == nil {
		return
	}

	for _, child := range node.children {
		switch block := child.Value().(type) {
		case *InnerBlock:
			r := new(md.Renderer)
			inListItemBlock := false
			foundCodeBlock := false

			result, _ := r.Render(
				block.Unwrap(),
				source,
				func(astNode ast.Node) ([]byte, bool) {
					return nil, false
				},
				func(n ast.Node, entering bool) (ast.WalkStatus, bool) {
					if n.Kind() == ast.KindListItem {
						inListItemBlock = entering
						if !entering {
							foundCodeBlock = false
						}
					} else if n.Kind() == ast.KindFencedCodeBlock {
						foundCodeBlock = true

						if entering {
							var buf bytes.Buffer
							_, _ = r.WriteTo(&buf)
							*cells = append(
								*cells,
								&Cell{
									Kind:     MarkupKind,
									Value:    buf.String(),
									Metadata: map[string]any{},
								},
							)

							codeBlock := FindByInner(child, n).Value().(*CodeBlock)
							metadata := make(map[string]any)
							for k, v := range codeBlock.Attributes() {
								metadata[k] = v
							}
							*cells = append(
								*cells,
								&Cell{
									Kind:     CodeKind,
									Value:    string(codeBlock.Value()),
									LangID:   codeBlock.Language(),
									Metadata: metadata,
								},
							)
						}
						return ast.WalkSkipChildren, true
					} else if inListItemBlock && foundCodeBlock && !entering {
						var buf bytes.Buffer
						_, _ = r.WriteTo(&buf)
						value := bytes.TrimPrefix(buf.Bytes(), []byte(r.Prefix()))
						if len(value) > 0 {
							*cells = append(
								*cells,
								&Cell{
									Kind:     MarkupKind,
									Value:    string(value),
									Metadata: map[string]any{},
								},
							)
						}
					}
					return ast.WalkContinue, false
				},
			)

			if len(result) > 0 {
				*cells = append(*cells, &Cell{
					Kind:     MarkupKind,
					Value:    string(result),
					Metadata: map[string]any{},
				})
			}
		case *CodeBlock:
			metadata := make(map[string]any)
			for k, v := range block.Attributes() {
				metadata[k] = v
			}
			*cells = append(*cells, &Cell{
				Kind:     CodeKind,
				Value:    string(block.Value()),
				LangID:   block.Language(),
				Metadata: metadata,
			})
		case *MarkdownBlock:
			metadata := make(map[string]any)
			for k, v := range block.Attributes() {
				metadata[k] = v
			}
			*cells = append(*cells, &Cell{
				Kind:     MarkupKind,
				Value:    string(block.Value()),
				Metadata: metadata,
			})
		}
	}
}

func Serialize(cells []*Cell) string {
	var b strings.Builder
	for _, cell := range cells {
		_, _ = b.WriteString(cell.Value)
		_ = b.WriteByte('\n')
	}
	return b.String()
}
