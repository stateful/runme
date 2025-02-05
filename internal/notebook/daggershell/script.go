package daggershell

import (
	"errors"
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
		stmts: []*syntax.Stmt{},
		printer: syntax.NewPrinter(
			syntax.Indent(2),
			syntax.BinaryNextLine(true),
			syntax.FunctionNextLine(true),
		),
		parser: syntax.NewParser(),
	}
}

func (s *Script) DeclareFunc(name, body string) error {
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

		// todo(sebastian): check for validity, e.g. func def inside itself
		return true
	})

	s.stmts = append(s.stmts, stmt)

	return nil
}

func (s *Script) Render(w io.Writer) error {
	return s.RenderWithCall(w, "")
}

func (s *Script) RenderWithCall(w io.Writer, name string) error {
	if name == "" {
		return s.printer.Print(w, &syntax.File{
			Name:  "DaggerShellScript",
			Stmts: s.stmts,
		})
	}

	stmts := make([]*syntax.Stmt, len(s.stmts))
	copy(stmts, s.stmts)
	f := &syntax.File{
		Name:  "DaggerShellScript",
		Stmts: stmts,
	}

	validFuncName := false
	// check if func name was previously declared
	syntax.Walk(f, func(node syntax.Node) bool {
		decl, ok := node.(*syntax.FuncDecl)
		if !ok {
			return true
		}

		if decl.Name.Value == name {
			validFuncName = true
			return false
		}

		return true
	})

	if !validFuncName {
		return errors.New("undeclared function name")
	}

	f.Stmts = append(f.Stmts, &syntax.Stmt{
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

	return s.printer.Print(w, f)
}
