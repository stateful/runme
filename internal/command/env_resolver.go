package command

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type EnvResolverMode uint8

const (
	// automatically decide whether to prompt or not
	EnvResolverModeAuto EnvResolverMode = iota
	// always prompt
	EnvResolverModePrompt
	// never prompt
	EnvResolverModeSkip
)

type EnvResolverSource func() []string

func EnvResolverSourceFunc(env []string) EnvResolverSource {
	return func() []string {
		return env
	}
}

// EnvResolver uses a list of EnvResolverSource to resolve environment variables
// found in a shell program. The result contains all found environment variables.
// If the env is in any source, it is considered resolved. Otherwise, it is marked
// as unresolved.
type EnvResolver struct {
	mode           EnvResolverMode
	sources        []EnvResolverSource
	envCache       map[string]string
	printer        *syntax.Printer
	modifiedScript bool
}

func NewEnvResolver(mode EnvResolverMode, sources ...EnvResolverSource) *EnvResolver {
	return &EnvResolver{
		mode:     mode,
		sources:  sources,
		envCache: nil,
		printer:  syntax.NewPrinter(),
	}
}

type EnvResolverResult struct {
	Name          string
	OriginalValue string
	Placeholder   string
	Message       string
	Value         string
}

func (r *EnvResolverResult) IsResolved() bool {
	return r.Value != ""
}

func (r *EnvResolverResult) IsPlaceholder() bool {
	return r.Placeholder != ""
}

func (r *EnvResolverResult) IsMessage() bool {
	return r.Message != ""
}

func (r *EnvResolver) Resolve(reader io.Reader, writer io.Writer) ([]*EnvResolverResult, error) {
	f, err := syntax.NewParser().Parse(reader, "")
	if err != nil {
		return nil, err
	}

	decls, err := r.walk(f)
	if err != nil {
		return nil, err
	}

	var result []*EnvResolverResult
	for _, decl := range decls {
		if len(decl.Args) < 1 {
			continue
		}

		arg := decl.Args[0]

		name := arg.Name.Value
		originalValue, originalQuoted := r.findOriginalValue(decl)
		value, _ := r.findEnvValue(name)

		item := &EnvResolverResult{
			Name:          name,
			OriginalValue: originalValue,
			Value:         value,
		}
		if originalQuoted {
			item.Placeholder = originalValue
		} else {
			item.Message = originalValue
		}
		result = append(result, item)
	}

	slices.SortStableFunc(result, func(a, b *EnvResolverResult) int {
		aResolved, bResolved := a.IsResolved(), b.IsResolved()
		if aResolved && bResolved {
			return strings.Compare(a.Name, b.Name)
		}
		if aResolved {
			return -1
		}
		return 1
	})

	syntax.NewPrinter().Print(writer, f)

	return result, nil
}

func (r *EnvResolver) findOriginalValue(decl *syntax.DeclClause) (string, bool) {
	var vals []string
	quoted := false

	if len(decl.Args) < 1 {
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

func (r *EnvResolver) findEnvValue(name string) (string, bool) {
	if r.envCache == nil {
		r.envCache = make(map[string]string)
		r.collectEnvFromSources()
	}
	val, ok := r.envCache[name]
	return val, ok
}

func (r *EnvResolver) collectEnvFromSources() {
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

func (r *EnvResolver) walk(f *syntax.File) (result []*syntax.DeclClause, err error) {
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

		// unless stmt is top level, we don't want to go deeper
		return false
	})

	var buf bytes.Buffer
	// syntax.DebugPrint(&buf, f)
	// syntax.NewPrinter().Print(&buf, f)
	fmt.Println(buf.String())
	return
}

func (r *EnvResolver) resolveExportStmt(stmt *syntax.Stmt) ([]*syntax.DeclClause, error) {
	var result []*syntax.DeclClause
	switch x := stmt.Cmd.(type) {
	case *syntax.DeclClause:
		if x.Variant.Value != "export" && len(x.Args) != 1 {
			return result, nil
		}

		if r.hasSubshell(x.Args[0]) {
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
			syntax.Comment{Text: fmt.Sprintf(" %s", exportStmt.String())},
		)
		stmt.Cmd = nil
		r.modifiedScript = true
	}

	return result, nil
}

// walk AST to check for subshell nodes
func (r *EnvResolver) hasSubshell(node syntax.Node) bool {
	hasSubshell := false
	syntax.Walk(node, func(x syntax.Node) bool {
		switch x.(type) {
		case *syntax.CmdSubst:
			hasSubshell = true
			return false
		case *syntax.Subshell:
			hasSubshell = true
			return false
		}
		return !hasSubshell
	})
	return hasSubshell
}
