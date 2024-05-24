package cmark

import (
	"bytes"
	"strconv"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"github.com/yuin/goldmark/ast"

	"github.com/stateful/runme/v3/pkg/document/constants"
)

type NodeSourceProvider func(ast.Node) ([]byte, bool)

func Render(doc ast.Node, source []byte) ([]byte, error) {
	lineBreak := []byte{'\n'}

	crlfCount := bytes.Count(source, []byte{'\r', '\n'})
	lfCount := bytes.Count(source, []byte{'\n'})
	if crlfCount == lfCount {
		lineBreak = []byte{'\r', '\n'}
	}

	finalLineBreaks := 1
	if flb, ok := doc.AttributeString(constants.FinalLineBreaksKey); ok {
		val, ok := flb.(int)
		if !ok {
			return nil, errors.Errorf("invalid type for %s expected int", constants.FinalLineBreaksKey)
		}
		finalLineBreaks = val
	}

	r := renderer{
		lineBreak:       lineBreak,
		finalLineBreaks: finalLineBreaks,
	}
	return r.Render(doc, source)
}

type renderer struct {
	lineBreak []byte

	beginLine       bool
	buf             bytes.Buffer
	inTightListItem bool
	finalLineBreaks int
	needCR          int
	prefix          []byte
}

func (r *renderer) blankline() {
	if r.needCR < 2 {
		r.needCR = 2
	}
}

func (r *renderer) cr() {
	if r.needCR < 1 {
		r.needCR = 1
	}
}

func (r *renderer) write(data []byte) error {
	k := len(r.buf.Bytes()) - 1

	for r.needCR > 0 {
		if k < 0 || r.buf.Bytes()[k] == '\n' {
			k--
			if r.beginLine && r.needCR > 1 {
				prefix := bytes.TrimFunc(r.prefix, unicode.IsSpace)
				if _, err := r.buf.Write(prefix); err != nil {
					return err
				}
			}
		} else {
			if _, err := r.buf.Write(r.lineBreak); err != nil {
				return err
			}
			if r.needCR > 1 {
				prefix := bytes.TrimFunc(r.prefix, unicode.IsSpace)
				if _, err := r.buf.Write(prefix); err != nil {
					return err
				}
			}
		}

		r.beginLine = true
		r.needCR--
	}

	for _, c := range data {
		if r.beginLine {
			if _, err := r.buf.Write(r.prefix); err != nil {
				return err
			}
		}

		if err := r.buf.WriteByte(c); err != nil {
			return err
		}

		r.beginLine = c == '\n'
	}

	return nil
}

func (r *renderer) BufferBytes() ([]byte, error) {
	var buf bytes.Buffer
	if _, err := r.buf.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r *renderer) RawBufferBytes() ([]byte, error) {
	var buf bytes.Buffer
	if _, err := r.buf.WriteTo(&buf); err != nil {
		return nil, err
	}
	return bytes.TrimPrefix(buf.Bytes(), r.prefix), nil
}

