package editor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/stateful/runme/internal/document"
	ulid "github.com/stateful/runme/internal/ulid"
	"github.com/yuin/goldmark/ast"
)

const (
	InternalAttributePrefix = "runme.dev"
	PrivateAttributePrefix  = "_"
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
	Cells       []*Cell               `json:"cells"`
	Metadata    map[string]string     `json:"metadata,omitempty"`
	Frontmatter *document.Frontmatter `json:"frontmatter,omitempty"`
}

// This mimics what otherwise would happen in the extension
func (n *Notebook) ForceLifecyleIdentities() {
	for _, c := range n.Cells {
		id, ok := c.Metadata[PrefixAttributeName(InternalAttributePrefix, "id")]
		if !ok && id == "" || !ulid.ValidID(id) {
			continue
		}
		c.Metadata["id"] = id
	}
}

func toCells(doc *document.Document, node *document.Node, source []byte) (result []*Cell) {
	toCellsRec(doc, node, &result, source)
	return
}

func toCellsRec(
	doc *document.Document,
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
							toCellsRec(doc, listItemNode, cells, source)
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
					toCellsRec(doc, child, cells, source)
				} else {
					*cells = append(*cells, &Cell{
						Kind:  MarkupKind,
						Value: fmtValue(block.Value()),
					})
				}
			}

		case *document.CodeBlock:
			textRange := block.TextRange()

			// In the future, we will include language detection (#77).
			metadata := block.Attributes()
			cellID := block.ID()
			if cellID != "" {
				metadata[PrefixAttributeName(InternalAttributePrefix, "id")] = cellID
			}
			metadata[PrefixAttributeName(InternalAttributePrefix, "name")] = block.Name()
			*cells = append(*cells, &Cell{
				Kind:       CodeKind,
				Value:      string(block.Content()),
				LanguageID: block.Language(),
				Metadata:   metadata,
				TextRange: &TextRange{
					Start: textRange.Start + doc.ContentOffset(),
					End:   textRange.End + doc.ContentOffset(),
				},
			})

		case *document.MarkdownBlock:
			value := block.Value()
			astNode := block.Unwrap()

			metadata := make(map[string]string)
			_, includeAstMetadata := os.LookupEnv("RUNME_AST_METADATA")

			if includeAstMetadata {
				astMetadata := DumpToMap(astNode, source, astNode.Kind().String())
				jsonAstMetaData, err := json.Marshal(astMetadata)
				if err != nil {
					log.Fatalf("Error converting to JSON: %s", err)
				}

				metadata["runme.dev/ast"] = string(jsonAstMetaData)
			}

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
				Kind:     MarkupKind,
				Value:    fmtValue(value),
				Metadata: metadata,
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

func PrefixAttributeName(prefix, name string) string {
	switch prefix {
	case InternalAttributePrefix:
		return prefix + "/" + name
	case PrivateAttributePrefix:
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
		if k == "index" || strings.HasPrefix(k, PrivateAttributePrefix) || strings.HasPrefix(k, InternalAttributePrefix) || len(k) == 0 {
			continue
		}
		keys = append(keys, k)
	}

	attr := make(document.Attributes, len(keys))

	for _, k := range keys {
		if len(k) <= 0 {
			continue
		}

		attr[k] = cell.Metadata[k]
	}

	if len(attr) > 0 {
		_, _ = w.Write([]byte{' '})
		_ = document.DefaultDocumentParser.Write(attr, w)
	}
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

func DumpToMap(node ast.Node, source []byte, root string) *map[string]interface{} {
	metadata := make(map[string]interface{})

	metadata["Kind"] = node.Kind().String()

	if node.Type() == ast.TypeBlock {
		buf := []string{}

		for i := 0; i < node.Lines().Len(); i++ {
			line := node.Lines().At(i)
			buf = append(buf, string(line.Value(source)))
		}

		metadata["RawText"] = strings.Join(buf, "")
	}

	for name, value := range DumpAttributes(node, source) {
		metadata[name] = value
	}

	children := []interface{}{}

	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		childrenMetadata := DumpToMap(c, source, node.Kind().String())
		children = append(children, childrenMetadata)
	}

	if len(children) > 0 {
		metadata["Children"] = children
	}

	return &metadata
}

func DumpAttributes(n ast.Node, source []byte) map[string]interface{} {
	attributes := make(map[string]interface{})

	switch n.Kind() {
	case ast.KindHeading:
		t := n.(*ast.Heading)
		attributes["Level"] = t.Level

	case ast.KindText:
		t := n.(*ast.Text)

		buf := []string{}
		if t.SoftLineBreak() {
			buf = append(buf, "SoftLineBreak")
		}
		if t.HardLineBreak() {
			buf = append(buf, "HardLineBreak")
		}
		if t.IsRaw() {
			buf = append(buf, "Raw")
		}

		// TODO: IsCode is not available in ast.Text
		// if t.IsCode() {
		// 	buf = append(buf, "Code")
		// }

		fs := strings.Join(buf, ", ")
		if len(fs) != 0 {
			fs = "(" + fs + ")"
		}

		attributes[fmt.Sprintf("Text%s", fs)] = strings.TrimRight(string(t.Text(source)), "\n")

	case ast.KindLink:
		t := n.(*ast.Link)
		attributes["Destination"] = string(t.Destination)
		attributes["Title"] = string(t.Title)

	case ast.KindList:
		t := n.(*ast.List)
		attributes["Ordered"] = t.IsOrdered()
		attributes["Marker"] = fmt.Sprintf("%c", t.Marker)
		attributes["Tight"] = t.IsTight

		if t.IsOrdered() {
			attributes["Start"] = fmt.Sprintf("%d", t.Start)
		}

	case ast.KindListItem:
		t := n.(*ast.ListItem)
		attributes["Offset"] = t.Offset
	}

	return attributes
}
