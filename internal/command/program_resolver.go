package command

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type ProgramResolverMode uint8

const (
	// unspecified is auto (default) which prompts for all unresolved
	// subsequent runs will likely resolve via session
	ProgramResolverModeAuto ProgramResolverMode = iota
	// always prompt even if resolved
	ProgramResolverModePrompt
	// don't prompt whatsover even unresolved is resolved
	ProgramResolverModeSkip
)

type ProgramResolverSource func() []string

func ProgramResolverSourceFunc(env []string) ProgramResolverSource {
	return func() []string {
		return env
	}
}

// ProgramResolver uses a list of ProgramResolverSource to resolve environment variables
// found in a shell program. The result contains all found environment variables.
// If the env is in any source, it is considered resolved. Otherwise, it is marked
// as unresolved.
type ProgramResolver struct {
	mode           ProgramResolverMode
	sources        []ProgramResolverSource
	envCache       map[string]string
	printer        *syntax.Printer
	modifiedScript bool
}

func NewProgramResolver(mode ProgramResolverMode, sources ...ProgramResolverSource) *ProgramResolver {
	return &ProgramResolver{
		mode:     mode,
		sources:  sources,
		envCache: nil,
		printer:  syntax.NewPrinter(),
	}
}

type ProgramResolverPrompt uint8

const (
	ProgramResolverUnresolved ProgramResolverPrompt = iota
	ProgramResolverResolved
	ProgramResolverMessage
	ProgramResolverPlaceholder
)

type ProgramResolverResult struct {
	Prompt        ProgramResolverPrompt
	Name          string
	OriginalValue string
	Value         string
}

func (r *ProgramResolverResult) IsResolved() bool {
	return r.Prompt == ProgramResolverResolved
}

func (r *ProgramResolverResult) IsPlaceholder() bool {
	return r.Prompt == ProgramResolverPlaceholder
}

func (r *ProgramResolverResult) IsMessage() bool {
	return r.Prompt == ProgramResolverMessage
}

func (r *ProgramResolver) Resolve(reader io.Reader, writer io.Writer) ([]*ProgramResolverResult, error) {
	f, err := syntax.NewParser().Parse(reader, "")
	if err != nil {
		return nil, err
	}

	decls, err := r.walk(f)
	if err != nil {
		return nil, err
	}

	var result []*ProgramResolverResult
	for _, decl := range decls {
		if len(decl.Args) < 1 {
			continue
		}

		arg := decl.Args[0]

		name := arg.Name.Value
		originalValue, originalQuoted := r.findOriginalValue(decl)
		value, ok := r.findEnvValue(name)

		prompt := ProgramResolverUnresolved
		switch r.mode {
		case ProgramResolverModePrompt:
			// once a value is resolved, it's a placeholder
			prompt = ProgramResolverPlaceholder
		case ProgramResolverModeSkip:
			if !ok {
				value = originalValue
			}
			prompt = ProgramResolverResolved
		default:
			if ok {
				prompt = ProgramResolverResolved
				break
			}
			if originalQuoted {
				prompt = ProgramResolverPlaceholder
			} else {
				prompt = ProgramResolverMessage
			}
		}

		item := &ProgramResolverResult{
			Prompt:        prompt,
			Name:          name,
			OriginalValue: originalValue,
			Value:         value,
		}
		result = append(result, item)
	}

	slices.SortStableFunc(result, func(a, b *ProgramResolverResult) int {
		aResolved, bResolved := a.IsResolved(), b.IsResolved()
		if aResolved && bResolved {
			return strings.Compare(a.Name, b.Name)
		}
		if aResolved {
			return -1
		}
		return 1
	})

	err = syntax.NewPrinter().Print(writer, f)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (r *ProgramResolver) findOriginalValue(decl *syntax.DeclClause) (string, bool) {
	var vals []string
	quoted := false

	if len(decl.Args) < 1 || decl.Args[0].Value == nil {
		return "", quoted
	}

	naked := false
	syntax.Walk(decl, func(node syntax.Node) bool {
		switch part := node.(type) {
		// skips var name
		case *syntax.Assign:
			naked = part.Naked
			return true
		// export FOO=bar
		case *syntax.Lit:
			if !naked {
				naked = true
				return true
			}
			vals = append(vals, part.Value)
			return true
		// export FOO="bar"
		case *syntax.DblQuoted:
			if len(part.Parts) == 1 {
				p, ok := part.Parts[0].(*syntax.Lit)
				// break for quoted stmt, ie non-literal
				// export FOO="$( echo 'this is a test' )"
				if !ok {
					break
				}
				vals = append(vals, p.Value)
				quoted = true
				return false
			}
		// export FOO='bar'
		case *syntax.SglQuoted:
			vals = append(vals, part.Value)
			quoted = true
			return false
		// export FOO=${FOO:-bar}
		case *syntax.ParamExp:
			if part.Exp.Op == syntax.DefaultUnsetOrNull {
				quoted = false
				vals = append(vals, part.Exp.Word.Lit())
				return true
			}
		}
		return true
	})

	return strings.Join(vals, " "), quoted
}

func (r *ProgramResolver) findEnvValue(name string) (string, bool) {
	if r.envCache == nil {
		r.envCache = make(map[string]string)
		r.collectEnvFromSources()
	}
	val, ok := r.envCache[name]
	return val, ok
}

func (r *ProgramResolver) collectEnvFromSources() {
	for _, source := range r.sources {
		env := source()
		for _, e := range env {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				r.envCache[parts[0]] = parts[1]
			}
		}
	}
}

