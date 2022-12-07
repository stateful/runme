package edit

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"

	"github.com/stateful/runme/internal/document"
	"github.com/yuin/goldmark/ast"
)

type CellKind int

const (
	MarkupKind CellKind = iota + 1
	CodeKind
)

// Cell resembles NotebookCellData from VS Code.
// https://github.com/microsoft/vscode/blob/085c409898bbc89c83409f6a394e73130b932add/src/vscode-dts/vscode.d.ts#L13715
type Cell struct {
	Kind     CellKind       `json:"kind"`
	Value    string         `json:"value"`
	LangID   string         `json:"languageId"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Notebook resembles NotebookData form VS Code.
// https://github.com/microsoft/vscode/blob/085c409898bbc89c83409f6a394e73130b932add/src/vscode-dts/vscode.d.ts#L13767
type Notebook struct {
	Cells    []*Cell        `json:"cells"`
	Metadata map[string]any `json:"metadata,omitempty"`
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
						Value: string(block.Value()),
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
								Value: string(listItemNode.Item().Value()),
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
						Value: string(block.Value()),
					})
				}
			}

		case *document.CodeBlock:
			value := block.Value()

			isListItem := node.Item() != nil && node.Item().Unwrap().Kind() == ast.KindListItem

			if childIdx == 0 && isListItem {
				listItem := node.Item().Unwrap().(*ast.ListItem)
				list := listItem.Parent().(*ast.List)
				isBulletList := list.Start == 0

				var prefix []byte

				if isBulletList {
					prefix = append(prefix, []byte{list.Marker, ' '}...)
				} else {
					itemNumber := list.Start
					tmp := child.Item().Unwrap()
					for tmp.PreviousSibling() != nil {
						tmp = tmp.PreviousSibling()
						itemNumber++
					}
					prefix = append([]byte(strconv.Itoa(itemNumber)), '.', ' ')
				}

				value = append(prefix, value...)
			}

			*cells = append(*cells, &Cell{
				Kind:   CodeKind,
				Value:  string(value),
				LangID: block.Language(),
			})

		case *document.MarkdownBlock:
			value := block.Value()

			isListItem := node.Item() != nil && node.Item().Unwrap().Kind() == ast.KindListItem

			if childIdx == 0 && isListItem {
				listItem := node.Item().Unwrap().(*ast.ListItem)
				list := listItem.Parent().(*ast.List)
				isBulletList := list.Start == 0

				var prefix []byte

				if isBulletList {
					prefix = append(prefix, []byte{list.Marker, ' '}...)
				} else {
					itemNumber := list.Start
					tmp := child.Item().Unwrap()
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
				Value: string(value),
			})
		}
	}
}

func countTrailingNewLines(b []byte) int {
	count := 0
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] == '\n' {
			count++
		} else {
			break
		}
	}
	return count
}

var (
	bulletMarkers  = []string{"- ", "* "}
	orderedMarkers = []string{". ", ") "}
)

func isBetweenListItems(cells []*Cell, idx int) (int, int, bool) {
	if idx <= 0 || idx >= len(cells)-1 {
		return -1, -1, false
	}

	prev := cells[idx-1]

	// Only patterns like "MARKER " and "NUMBER. " are checked
	// which means that the value must be at least 2 characters.
	if len(prev.Value) < 2 {
		return -1, -1, false
	}

	for i := idx + 1; i < len(cells); i++ {
		next := cells[i]
		if len(next.Value) < 2 {
			continue
		}

		// Between bullet list items
		for _, marker := range bulletMarkers {
			if prev.Value[0:2] == marker && next.Value[0:2] == marker {
				return idx - 1, i, true
			}
		}

		// Between ordered list items
		numberPrevStop, numberNextStop := -1, -1
		for _, marker := range orderedMarkers {
			prevIdx := strings.Index(prev.Value, marker)
			nextIdx := strings.Index(next.Value, marker)
			if prevIdx > -1 && nextIdx > -1 {
				numberPrevStop = prevIdx
				numberNextStop = nextIdx
				break
			}
		}
		if numberPrevStop == -1 || numberNextStop == -1 {
			continue
		}
		prevNumber, err := strconv.Atoi(prev.Value[0:numberPrevStop])
		if err != nil {
			return -1, -1, false
		}
		nextNumber, err := strconv.Atoi(next.Value[0:numberNextStop])
		if err != nil {
			continue
		}
		if nextNumber-prevNumber == 1 {
			return idx - 1, i, true
		}
	}
	return -1, -1, false
}

func isInTightList(cells []*Cell, idx int) bool {
	if idx < 0 || idx >= len(cells)-1 {
		return false
	}

	cell := cells[idx]
	next := cells[idx+1]

	// Only patterns like "MARKER " and "NUMBER. " are checked
	// which means that the value must be at least 2 characters.
	if len(cell.Value) < 2 || len(next.Value) < 2 {
		return false
	}

	// Between bullet list items
	for _, marker := range bulletMarkers {
		if cell.Value[0:2] == marker && next.Value[0:2] == marker {
			return true
		}
	}

	// Between ordered list items
	numberPrevStop, numberNextStop := -1, -1
	for _, marker := range orderedMarkers {
		prevIdx := strings.Index(cell.Value, marker)
		nextIdx := strings.Index(next.Value, marker)
		if prevIdx > -1 && nextIdx > -1 {
			numberPrevStop = prevIdx
			numberNextStop = nextIdx
			break
		}
	}
	if numberPrevStop == -1 || numberNextStop == -1 {
		return false
	}
	cellNumber, err := strconv.Atoi(cell.Value[0:numberPrevStop])
	if err != nil {
		return false
	}
	nextNumber, err := strconv.Atoi(next.Value[0:numberNextStop])
	if err != nil {
		return false
	}
	return nextNumber-cellNumber == 1
}

func serializeCells(cells []*Cell) []byte {
	var buf bytes.Buffer

	prefix := ""
	prefixReset := map[int]func(string) string{}

	for idx, cell := range cells {
		if _, stop, ok := isBetweenListItems(cells, idx); ok {
			prefix += "   "
			prefixReset[stop] = func(s string) string { return s[0 : len(s)-3] }
		}
		if fn, ok := prefixReset[idx]; ok {
			prefix = fn(prefix)
		}

		s := bufio.NewScanner(strings.NewReader(cell.Value))
		for s.Scan() {
			_, _ = buf.WriteString(prefix)
			_, _ = buf.Write(s.Bytes())
			_ = buf.WriteByte('\n')
		}

		if idx != len(cells)-1 {
			nlCount := countTrailingNewLines(buf.Bytes())
			nlRequired := 2
			if isInTightList(cells, idx) {
				nlRequired = 1
			}
			for i := nlCount; i < nlRequired; i++ {
				_ = buf.WriteByte('\n')
			}
		}
	}

	return buf.Bytes()
}