func (r *renderer) Render(doc ast.Node, source []byte) ([]byte, error) {
	err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		status := ast.WalkContinue

		switch node.Kind() {
		// blocks
		case ast.KindDocument:

		case ast.KindHeading:
			if entering {
				n := node.(*ast.Heading)
				value := append(bytes.Repeat([]byte{'#'}, n.Level), ' ')
				err := r.write(value)
				if err != nil {
					return ast.WalkStop, err
				}
			} else {
				r.blankline()
			}

		case ast.KindBlockquote:
			prefix := []byte{'>', ' '}
			if entering {
				if err := r.write(prefix); err != nil {
					return ast.WalkStop, err
				}
				r.prefix = append(r.prefix, prefix...)
			} else {
				r.prefix = r.prefix[0 : len(r.prefix)-len(prefix)]
				r.blankline()
			}

		case ast.KindCodeBlock:
			var code bytes.Buffer
			for ll, i := node.Lines().Len(), 0; i < ll; i++ {
				line := node.Lines().At(i)
				_, _ = code.Write(line.Value(source))
			}

			prefix := []byte{' ', ' ', ' ', ' '}

			if entering {
				firstInListItem := node.PreviousSibling() == nil && node.Parent() != nil && node.Parent().Kind() == ast.KindListItem
				if !firstInListItem {
					r.blankline()
				}

				r.prefix = append(r.prefix, prefix...)

				if err := r.write(code.Bytes()); err != nil {
					return ast.WalkStop, err
				}
			} else {
				r.prefix = r.prefix[0 : len(r.prefix)-len(prefix)]
				r.blankline()
			}

		case ast.KindFencedCodeBlock:
			var code bytes.Buffer
			for ll, i := node.Lines().Len(), 0; i < ll; i++ {
				line := node.Lines().At(i)
				_, _ = code.Write(line.Value(source))
			}

			ticksCount := longestBacktickSeq(code.Bytes())
			if ticksCount < 3 {
				ticksCount = 3
			}

			if entering {
				firstInListItem := node.PreviousSibling() == nil && node.Parent() != nil && node.Parent().Kind() == ast.KindListItem
				if !firstInListItem {
					r.blankline()
				}

				err := r.write(bytes.Repeat([]byte{'`'}, ticksCount))
				if err != nil {
					return ast.WalkStop, err
				}

				if n, ok := node.(*ast.FencedCodeBlock); ok && n.Info != nil {
					info := n.Info.Segment.Value(source)
					err := r.write(info)
					if err != nil {
						return ast.WalkStop, err
					}
				}

				r.cr()

				if err := r.write(code.Bytes()); err != nil {
					return ast.WalkStop, err
				}
			} else {
				r.cr()
				err := r.write(bytes.Repeat([]byte{'`'}, ticksCount))
				if err != nil {
					return ast.WalkStop, err
				}
				r.blankline()
			}

		case ast.KindHTMLBlock:
			if !entering {
				break
			}
			r.blankline()
			for i := 0; i < node.Lines().Len(); i++ {
				line := node.Lines().At(i)
				err := r.write(line.Value(source))
				if err != nil {
					return ast.WalkStop, err
				}
			}
			r.blankline()

		case ast.KindList:
			if !entering {
				r.blankline()
			}

		case ast.KindListItem:
			listNode := node.Parent().(*ast.List)

			if entering {
				// Some tight list items have TextBlock as an only child and others
				// have Paragraph.
				r.inTightListItem = node.ChildCount() == 1

				if !listNode.IsOrdered() {
					err := r.write([]byte{listNode.Marker, ' '})
					if err != nil {
						return ast.WalkStop, err
					}
				} else {
					itemNumber := listNode.Start
					tmp := node
					for tmp.PreviousSibling() != nil {
						tmp = tmp.PreviousSibling()
						itemNumber++
					}
					err := r.write([]byte(strconv.Itoa(itemNumber)))
					if err != nil {
						return ast.WalkStop, err
					}
					err = r.write([]byte(". "))
					if err != nil {
						return ast.WalkStop, err
					}
				}
				r.prefix = append(r.prefix, []byte{' ', ' ', ' '}...)
			} else {
				r.inTightListItem = false
				r.prefix = r.prefix[0 : len(r.prefix)-3]
			}

		case ast.KindParagraph:
			if !entering {
				if r.inTightListItem {
					r.cr()
				} else {
					r.blankline()
				}
			}

		case ast.KindTextBlock:
			if !entering {
				r.cr()
			}

		case ast.KindThematicBreak:
			if entering {
				r.blankline()
				if err := r.write([]byte("---")); err != nil {
					return ast.WalkStop, err
				}
				r.blankline()
			}

		// inline
		case ast.KindAutoLink:
			n := node.(*ast.AutoLink)
			if entering {
				var b strings.Builder
				switch n.AutoLinkType {
				case ast.AutoLinkEmail:
					_, _ = b.WriteString("<mailto:")
				case ast.AutoLinkURL:
					_, _ = b.WriteString("<")
				}
				_, _ = b.WriteString(string(n.URL(source)))
				_, _ = b.WriteString(">")

				if err := r.write([]byte(b.String())); err != nil {
					return ast.WalkStop, err
				}
			}

		case ast.KindCodeSpan:
			if err := r.write([]byte{'`'}); err != nil {
				return ast.WalkStop, err
			}

		case ast.KindEmphasis:
			var p ast.Node
			for p = node.Parent(); p.Type() == ast.TypeInline; {
				p = p.Parent()
			}
			underscores := 0
			for i := 0; i < p.Lines().Len(); i++ {
				line := p.Lines().At(i)
				underscores += bytes.Count(line.Value(source), []byte{'_'})
			}
			mark := []byte{'_'}
			if underscores == 0 {
				mark = []byte{'*'}
			}
			n := node.(*ast.Emphasis)
			mark = bytes.Repeat(mark, n.Level)
			if err := r.write(mark); err != nil {
				return ast.WalkStop, err
			}

		case ast.KindImage:
			if entering {
				if err := r.write([]byte("![")); err != nil {
					return ast.WalkStop, err
				}
			} else {
				n := node.(*ast.Image)

				var b strings.Builder
				_, _ = b.WriteString("](")
				_, _ = b.Write(n.Destination)
				if title := n.Title; len(title) > 0 {
					_, _ = b.WriteString(` "`)
					_, _ = b.Write(title)
					_, _ = b.WriteString(`"`)
				}
				_, _ = b.WriteString(")")

				if err := r.write([]byte(b.String())); err != nil {
					return ast.WalkStop, err
				}
			}

		case ast.KindLink:
			if entering {
				if err := r.write([]byte{'['}); err != nil {
					return ast.WalkStop, err
				}
			} else {
				n := node.(*ast.Link)

				var buf bytes.Buffer
				_, _ = buf.Write([]byte{']', '('})
				_, _ = buf.Write(n.Destination)
				if title := n.Title; len(title) > 0 {
					_, _ = buf.Write([]byte{' ', '"'})
					_, _ = buf.Write(title)
					_ = buf.WriteByte('"')
				}
				_ = buf.WriteByte(')')

				if err := r.write(buf.Bytes()); err != nil {
					return ast.WalkStop, err
				}
			}

		case ast.KindRawHTML:
			if entering {
				if err := r.write(node.Text(source)); err != nil {
					return ast.WalkStop, err
				}
				return ast.WalkSkipChildren, nil
			}

		case ast.KindText:
			if entering {
				if err := r.write(node.Text(source)); err != nil {
					return ast.WalkStop, err
				}
				n := node.(*ast.Text)
				if n.SoftLineBreak() {
					r.cr()
				} else if n.HardLineBreak() {
					if err := r.write([]byte("  ")); err != nil {
						return ast.WalkStop, err
					}
					r.cr()
				}
			}

		case ast.KindString:
			if entering {
				if err := r.write(node.Text(source)); err != nil {
					return ast.WalkStop, err
				}
			}
		}

		return status, nil
	})
	if err != nil {
		return r.buf.Bytes(), errors.WithStack(err)
	}

	// Finish writing any remaining characters.
	r.needCR = r.finalLineBreaks
	err = r.write(nil)
	return r.buf.Bytes(), errors.WithStack(err)
}

func longestBacktickSeq(data []byte) int {
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
