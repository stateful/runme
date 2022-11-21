package md

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/yuin/goldmark/ast"
)

func Render(doc ast.Node, source []byte) ([]byte, error) {
	return new(renderer).render(doc, source)
}

type renderer struct {
	noLinebreaks bool
	beginContent bool
	beginLine    bool
	needCR       int
	prefix       string
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

func (r *renderer) out(w bulkWriter, data []byte) error {
	k := len(w.Bytes()) - 1

	for r.needCR > 0 {
		if k < 0 || w.buf.Bytes()[k] == '\n' {
			k--
		} else {
			w.WriteByte('\n')
			if r.needCR > 1 {
				w.Write([]byte(r.prefix))
			}
		}

		r.beginContent = true
		r.beginLine = true
		r.needCR--
	}

	for _, c := range string(data) {
		if r.beginLine {
			w.Write([]byte(r.prefix))
		}

		if c == '\n' {
			w.WriteByte('\n')
			r.beginContent = true
			r.beginLine = true
		} else {
			w.WriteRune(c)
			r.beginLine = false
			r.beginContent = false
		}
	}

	return w.Err()
}

func (r *renderer) render(doc ast.Node, source []byte) ([]byte, error) {
	var buf bytes.Buffer

	err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		w := bulkWriter{buf: &buf}

		switch node.Kind() {
		// blocks
		case ast.KindDocument:
			if !entering {
				r.needCR = 1
				if err := r.out(w, nil); err != nil {
					return ast.WalkStop, err
				}
			}
		case ast.KindHeading:
			n := node.(*ast.Heading)
			if entering {
				err := r.out(w, bytes.Repeat([]byte{'#'}, n.Level))
				if err != nil {
					return ast.WalkStop, err
				}
				w.WriteByte(' ')
				r.noLinebreaks = true
				r.beginContent = true
			} else {
				r.noLinebreaks = false
				r.blankline()
			}
		case ast.KindBlockquote:
			if entering {
				err := r.out(w, []byte("> "))
				if err != nil {
					return ast.WalkStop, err
				}
				r.beginContent = true
				r.prefix = "> "
			} else {
				r.prefix = r.prefix[0 : len(r.prefix)-2]
				r.blankline()
			}
		case ast.KindCodeBlock, ast.KindFencedCodeBlock:
			// TODO: fix it and follow CMARK_NODE_CODE

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

				err := r.out(w, bytes.Repeat([]byte{'`'}, ticksCount))
				if err != nil {
					return ast.WalkStop, err
				}

				if n, ok := node.(*ast.FencedCodeBlock); ok && n.Info != nil {
					info := n.Info.Segment.Value(source)
					err := r.out(w, info)
					if err != nil {
						return ast.WalkStop, err
					}
				}

				r.cr()

				if err := r.out(w, code.Bytes()); err != nil {
					return ast.WalkStop, err
				}
			} else {
				r.cr()
				err := r.out(w, bytes.Repeat([]byte{'`'}, ticksCount))
				if err != nil {
					return ast.WalkStop, err
				}
				r.blankline()
			}

			return ast.WalkContinue, nil

		case ast.KindHTMLBlock:
			if !entering {
				break
			}
			r.blankline()
			err := r.out(w, node.Text(source))
			if err != nil {
				return ast.WalkStop, err
			}
			r.blankline()

			return ast.WalkSkipChildren, nil

		case ast.KindList:
			if !entering && node.NextSibling() != nil && node.NextSibling().Kind() == ast.KindList {
				r.cr()
				err := r.out(w, []byte("<!-- end list -->"))
				if err != nil {
					return ast.WalkStop, err
				}
				r.blankline()
			}
		case ast.KindListItem:
			listNode := node.Parent().(*ast.List)
			isBulletList := listNode.Start == 0

			if entering {
				if isBulletList {
					err := r.out(w, []byte("  - "))
					if err != nil {
						return ast.WalkStop, err
					}
					r.beginContent = true
				} else {
					itemNumber := listNode.Start
					tmp := node
					for tmp.PreviousSibling() != nil {
						tmp = tmp.PreviousSibling()
						itemNumber++
					}
					err := r.out(w, []byte(strconv.Itoa(itemNumber)))
					if err != nil {
						return ast.WalkStop, err
					}
					err = r.out(w, []byte(". "))
					if err != nil {
						return ast.WalkStop, err
					}
					r.beginContent = true
				}
				r.prefix += "    "
			} else {
				r.prefix = r.prefix[0 : len(r.prefix)-4]
			}

		case ast.KindParagraph:
			if !entering {
				r.blankline()
			}

		case ast.KindTextBlock:
			if !entering {
				r.blankline()
			}

		case ast.KindThematicBreak:
			if entering {
				r.blankline()
				if err := r.out(w, []byte("-----")); err != nil {
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

				if err := r.out(w, []byte(b.String())); err != nil {
					return ast.WalkStop, err
				}
			}

		case ast.KindCodeSpan:
			if err := r.out(w, []byte("`")); err != nil {
				return ast.WalkStop, err
			}
		case ast.KindEmphasis:
			n := node.(*ast.Emphasis)
			mark := "*"
			if n.Level > 1 {
				mark = "**"
			}
			if err := r.out(w, []byte(mark)); err != nil {
				return ast.WalkStop, err
			}
		case ast.KindImage:
			if entering {
				if err := r.out(w, []byte("![")); err != nil {
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

				if err := r.out(w, []byte(b.String())); err != nil {
					return ast.WalkStop, err
				}
			}
		case ast.KindLink:
			if entering {
				if err := r.out(w, []byte{'['}); err != nil {
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

				if err := r.out(w, []byte(b.String())); err != nil {
					return ast.WalkStop, err
				}
			}
		case ast.KindRawHTML:
			if entering {
				if err := r.out(w, node.Text(source)); err != nil {
					return ast.WalkStop, err
				}
				return ast.WalkSkipChildren, nil
			}
		case ast.KindText:
			if entering {
				if err := r.out(w, node.Text(source)); err != nil {
					return ast.WalkStop, err
				}
			}
		case ast.KindString:
			if entering {
				if err := r.out(w, node.Text(source)); err != nil {
					return ast.WalkStop, err
				}
			}
		}

		return ast.WalkContinue, nil
	})

	return buf.Bytes(), errors.WithStack(err)
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

type bulkWriter struct {
	buf *bytes.Buffer
	n   int
	err error
}

func (w *bulkWriter) Bytes() []byte {
	return w.buf.Bytes()
}

func (w *bulkWriter) Err() error {
	return w.err
}

func (w *bulkWriter) Result() (int, error) {
	return w.n, w.err
}

func (w *bulkWriter) Write(p []byte) {
	if w.err != nil {
		return
	}
	n, err := w.buf.Write(p)
	w.n += n
	w.err = err
}

func (w *bulkWriter) WriteByte(b byte) {
	if w.err != nil {
		return
	}
	w.Write([]byte{b})
}

func (w *bulkWriter) WriteRune(r rune) {
	if w.err != nil {
		return
	}
	n, err := w.buf.WriteRune(r)
	w.n += n
	w.err = err
}