func (r *ProgramResolver) walk(f *syntax.File) (result []*syntax.DeclClause, err error) {
	syntax.Walk(f, func(node syntax.Node) bool {
		if err != nil {
			return false
		}

		switch x := node.(type) {
		case *syntax.File:
			return true
		case *syntax.Stmt:
			var decls []*syntax.DeclClause
			decls, err = r.resolveExportStmt(x)
			if err != nil {
				return false
			}
			result = append(result, decls...)
			return false
		default:
			// noop
		}

		// unless stmt is at top level bail
		// we don't want to go deeper
		return false
	})

	return
}

func (r *ProgramResolver) resolveExportStmt(stmt *syntax.Stmt) ([]*syntax.DeclClause, error) {
	var result []*syntax.DeclClause
	if x, ok := stmt.Cmd.(*syntax.DeclClause); ok {
		if x.Variant.Value != "export" && len(x.Args) != 1 {
			return result, nil
		}

		if r.hasExpr(x.Args[0]) {
			return result, nil
		}

		result = append(result, x)
		var exportStmt bytes.Buffer
		err := r.printer.Print(&exportStmt, x)
		if err != nil {
			return nil, err
		}
		stmt.Comments = append(stmt.Comments,
			syntax.Comment{Text: "\n"},
			syntax.Comment{Text: fmt.Sprintf(" %s set in smart env store", x.Args[0].Name.Value)},
			syntax.Comment{Text: fmt.Sprintf(" %q", exportStmt.String())},
		)
		stmt.Cmd = nil
		r.modifiedScript = true
	}

	return result, nil
}

// walk AST to check for expression nodes
func (r *ProgramResolver) hasExpr(node syntax.Node) bool {
	hasSubshell := false
	syntax.Walk(node, func(x syntax.Node) bool {
		switch x.(type) {
		case *syntax.CmdSubst:
			hasSubshell = true
			return false
		case *syntax.Subshell:
			hasSubshell = true
			return false
		case *syntax.ParamExp:
			hasSubshell = true
			return false
		case *syntax.ArithmExp:
			hasSubshell = true
			return false
		case *syntax.ProcSubst:
			hasSubshell = true
			return false
		case *syntax.ExtGlob:
			hasSubshell = true
			return false
		case *syntax.BraceExp:
			hasSubshell = true
			return false
		}
		return !hasSubshell
	})
	return hasSubshell
}
