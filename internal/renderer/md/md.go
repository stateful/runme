package md

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/yuin/goldmark/ast"
)

type NodeSourceProvider func(ast.Node) ([]byte, bool)

func Render(doc ast.Node, source []byte) ([]byte, error) {
	return new(Renderer).Render(
		doc,
		source,
		func(ast.Node) ([]byte, bool) { return nil, false },
		func(ast.Node, bool) (ast.WalkStatus, bool) { return ast.WalkContinue, false },
	)
}

func RenderWithSourceProvider(
	doc ast.Node,
	source []byte,
	provider NodeSourceProvider,
) ([]byte, error) {
	return new(Renderer).Render(
		doc,
		source,
		provider,
		func(ast.Node, bool) (ast.WalkStatus, bool) { return ast.WalkContinue, false },
	)
}

type Renderer struct {
	beginLine       bool
	buf             bytes.Buffer
	inTightListItem bool
	needCR          int
	prefix          string
}

func (r *Renderer) blankline() {
	if r.needCR < 2 {
		r.needCR = 2
	}
}

func (r *Renderer) cr() {
	if r.needCR < 1 {
		r.needCR = 1
	}
}

func (r *Renderer) write(data []byte) error {
	k := len(r.buf.Bytes()) - 1

	for r.needCR > 0 {
		if k < 0 || r.buf.Bytes()[k] == '\n' {
			k--
			if r.beginLine && r.needCR > 1 {
				if _, err := r.buf.Write(bytes.TrimRight([]byte(r.prefix), " ")); err != nil {
					return err
				}
			}
		} else {
			if err := r.buf.WriteByte('\n'); err != nil {
				return err
			}
			if r.needCR > 1 {
				if _, err := r.buf.Write(bytes.TrimRight([]byte(r.prefix), " ")); err != nil {
					return err
				}
			}
		}

		r.beginLine = true
		r.needCR--
	}

	for _, c := range string(data) {
		if r.beginLine {
			if _, err := r.buf.Write([]byte(r.prefix)); err != nil {
				return err
			}
		}

		var err error
		if c == '\n' {
			err = r.buf.WriteByte('\n')
			r.beginLine = true
		} else {
			_, err = r.buf.WriteRune(c)
			r.beginLine = false
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Renderer) BufferBytes() ([]byte, error) {
	var buf bytes.Buffer
	if _, err := r.buf.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r *Renderer) RawBufferBytes() ([]byte, error) {
	var buf bytes.Buffer
	if _, err := r.buf.WriteTo(&buf); err != nil {
		return nil, err
	}
	return bytes.TrimPrefix(buf.Bytes(), []byte(r.prefix)), nil
}

func (r *Renderer) Render(
	doc ast.Node,
	source []byte,
	nodeSourceProvider NodeSourceProvider,
	callback func(ast.Node, bool) (_ ast.WalkStatus, skip bool),
) ([]byte, error) {
	err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		status, skip := callback(node, entering)
		if skip {
			return status, nil
		}

		switch node.Kind() {
		// blocks
		case ast.KindDocument:

		case ast.KindHeading:
			if entering {
				value, ok := nodeSourceProvider(node)
				if !ok {
					n := node.(*ast.Heading)
					value = bytes.Repeat([]byte{'#'}, n.Level)
					value = append(value, ' ')
				} else {
					status = ast.WalkSkipChildren
				}

				err := r.write(value)
				if err != nil {
					return ast.WalkStop, err
				}
			} else {
				r.blankline()
			}

		case ast.KindBlockquote:
			if entering {
				err := r.write([]byte("> "))
				if err != nil {
					return ast.WalkStop, err
				}
				r.prefix += "> "
			} else {
				r.prefix = r.prefix[0 : len(r.prefix)-2]
				r.blankline()
			}

		case ast.KindCodeBlock:
			value, ok := nodeSourceProvider(node)
			if ok {
				if entering {
					if err := r.write(value); err != nil {
						return ast.WalkStop, err
					}
				} else {
					r.blankline()
				}
				return ast.WalkSkipChildren, nil
			}

			var code bytes.Buffer
			for ll, i := node.Lines().Len(), 0; i < ll; i++ {
				line := node.Lines().At(i)
				_, _ = code.Write(line.Value(source))
			}

			if entering {
				firstInListItem := node.PreviousSibling() == nil && node.Parent() != nil && node.Parent().Kind() == ast.KindListItem
				if !firstInListItem {
					r.blankline()
				}

				r.prefix += "    "

				if err := r.write(code.Bytes()); err != nil {
					return ast.WalkStop, err
				}
			} else {
				r.prefix = r.prefix[0 : len(r.prefix)-4]
				r.blankline()
			}

		case ast.KindFencedCodeBlock:
			value, ok := nodeSourceProvider(node)
			if ok {
				if entering {
					if err := r.write(value); err != nil {
						return ast.WalkStop, err
					}
				} else {
					r.blankline()
				}
				return ast.WalkSkipChildren, nil
			}

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
			value, ok := nodeSourceProvider(node)
			if ok {
				if entering {
					if err := r.write(value); err != nil {
						return ast.WalkStop, err
					}
				} else {
					r.blankline()
				}
				return ast.WalkSkipChildren, nil
			}

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
			if !entering && node.NextSibling() != nil && node.NextSibling().Kind() == ast.KindList {
				r.cr()
				err := r.write([]byte("<!-- end list -->"))
				if err != nil {
					return ast.WalkStop, err
				}
				r.blankline()
			}

		case ast.KindListItem:
			listNode := node.Parent().(*ast.List)
			isBulletList := listNode.Start == 0

			if entering {
				r.inTightListItem = node.ChildCount() == 1

				if isBulletList {
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
				r.prefix += "   "
			} else {
				r.inTightListItem = false
				r.prefix = r.prefix[0 : len(r.prefix)-3]
			}

		case ast.KindParagraph:
			if entering {
				value, ok := nodeSourceProvider(node)
				if ok {
					status = ast.WalkSkipChildren
					if err := r.write(value); err != nil {
						return ast.WalkStop, err
					}
				}
			} else {
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
			value, ok := nodeSourceProvider(node)
			if ok {
				if entering {
					r.blankline()
					if err := r.write(value); err != nil {
						return ast.WalkStop, err
					}
					r.blankline()
				}
				return ast.WalkSkipChildren, nil
			}

			if entering {
				r.blankline()
				if err := r.write([]byte("---")); err != nil {
					return ast.WalkStop, err
				}
				r.blankline()
			}

		// inline
		case ast.KindAutoLink:
			value, ok := nodeSourceProvider(node)
			if ok {
				if entering {
					if err := r.write(value); err != nil {
						return ast.WalkStop, err
					}
				}
				return ast.WalkSkipChildren, nil
			}

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
			value, ok := nodeSourceProvider(node)
			if ok {
				if entering {
					if err := r.write(value); err != nil {
						return ast.WalkStop, err
					}
				}
				return ast.WalkSkipChildren, nil
			}

			if err := r.write([]byte("`")); err != nil {
				return ast.WalkStop, err
			}

		case ast.KindEmphasis:
			value, ok := nodeSourceProvider(node)
			if ok {
				if entering {
					if err := r.write(value); err != nil {
						return ast.WalkStop, err
					}
				}
				return ast.WalkSkipChildren, nil
			}

			n := node.(*ast.Emphasis)
			mark := "*"
			if n.Level > 1 {
				mark = "**"
			}
			if err := r.write([]byte(mark)); err != nil {
				return ast.WalkStop, err
			}

		case ast.KindImage:
			value, ok := nodeSourceProvider(node)
			if ok {
				if entering {
					if err := r.write(value); err != nil {
						return ast.WalkStop, err
					}
				}
				return ast.WalkSkipChildren, nil
			}

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
			value, ok := nodeSourceProvider(node)
			if ok {
				if entering {
					if err := r.write(value); err != nil {
						return ast.WalkStop, err
					}
				}
				return ast.WalkSkipChildren, nil
			}

			if entering {
				if err := r.write([]byte{'['}); err != nil {
					return ast.WalkStop, err
				}
			} else {
				n := node.(*ast.Link)

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

		case ast.KindRawHTML:
			value, ok := nodeSourceProvider(node)
			if ok {
				if entering {
					if err := r.write(value); err != nil {
						return ast.WalkStop, err
					}
				}
				return ast.WalkSkipChildren, nil
			}

			if entering {
				if err := r.write(node.Text(source)); err != nil {
					return ast.WalkStop, err
				}
				return ast.WalkSkipChildren, nil
			}

		case ast.KindText:
			value, ok := nodeSourceProvider(node)
			if ok {
				if entering {
					if err := r.write(value); err != nil {
						return ast.WalkStop, err
					}
				}
				return ast.WalkSkipChildren, nil
			}

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
			value, ok := nodeSourceProvider(node)
			if ok {
				if entering {
					if err := r.write(value); err != nil {
						return ast.WalkStop, err
					}
				}
				return ast.WalkSkipChildren, nil
			}

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

	// Finish writing any outstanding characters.
	r.needCR = 1
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
