package document

import (
	"fmt"

	"github.com/rs/xid"
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

func (s *ParsedSource) findBlocks(nameRes *nameResolver, docNode ast.Node) (result Blocks) {
	for c := docNode.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindFencedCodeBlock:
			result = append(result, &CodeBlock{
				id:           newID(),
				inner:        c.(*ast.FencedCodeBlock),
				nameResolver: nameRes,
				source:       s.data,
			})
		default:
			if innerCodeBlock, ok := s.hasChildOfKind(c, ast.KindFencedCodeBlock); ok {
				fmt.Printf("found inner fenced code block\n")

				switch c.Kind() {
				case ast.KindList:
					listItem := innerCodeBlock.Parent()

					// move the code block into the root node
					listItem.RemoveChild(listItem, innerCodeBlock)
					docNode.InsertAfter(docNode, c, innerCodeBlock)

					// split the list if there are any list items
					// after listItem
					if listItem.NextSibling() != nil {
						newList := ast.NewList(c.(*ast.List).Marker)
						for item := listItem.NextSibling(); item != nil; item = item.NextSibling() {
							c.RemoveChild(c, item)
							newList.AppendChild(newList, item)
						}
						docNode.InsertAfter(docNode, innerCodeBlock, newList)
					}
				case ast.KindBlockquote:
					nextParagraph := innerCodeBlock.NextSibling()

					// move the code block into the root node
					c.RemoveChild(c, innerCodeBlock)
					docNode.InsertAfter(docNode, c, innerCodeBlock)

					// move all paragraphs after the code block
					// into the new block quote
					if nextParagraph != nil {
						newBlockQuote := ast.NewBlockquote()
						for item := nextParagraph; item != nil; item = item.NextSibling() {
							c.RemoveChild(c, item)
							newBlockQuote.AppendChild(newBlockQuote, item)
						}
						docNode.InsertAfter(docNode, innerCodeBlock, newBlockQuote)
					}
				}
			}

			result = append(result, &MarkdownBlock{
				id:     newID(),
				inner:  c,
				source: s.data,
			})
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

func (s *ParsedSource) CodeBlocks() CodeBlocks {
	var result CodeBlocks

	nameRes := &nameResolver{
		namesCounter: map[string]int{},
		cache:        map[interface{}]string{},
	}

	// TODO(adamb): check the case when a paragraph is immediately
	// followed by a code block without a new line separating them.
	// Currently, such a code block is not detected at all.
	for c := s.root.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Kind() == ast.KindFencedCodeBlock {
			result = append(result, &CodeBlock{
				id:           newID(),
				inner:        c.(*ast.FencedCodeBlock),
				nameResolver: nameRes,
				source:       s.data,
			})
		}
	}

	return result
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

func newID() string {
	return xid.New().String()
}
