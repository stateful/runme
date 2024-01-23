package command

import (
	"io"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
)

type (
	ResolveEnvResult = runnerv2alpha1.ResolveEnvResult
)

type EnvResolverSource func() []string

func EnvResolverSourceFunc(env []string) EnvResolverSource {
	return func() []string {
		return env
	}
}

// EnvResolver uses a list of EnvResolverSource to resolve environment variables
// found in a shell program. The result contains all found environment variables.
// If the env is in any source, it is considered resolved. Otherwise, it is makred
// as unresolved.
type EnvResolver struct {
	sources  []EnvResolverSource
	envCache map[string]string
}

func NewEnvResolver(sources ...EnvResolverSource) *EnvResolver {
	return &EnvResolver{sources: sources, envCache: nil}
}

func (r *EnvResolver) Resolve(reader io.Reader) ([]*ResolveEnvResult, error) {
	decls, err := r.parse(reader)
	if err != nil {
		return nil, err
	}

	var result []*ResolveEnvResult

	for _, decl := range decls {
		if len(decl.Args) != 1 {
			continue
		}

		arg := decl.Args[0]

		name := arg.Name.Value
		originalValue := r.findOriginalValue(decl)

		value, ok := r.findEnvValue(name)
		if ok {
			result = append(result, &ResolveEnvResult{
				Result: &runnerv2alpha1.ResolveEnvResult_ResolvedEnv_{
					ResolvedEnv: &runnerv2alpha1.ResolveEnvResult_ResolvedEnv{
						Name:          name,
						ResolvedValue: value,
						OriginalValue: originalValue,
					},
				},
			})
		} else {
			result = append(result, &ResolveEnvResult{
				Result: &runnerv2alpha1.ResolveEnvResult_UnresolvedEnv_{
					UnresolvedEnv: &runnerv2alpha1.ResolveEnvResult_UnresolvedEnv{
						Name:          name,
						OriginalValue: originalValue,
					},
				},
			})
		}
	}

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
	case *syntax.Lit:
		return part.Value
	case *syntax.DblQuoted:
		if len(part.Parts) == 1 {
			return part.Parts[0].(*syntax.Lit).Value
		}
	case *syntax.SglQuoted:
		return part.Value
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

	return result, nil
}
