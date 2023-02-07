package row

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/padding"
	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"
)

const (
	colWidthP byte = iota + 1 // percentage
	colWidthS                 // static
)

type colWidth struct {
	t byte
	v float64
}

func (w colWidth) Val(totalWidth int) int {
	switch w.t {
	case colWidthP:
		return int(math.Round(float64(totalWidth) * w.ValP()))
	case colWidthS:
		return w.ValS()
	default:
		return 0
	}
}

func (w colWidth) ValP() float64 {
	return w.v
}

func (w colWidth) ValS() int {
	return int(w.v)
}

type Option func(*Definition)

func WithPctColumns(columns []float64) Option {
	return func(d *Definition) {
		colWidths := make([]colWidth, 0, len(columns))
		for _, c := range columns {
			colWidths = append(colWidths, colWidth{t: colWidthP, v: c})
		}
		d.colWidths = colWidths
	}
}

func WithFixedColumns(columns []int) Option {
	return func(d *Definition) {
		colWidths := make([]colWidth, 0, len(columns))
		for _, c := range columns {
			colWidths = append(colWidths, colWidth{t: colWidthS, v: float64(c)})
		}
		d.colWidths = colWidths
	}
}

func WithColumnStyles(styles []lipgloss.Style) Option {
	return func(d *Definition) {
		d.colStyles = styles
	}
}

type Definition struct {
	colWidths []colWidth
	colStyles []lipgloss.Style
	width     int
}

func NewDefinition(width int, opts ...Option) Definition {
	d := Definition{
		width: width,
	}

	for _, opt := range opts {
		opt(&d)
	}

	return d
}

func (m Definition) Width() int {
	return m.width
}

func (m *Definition) SetWidth(width int) {
	m.width = width
}

func (m Definition) ColWidth(idx int) int {
	return m.colWidths[idx].Val(m.width)
}

func (m Definition) ColWidths() []int {
	items := make([]int, 0, len(m.colWidths))
	for _, colWidth := range m.colWidths {
		items = append(items, colWidth.Val(m.width))
	}
	return items
}

func (m Definition) Style(idx int) lipgloss.Style {
	s := lipgloss.NewStyle()

	if len(m.colStyles)-1 >= idx {
		s = m.colStyles[idx].Copy()
	}

	return s.Width(m.ColWidth(idx))
}

func String(d Definition, data []string) string {
	table := make([][]string, len(data)) // col -> rows
	maxRows := 0

	for colIdx, str := range data {
		colWidth := d.ColWidth(colIdx)
		str = wrap.String(wordwrap.String(str, colWidth), colWidth)
		table[colIdx] = strings.Split(str, "\n")
		if l := len(table[colIdx]); maxRows < l {
			maxRows = l
		}
	}

	var b strings.Builder

	for rowIdx := 0; rowIdx < maxRows; rowIdx++ {
		for colIdx := 0; colIdx < len(table); colIdx++ {
			col := table[colIdx]

			// The second condition is due to padding.String()
			// not handling empty strings as needed.
			if rowIdx < len(col) && col[rowIdx] != "" {
				b.WriteString(padding.String(d.Style(colIdx).Render(col[rowIdx]), uint(d.ColWidth(colIdx))))
			} else {
				b.WriteString(strings.Repeat(" ", d.ColWidth(colIdx)))
			}
		}

		if rowIdx < maxRows-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}
