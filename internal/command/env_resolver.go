package command

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
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
	sources  []EnvResolverSource
	envCache map[string]string
}

func NewEnvResolver(sources ...EnvResolverSource) *EnvResolver {
	return &EnvResolver{sources: sources, envCache: nil}
}

type EnvResolverResult struct {
	Name          string
	OriginalValue string
	Value         string
}

func (r *EnvResolverResult) IsResolved() bool {
	return r.Value != ""
}

func (r *EnvResolver) Resolve(reader io.Reader) ([]*EnvResolverResult, error) {
	decls, err := r.parse(reader)
	if err != nil {
		return nil, err
	}

	var result []*EnvResolverResult

	for _, decl := range decls {
		if len(decl.Args) != 1 {
			continue
		}

		arg := decl.Args[0]

		name := arg.Name.Value
		originalValue := r.findOriginalValue(decl)
		value, _ := r.findEnvValue(name)

		result = append(result, &EnvResolverResult{
			Name:          name,
			OriginalValue: originalValue,
			Value:         value,
		})
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

	return result, nil
}

func (r *EnvResolver) findOriginalValue(decl *syntax.DeclClause) string {
	if len(decl.Args) != 1 {
		return ""
	}

	arg := decl.Args[0]

	if arg.Value == nil {
		return ""
	}

	parts := arg.Value.Parts

	if len(parts) != 1 {
		return ""
	}

	switch part := parts[0].(type) {
	// export FOO=bar
	case *syntax.Lit:
		return part.Value
	// export FOO="bar"
	case *syntax.DblQuoted:
		if len(part.Parts) == 1 {
			p, ok := part.Parts[0].(*syntax.Lit)
			// break for quoted stmt, ie non-literal
			// export FOO="$( echo 'this is a test' )"
			if !ok {
				break
			}
			return p.Value
		}
	// export FOO='bar'
	case *syntax.SglQuoted:
		return part.Value
	// export FOO=${FOO:-bar}
	case *syntax.ParamExp:
		if part.Exp.Op == syntax.DefaultUnsetOrNull {
			return part.Exp.Word.Lit()
		}
	}

	return ""
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

func (r *EnvResolver) parse(reader io.Reader) ([]*syntax.DeclClause, error) {
	f, err := syntax.NewParser().Parse(reader, "")
	if err != nil {
		return nil, err
	}

	var result []*syntax.DeclClause

	syntax.Walk(f, func(node syntax.Node) bool {
		switch x := node.(type) {
		case *syntax.DeclClause:
			if x.Variant.Value == "export" && len(x.Args) == 1 {
				result = append(result, x)
				return false
			}
		default:
			// noop
		}
		return true
	})

	// f.Stmts = f.Stmts[0:3]

	var buf bytes.Buffer
	syntax.DebugPrint(&buf, f)
	// syntax.NewPrinter().Print(&buf, f)
	fmt.Println(buf.String())

	return result, nil
}
