package editor

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/stateful/runme/internal/document"
	"github.com/yuin/goldmark/ast"
	"golang.org/x/exp/slices"
)

const (
	internalAttributePrefix = "runme.dev"
	privateAttributePrefix  = "_"
)

type CellKind int

const (
	MarkupKind CellKind = iota + 1
	CodeKind
)

type TextRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Cell resembles NotebookCellData from VS Code.
// https://github.com/microsoft/vscode/blob/085c409898bbc89c83409f6a394e73130b932add/src/vscode-dts/vscode.d.ts#L13715
type Cell struct {
	Kind       CellKind          `json:"kind"`
	Value      string            `json:"value"`
	LanguageID string            `json:"languageId"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	TextRange  *TextRange        `json:"textRange,omitempty"`
}

// Notebook resembles NotebookData form VS Code.
// https://github.com/microsoft/vscode/blob/085c409898bbc89c83409f6a394e73130b932add/src/vscode-dts/vscode.d.ts#L13767
type Notebook struct {
	Cells    []*Cell           `json:"cells"`
	Metadata map[string]string `json:"metadata,omitempty"`

	contentOffset int

	parsedFrontmatter *document.Frontmatter
}

func (n *Notebook) GetContentOffset() int {
	return n.contentOffset
}

func (n *Notebook) ParsedFrontmatter() (document.Frontmatter, *document.FrontmatterParseInfo) {
	raw, ok := n.Metadata[FrontmatterKey]
	if n.parsedFrontmatter != nil {
		return *n.parsedFrontmatter, nil
	}

	if !ok {
		return document.Frontmatter{}, nil
	}

	f, pi := document.ParseFrontmatter(raw)
	n.parsedFrontmatter = &f

	return f, &pi
}

func toCells(node *document.Node, source []byte) (result []*Cell) {
	toCellsRec(node, &result, source)
	return
}

func toCellsRec(
	node *document.Node,
	cells *[]*Cell,
	source []byte,
) {
	if node == nil {
		return
	}

	for childIdx, child := range node.Children() {
		switch block := child.Item().(type) {
		case *document.InnerBlock:
			switch block.Unwrap().Kind() {
			case ast.KindList:
				nodeWithCode := document.FindNode(child, func(n *document.Node) bool {
					return n.Item().Kind() == document.CodeBlockKind
				})
				if nodeWithCode == nil {
					*cells = append(*cells, &Cell{
						Kind:  MarkupKind,
						Value: fmtValue(block.Value()),
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
								Kind:  MarkupKind,
								Value: fmtValue(listItemNode.Item().Value()),
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
						Kind:  MarkupKind,
						Value: fmtValue(block.Value()),
					})
				}
			}

		case *document.CodeBlock:
			textRange := block.TextRange()

			// If the lang is unknown (empty) or supported then return a code cell.
			// Otherwise, return a markup cell (#85).
			// In the future, we will include language detection (#77).
			if lang := block.Language(); lang == "" || isEditorSupported(lang) {
				metadata := block.Attributes()
				metadata[prefixAttributeName(internalAttributePrefix, "name")] = block.Name()
				*cells = append(*cells, &Cell{
					Kind:       CodeKind,
					Value:      string(block.Content()),
					LanguageID: block.Language(),
					Metadata:   metadata,
					TextRange: &TextRange{
						Start: textRange.Start,
						End:   textRange.End,
					},
				})
			} else {
				*cells = append(*cells, &Cell{
					Kind:  MarkupKind,
					Value: fmtValue(block.Value()),
				})
			}

		case *document.MarkdownBlock:
			value := block.Value()

			isListItem := node.Item() != nil && node.Item().Unwrap().Kind() == ast.KindListItem
			if childIdx == 0 && isListItem {
				listItem := node.Item().Unwrap().(*ast.ListItem)
				list := listItem.Parent().(*ast.List)

				var prefix []byte

				if !list.IsOrdered() {
					prefix = append(prefix, []byte{list.Marker, ' '}...)
				} else {
					itemNumber := list.Start
					tmp := node.Item().Unwrap()
					for tmp.PreviousSibling() != nil {
						tmp = tmp.PreviousSibling()
						itemNumber++
					}
					prefix = append([]byte(strconv.Itoa(itemNumber)), '.', ' ')
				}

				value = append(prefix, value...)
			}

			*cells = append(*cells, &Cell{
				Kind:  MarkupKind,
				Value: fmtValue(value),
			})
		}
	}
}

func countTrailingNewLines(b []byte) int {
	count := 0
	lastIdx := 0
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] == '\n' {
			lastIdx = i
			count++
		} else if b[i] == '\r' && lastIdx-i == 1 {
			continue
		} else {
			break
		}
	}
	return count
}

func prefixAttributeName(prefix, name string) string {
	switch prefix {
	case internalAttributePrefix:
		return prefix + "/" + name
	case privateAttributePrefix:
		fallthrough
	default:
		return prefix + name
	}
}

func serializeFencedCodeAttributes(w io.Writer, cell *Cell) {
	// Filter out private keys, i.e. starting with "_" or "runme.dev/".
	// A key with a name "index" that comes from VS Code is also filtered out.
	keys := make([]string, 0, len(cell.Metadata))
	for k := range cell.Metadata {
		if k == "index" || strings.HasPrefix(k, privateAttributePrefix) || strings.HasPrefix(k, internalAttributePrefix) {
			continue
		}
		keys = append(keys, k)
	}
	// Sort attributes by key, however, keep the element
	// with the key "name" in front.
	slices.SortFunc(keys, func(a, b string) bool {
		if a == "name" {
			return true
		}
		if b == "name" {
			return false
		}
		return a < b
	})
	if len(keys) == 0 {
		return
	}

	_, _ = w.Write([]byte{' ', '{', ' '})
	i := 0
	for _, k := range keys {
		v := cell.Metadata[k]
		_, _ = w.Write([]byte(fmt.Sprintf("%s=%v", k, v)))
		i++
		if i < len(keys) {
			_, _ = w.Write([]byte{' '})
		}
	}
	_, _ = w.Write([]byte{' ', '}'})
}

func serializeCells(cells []*Cell) []byte {
	var buf bytes.Buffer

	for idx, cell := range cells {
		value := cell.Value

		switch cell.Kind {
		case CodeKind:
			ticksCount := longestBacktickSeq(value)
			if ticksCount < 3 {
				ticksCount = 3
			}

			_, _ = buf.Write(bytes.Repeat([]byte{'`'}, ticksCount))
			_, _ = buf.WriteString(cell.LanguageID)

			serializeFencedCodeAttributes(&buf, cell)

			_ = buf.WriteByte('\n')
			_, _ = buf.WriteString(cell.Value)
			_ = buf.WriteByte('\n')
			_, _ = buf.Write(bytes.Repeat([]byte{'`'}, ticksCount))

		case MarkupKind:
			_, _ = buf.WriteString(cell.Value)
		}

		nlRequired := 2
		if idx == len(cells)-1 {
			nlRequired = 1
		}
		nlCount := countTrailingNewLines(buf.Bytes())
		for i := nlCount; i < nlRequired; i++ {
			_ = buf.WriteByte('\n')
		}
	}

	return buf.Bytes()
}

func longestBacktickSeq(data string) int {
	longest, current := 0, 0
	for _, b := range data {
		if b == '`' {
			current++
		} else {
			if current > longest {
				longest = current
			}
			current = 0
		}
	}
	return longest
}

func fmtValue(s []byte) string {
	return string(trimRightNewLine(s))
}

func trimRightNewLine(s []byte) []byte {
	s = bytes.TrimRight(s, "\r\n")
	return bytes.TrimRight(s, "\n")
}

var supportedExecutables = []string{
	"bash",
	"bat", // fallback to sh
	"sh",
	"shell",
	"zsh",
}

func isEditorSupported(lang string) bool {
	for _, item := range supportedExecutables {
		if item == lang {
			return true
		}
	}
	return false
}
