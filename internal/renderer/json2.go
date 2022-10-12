package renderer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"strings"

	"github.com/pkg/errors"
	"github.com/yuin/goldmark/ast"
	gmrenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

type bulkWriter struct {
	io.Writer
	n   int
	err error
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
	n, err := w.Writer.Write(p)
	w.n += n
	w.err = err
}

func (w *bulkWriter) WriteByte(b byte) {
	w.Write([]byte{b})
}

type node struct {
	data *block
	next *node
}

type stack struct {
	head   *node
	len    int
	maxLen int
}

func (s *stack) Peek() *block {
	if s.head == nil {
		return nil
	}
	return s.head.data
}

func (s *stack) Push(b *block) {
	if s.head == nil {
		s.head = &node{data: b}
	} else {
		n := &node{data: b}
		n.next = s.head
		s.head = n
	}

	s.len++

	if s.len > s.maxLen {
		s.maxLen = s.len
	}
}

func (s *stack) Pop() *block {
	if s.head == nil {
		return nil
	}

	n := s.head
	s.head = n.next
	n.next = nil

	s.len--

	return n.data
}

type block struct {
	*bulkWriter
	buf   *bytes.Buffer
	inner ast.Node
}

func (b *block) IsDocument() bool {
	return b.inner.Kind() == ast.KindDocument
}

func newBlock(node ast.Node) *block {
	var buf bytes.Buffer
	return &block{
		bulkWriter: &bulkWriter{Writer: &buf},
		buf:        &buf,
		inner:      node,
	}
}

func (b *block) RenderContent(w io.Writer) error {
	_, err := b.Result()
	if err != nil {
		return errors.WithStack(err)
	}
	_, err = w.Write(b.buf.Bytes())
	return errors.WithStack(err)
}

func (b *block) RenderAsJSON(w io.Writer) error {
	data, err := json.Marshal(b)
	if err != nil {
		return errors.WithStack(err)
	}
	_, err = w.Write(data)
	return errors.WithStack(err)
}

func (b *block) MarshalJSON() ([]byte, error) {
	if _, err := b.Result(); err != nil {
		return nil, err
	}

	type markdownBlock struct {
		Markdown string `json:"markdown,omitempty"`
	}

	block := markdownBlock{
		Markdown: b.buf.String(),
	}

	return json.Marshal(block)
}

type Renderer struct {
	stack *stack
}

var _ gmrenderer.Renderer = (*Renderer)(nil)

func NewRenderer() *Renderer {
	return &Renderer{stack: &stack{}}
}

func (r *Renderer) Render(w io.Writer, source []byte, n ast.Node) error {
	writer := bufio.NewWriter(w)
	err := ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		s := ast.WalkStatus(ast.WalkContinue)
		var (
			err error
			f   gmrenderer.NodeRendererFunc
		)
		switch n.Kind() {
		case ast.KindDocument:
			f = r.renderDocument
		case ast.KindHeading:
			f = r.renderHeading
		case ast.KindBlockquote:
			f = r.renderBlockquote
		case ast.KindCodeBlock:
			f = r.renderCodeBlock
		case ast.KindFencedCodeBlock:
			f = r.renderFencedCodeBlock
		case ast.KindHTMLBlock:
			f = r.renderHTMLBlock
		case ast.KindList:
			f = r.renderList
		case ast.KindListItem:
			f = r.renderListItem
		case ast.KindParagraph:
			f = r.renderParagraph
		case ast.KindTextBlock:
			f = r.renderTextBlock
		case ast.KindThematicBreak:
			f = r.renderThematicBreak
		case ast.KindAutoLink:
			f = r.renderAutoLink
		case ast.KindCodeSpan:
			f = r.renderCodeSpan
		case ast.KindEmphasis:
			f = r.renderEmphasis
		case ast.KindImage:
			f = r.renderImage
		case ast.KindLink:
			f = r.renderLink
		case ast.KindRawHTML:
			f = r.renderRawHTML
		case ast.KindText:
			f = r.renderText
		case ast.KindString:
			f = r.renderString
		}
		if f != nil {
			s, err = f(writer, source, n, entering)
		}
		return s, err
	})
	if err != nil {
		return errors.WithStack(err)
	}
	return errors.WithStack(writer.Flush())
}

