package notebook

import (
	"io"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type Script struct {
	stmts   []*syntax.Stmt
	printer *syntax.Printer
	parser  *syntax.Parser
}

func NewScript() *Script {
	return &Script{
		stmts:   []*syntax.Stmt{},
		printer: syntax.NewPrinter(syntax.SingleLine(true)),
		parser:  syntax.NewParser(),
	}
}

func (s *Script) declareFunc(name, body string) error {
	stmt := &syntax.Stmt{
		Cmd: &syntax.FuncDecl{
			Parens: true,
			Name: &syntax.Lit{
				Value: name,
			},
			Body: &syntax.Stmt{
				Cmd: &syntax.Block{},
			},
		},
	}

	snippet, err := s.parser.Parse(strings.NewReader(body), name)
	if err != nil {
		return err
	}

	// assign stmts to insert function body
	syntax.Walk(stmt, func(node syntax.Node) bool {
		if block, ok := node.(*syntax.Block); ok {
			block.Stmts = snippet.Stmts
			return false
		}

		return true
	})

	s.stmts = append(s.stmts, stmt)

	return nil
}

func (s *Script) Render(w io.Writer) error {
	return s.RenderWithCall(w, "")
}

func (s *Script) RenderWithCall(w io.Writer, name string) error {
	stmts := s.stmts

	if name != "" {
		stmts = make([]*syntax.Stmt, len(s.stmts))
		copy(stmts, s.stmts)
		stmts = append(stmts, &syntax.Stmt{
			Cmd: &syntax.CallExpr{
				Args: []*syntax.Word{
					{
						Parts: []syntax.WordPart{
							&syntax.Lit{Value: name},
						},
					},
				},
			},
		})
	}

	return s.printer.Print(w, &syntax.File{
		Name:  "DaggerShellScript",
		Stmts: stmts,
	})
}
