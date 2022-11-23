package document

import (
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type ParsedSource struct {
	data []byte
	root ast.Node
}

func (s *ParsedSource) Root() ast.Node {
	return s.root
}

func (s *ParsedSource) Source() []byte {
	return s.data
}

func (s *ParsedSource) hasChildOfKind(node ast.Node, kind ast.NodeKind) (ast.Node, bool) {
	if node.Type() != ast.TypeBlock {
		return nil, false
	}

	if node.Kind() == kind {
		return node, true
	}

	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		if node, ok := s.hasChildOfKind(c, kind); ok {
			return node, ok
		}
	}
	return nil, false
}

func hasLine(node ast.Node) bool {
	return node.Lines().Len() > 0
}

func hasLineRecursive(node ast.Node) bool {
	if hasLine(node) {
		return true
	}

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if hasLineRecursive(child) {
			return true
		}
	}

	return false
}

type extractedNode struct {
	isExtra         bool
	parent          ast.Node
	previousSibling ast.Node
}

func (s *ParsedSource) findBlocks(nameRes *nameResolver, docNode ast.Node) (result Blocks) {
	extractedCodeBlocks := make(map[ast.Node]extractedNode)

	for c := docNode.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindFencedCodeBlock:
			if !hasLine(c) {
				break
			}

			block := newCodeBlock(s.data, nameRes, c.(*ast.FencedCodeBlock))

			if info, ok := extractedCodeBlocks[c]; ok {
				block.isExtra = info.isExtra
				block.parent = info.parent
				block.previousSibling = info.previousSibling
				delete(extractedCodeBlocks, c)
			}

			result = append(result, block)

		default:
			if innerCodeBlock, ok := s.hasChildOfKind(c, ast.KindFencedCodeBlock); ok {
				if !hasLine(innerCodeBlock) {
					break
				}

				switch c.Kind() {
				case ast.KindList:
					listItem := innerCodeBlock.Parent()

					// Move the code block into the root node,
					// as well as all its next siblings.
					innerNodesToMove := []ast.Node{innerCodeBlock}
					for node := innerCodeBlock.NextSibling(); node != nil; node = node.NextSibling() {
						innerNodesToMove = append(innerNodesToMove, node)
					}

					itemToInsertAfter := c
					for _, node := range innerNodesToMove {
						extractedCodeBlocks[node] = extractedNode{
							parent:          node.Parent(),
							previousSibling: node.PreviousSibling(),
						}
						listItem.RemoveChild(listItem, node)
						docNode.InsertAfter(docNode, itemToInsertAfter, node)
						itemToInsertAfter = node
					}

					// Split the list if there are any list items
					// after listItem.
					if listItem.NextSibling() != nil {
						var itemsToMove []ast.Node
						for item := listItem.NextSibling(); item != nil; item = item.NextSibling() {
							itemsToMove = append(itemsToMove, item)
						}

						newList := ast.NewList(c.(*ast.List).Marker)
						for _, item := range itemsToMove {
							extractedCodeBlocks[listItem] = extractedNode{
								parent:          item.Parent(),
								previousSibling: item.PreviousSibling(),
							}
							c.RemoveChild(c, item)
							newList.AppendChild(newList, item)
						}
						newList.Start = c.(*ast.List).Start + c.ChildCount()
						docNode.InsertAfter(docNode, itemToInsertAfter, newList)
						extractedCodeBlocks[newList] = extractedNode{
							isExtra: true,
						}
					}
				case ast.KindBlockquote:
					nextParagraph := innerCodeBlock.NextSibling()

					// move the code block into the root node
					extractedCodeBlocks[innerCodeBlock] = extractedNode{
						parent:          innerCodeBlock.Parent(),
						previousSibling: innerCodeBlock.PreviousSibling(),
					}
					c.RemoveChild(c, innerCodeBlock)
					docNode.InsertAfter(docNode, c, innerCodeBlock)

					// move all paragraphs after the code block
					// into the new block quote
					if nextParagraph != nil {
						var itemsToMove []ast.Node
						for item := nextParagraph; item != nil; item = item.NextSibling() {
							itemsToMove = append(itemsToMove, item)
						}

						newBlockQuote := ast.NewBlockquote()
						for _, item := range itemsToMove {
							extractedCodeBlocks[item] = extractedNode{
								parent:          item.Parent(),
								previousSibling: item.PreviousSibling(),
							}
							c.RemoveChild(c, item)
							newBlockQuote.AppendChild(newBlockQuote, item)
						}
						docNode.InsertAfter(docNode, innerCodeBlock, newBlockQuote)
						extractedCodeBlocks[newBlockQuote] = extractedNode{isExtra: true}
					}
				}
			}

			if hasLineRecursive(c) || c.Kind() == ast.KindThematicBreak {
				block := newMarkdownBlock(s.data, c)

				if info, ok := extractedCodeBlocks[c]; ok {
					block.isExtra = info.isExtra
					block.parent = info.parent
					block.previousSibling = info.previousSibling
					delete(extractedCodeBlocks, c)
				}

				result = append(result, block)
			}
		}
	}
	return
}

func (s *ParsedSource) Blocks() Blocks {
	nameRes := &nameResolver{
		namesCounter: map[string]int{},
		cache:        map[interface{}]string{},
	}
	return s.findBlocks(nameRes, s.root)
}

type defaultParser struct {
	parser parser.Parser
}

func newDefaultParser() *defaultParser {
	return &defaultParser{parser: goldmark.DefaultParser()}
}

func (p *defaultParser) Parse(data []byte) *ParsedSource {
	root := p.parser.Parse(text.NewReader(data))
	return &ParsedSource{data: data, root: root}
}

type nameResolver struct {
	namesCounter map[string]int
	cache        map[interface{}]string
}

func (r *nameResolver) Get(obj interface{}, name string) string {
	if v, ok := r.cache[obj]; ok {
		return v
	}

	var result string

	r.namesCounter[name]++

	if r.namesCounter[name] == 1 {
		result = name
	} else {
		result = fmt.Sprintf("%s-%d", name, r.namesCounter[name])
	}

	r.cache[obj] = result

	return result
}