func (r *Renderer) AddOptions(...gmrenderer.Option) {
	panic("not implemented")
}

func (r *Renderer) renderDocument(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.Write([]byte(`{"document":[`))
		r.stack.Push(newBlock(node))
	} else {
		block := r.stack.Pop()
		if _, err := block.Result(); err != nil {
			return ast.WalkStop, errors.WithStack(err)
		}
		_, _ = w.Write(block.buf.Bytes())
		_, _ = w.Write([]byte(`]}`))
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderHeading(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	log.Printf("[renderHeading] entering = %t\n", entering)

	n := node.(*ast.Heading)
	if entering {
		if r.stack.Peek().IsDocument() {
			if r.stack.maxLen >= 2 {
				_, _ = w.WriteRune(',')
			}

			block := newBlock(node)
			block.Write([]byte(strings.Repeat("#", n.Level)))
			block.WriteByte(' ')
			r.stack.Push(block)
		}
	} else {
		if r.stack.Peek().inner == node {
			block := r.stack.Pop()

			if r.stack.Peek().IsDocument() {
				if err := block.RenderAsJSON(w); err != nil {
					return ast.WalkStop, errors.WithStack(err)
				}
			} else {
				if err := block.RenderContent(w); err != nil {
					return ast.WalkStop, errors.WithStack(err)
				}
			}
		}
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderBlockquote(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	log.Printf("[renderBlockquote] entering = %t\n", entering)

	if entering {
		if r.stack.Peek().IsDocument() {
			if r.stack.maxLen >= 2 {
				_, _ = w.WriteRune(',')
			}

			block := newBlock(node)
			block.Write([]byte("> "))
			r.stack.Push(block)
		}
	} else {
		if r.stack.Peek().inner == node {
			block := r.stack.Pop()

			if r.stack.Peek().IsDocument() {
				if err := block.RenderAsJSON(w); err != nil {
					return ast.WalkStop, errors.WithStack(err)
				}
			} else {
				if err := block.RenderContent(w); err != nil {
					return ast.WalkStop, errors.WithStack(err)
				}
			}
		}
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderCodeBlock(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *Renderer) renderFencedCodeBlock(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *Renderer) renderHTMLBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *Renderer) renderList(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *Renderer) renderListItem(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *Renderer) renderParagraph(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	log.Printf("[renderParagraph] entering = %t\n", entering)

	if entering {
		if r.stack.Peek().IsDocument() {
			if r.stack.maxLen >= 2 {
				_, _ = w.WriteRune(',')
			}

			block := newBlock(node)

			for i := 0; i < node.Lines().Len(); i++ {
				line := node.Lines().At(i)
				block.Write(line.Value(source))
			}

			r.stack.Push(block)
		} else {
			block := r.stack.Peek()

			for i := 0; i < node.Lines().Len(); i++ {
				line := node.Lines().At(i)
				block.Write(line.Value(source))
			}
		}
	} else {
		if r.stack.Peek().inner == node {
			block := r.stack.Pop()

			if r.stack.Peek().IsDocument() {
				if err := block.RenderAsJSON(w); err != nil {
					return ast.WalkStop, errors.WithStack(err)
				}
			} else {
				if err := block.RenderContent(w); err != nil {
					return ast.WalkStop, errors.WithStack(err)
				}
			}
		}
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderTextBlock(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *Renderer) renderThematicBreak(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *Renderer) renderAutoLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *Renderer) renderCodeSpan(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *Renderer) renderEmphasis(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	log.Printf("[renderEmphasis] entering = %t\n", entering)
	if entering {
		r.stack.Peek().Write([]byte("**"))
	} else {
		r.stack.Peek().Write([]byte("**"))
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *Renderer) renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *Renderer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *Renderer) renderText(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	log.Printf("[renderText] entering = %t\n", entering)
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Text)
	r.stack.Peek().Write(n.Segment.Value(source))
	return ast.WalkContinue, nil
}

func (r *Renderer) renderString(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	log.Printf("[renderString] entering = %t\n", entering)
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Text)
	r.stack.Peek().Write(n.Segment.Value(source))
	return ast.WalkContinue, nil
}
