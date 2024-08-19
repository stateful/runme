package command

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"mvdan.cc/sh/v3/syntax"
)

type ProgramResolverMode uint8

const (
	// ProgramResolverModeAuto is a default which prompts for all unresolved variables.
	ProgramResolverModeAuto ProgramResolverMode = iota
	// ProgramResolverModePromptAll always prompts even if variables are resolved.
	ProgramResolverModePromptAll
	// ProgramResolverModeSkipAll does not prompt even if variables are unresolved.
	// All variables will be marked as resolved.
	ProgramResolverModeSkipAll
)

type ProgramResolverSource func() []string

func ProgramResolverSourceFunc(env []string) ProgramResolverSource {
	return func() []string {
		return env
	}
}

// ProgramResolver uses a list of ProgramResolverSource to resolve environment variables
// found in a shell program.
type ProgramResolver struct {
	mode    ProgramResolverMode
	sources []ProgramResolverSource

	sensitiveEnvNames []string
	envCache          *sync.Map

	astPrinter *syntax.Printer
}

func NewProgramResolver(mode ProgramResolverMode, sensitiveEnvNames []string, sources ...ProgramResolverSource) *ProgramResolver {
	return &ProgramResolver{
		mode:              mode,
		sensitiveEnvNames: sensitiveEnvNames,
		sources:           sources,
		astPrinter:        syntax.NewPrinter(),
	}
}

type ProgramResolverStatus uint8

const (
	// ProgramResolverStatusUnresolved indicates a variable is unresolved.
	ProgramResolverStatusUnresolved ProgramResolverStatus = iota
	// ProgramResolverStatusUnresolvedWithMessage indicates a variable is unresolved but it has a message.
	// It typically means that the variable is of form `export FOO=this is a message`.
	ProgramResolverStatusUnresolvedWithMessage
	// ProgramResolverStatusUnresolvedWithPlaceholder indicates a variable is unresolved but it has a placeholder.
	// It typically means that the variable is of form `export FOO="this is a placeholder"`.
	ProgramResolverStatusUnresolvedWithPlaceholder
	// ProgramResolverStatusUnresolvedWithSecret indicates a variable is unresolved and needs to be treated with sensitivity.
	// It typically means that the variable is a password, certificate, or access key.
	ProgramResolverStatusUnresolvedWithSecret
	// ProgramResolverStatusResolved indicates a variable is resolved.
	ProgramResolverStatusResolved
)

type ProgramResolverResult struct {
	Variables       []ProgramResolverVarResult
	ModifiedProgram bool
}

type ProgramResolverVarResult struct {
	// Status indicates the status of the result.
	Status ProgramResolverStatus

	// Name is the name of the variable.
	// It is set always.
	Name string

	// OriginalValue is the original value of the variable.
	// It's either a placeholder (`export FOO="this is a placeholder"`) or
	// a message (`export FOO=this is a message`).
	OriginalValue string

	// Value is the resolved value of the variable.
	// It is set only if Status is ProgramResolverStatusResolved.
	Value string
}

// Resolve resolves the environment variables found in a shell program.
// It might modify the program and write it provided writer.
func (r *ProgramResolver) Resolve(reader io.Reader, writer io.Writer) (*ProgramResolverResult, error) {
	f, err := syntax.NewParser().Parse(reader, "")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	decls, modified, err := r.walk(f)
	if err != nil {
		return nil, err
	}

	result := ProgramResolverResult{
		ModifiedProgram: modified,
	}

	for _, decl := range decls {
		if len(decl.Args) < 1 {
			continue
		}

		name := decl.Args[0].Name.Value
		originalValue, isPlaceholder := r.findOriginalValue(decl)
		resolvedValue, hasResolvedValue := r.findEnvValue(name)
		isSensitive := r.IsEnvSensitive(name)

		varResult := ProgramResolverVarResult{
			Status:        ProgramResolverStatusUnresolved,
			Name:          name,
			OriginalValue: originalValue,
			Value:         resolvedValue,
		}

		switch r.mode {
		case ProgramResolverModePromptAll:
			if isSensitive {
				varResult.Status = ProgramResolverStatusUnresolvedWithSecret
				varResult.Value = ""
				break
			}
			if hasResolvedValue {
				varResult.Status = ProgramResolverStatusUnresolvedWithPlaceholder
				break
			}
			if isPlaceholder {
				varResult.Status = ProgramResolverStatusUnresolvedWithPlaceholder
			} else {
				varResult.Status = ProgramResolverStatusUnresolvedWithMessage
			}
		case ProgramResolverModeSkipAll:
			// For ProgramResolverModeSkip the status is awalys ProgramResolverStatusResolved.
			varResult.Status = ProgramResolverStatusResolved
			if !hasResolvedValue {
				varResult.Value = originalValue
			}
		default:
			if hasResolvedValue {
				varResult.Status = ProgramResolverStatusResolved
			} else if isSensitive {
				varResult.Status = ProgramResolverStatusUnresolvedWithSecret
			} else if isPlaceholder {
				varResult.Status = ProgramResolverStatusUnresolvedWithPlaceholder
			} else if originalValue != "" {
				varResult.Status = ProgramResolverStatusUnresolvedWithMessage
			} else {
				varResult.Status = ProgramResolverStatusUnresolved
			}
		}

		result.Variables = append(result.Variables, varResult)
	}

	slices.SortStableFunc(result.Variables, func(a, b ProgramResolverVarResult) int {
		aResolved, bResolved := a.Status == ProgramResolverStatusResolved, b.Status == ProgramResolverStatusResolved
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
	return &result, nil
}

// findOriginalValue walks the AST to find the original value of the variable.
// This method is called only for the value of "export" statement.
func (r *ProgramResolver) findOriginalValue(decl *syntax.DeclClause) (string, bool) {
	if len(decl.Args) < 1 || decl.Args[0].Value == nil {
		return "", false
	}

	var fragments []string

	isPlaceholder := false
	processingVarName := false

	syntax.Walk(decl, func(node syntax.Node) bool {
		switch node := node.(type) {
		// Skip variable name.
		case *syntax.Assign:
			if node.Name != nil && !node.Naked {
				processingVarName = true
			}

			return true

		// export FOO=bar
		case *syntax.Lit:
			if processingVarName {
				processingVarName = false
				return false
			}

			fragments = append(fragments, node.Value)

			return true

		// export FOO="bar"
		case *syntax.DblQuoted:
			if len(node.Parts) == 1 {
				p, ok := node.Parts[0].(*syntax.Lit)
				// Break for quoted stmt, ie non-literal, e.g. export FOO="$( echo 'this is a test' )"
				if !ok {
					break
				}

				if p.Value != "" {
					isPlaceholder = true
					fragments = append(fragments, p.Value)
				}

				return false
			}

		// export FOO='bar'
		case *syntax.SglQuoted:
			if node.Value != "" {
				isPlaceholder = true
				fragments = append(fragments, node.Value)
			}
			return false

		// export FOO=${FOO:-bar}
		case *syntax.ParamExp:
			if node.Exp.Op == syntax.DefaultUnsetOrNull {
				fragments = append(fragments, node.Exp.Word.Lit())
				return true
			}
		}

		return true
	})

	return strings.Join(fragments, " "), isPlaceholder
}

func (r *ProgramResolver) findEnvValue(name string) (string, bool) {
	r.collectEnvFromSources()

	val, ok := r.envCache.Load(name)
	if ok {
		return val.(string), ok
	}
	return "", ok
}

func (r *ProgramResolver) IsEnvSensitive(name string) bool {
	for _, key := range r.sensitiveEnvNames {
		if key == name {
			return true
		}
	}
	return false
}

func (r *ProgramResolver) collectEnvFromSources() {
	if r.envCache != nil {
		return
	}

	r.envCache = &sync.Map{}

	for _, source := range r.sources {
		env := source()
		for _, e := range env {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				r.envCache.Store(parts[0], parts[1])
			}
		}
	}
}

func (r *ProgramResolver) walk(f *syntax.File) (result []*syntax.DeclClause, modified bool, err error) {
	syntax.Walk(f, func(node syntax.Node) bool {
		if err != nil {
			return false
		}

		switch node := node.(type) {
		case *syntax.File:
			return true
		case *syntax.Stmt:
			decl, ok := r.isStmtSupportedDecl(node)
			if !ok {
				return false
			}

			err = r.modifyStmt(node, decl)
			if err != nil {
				return false
			}

			// TODO(adamb): does not make sense
			modified = true

			result = append(result, decl)

			return false
		default:
			// noop
		}

		// Unless stmt is at top level bail;
		// we don't want to go deeper.
		return false
	})

	return
}

func (r *ProgramResolver) isStmtSupportedDecl(stmt *syntax.Stmt) (*syntax.DeclClause, bool) {
	decl, ok := stmt.Cmd.(*syntax.DeclClause)
	if !ok {
		return nil, false
	}

	isSupported := decl.Variant.Value == "export" && !r.hasExpr(decl.Args[0])

	if !isSupported {
		return nil, false
	}

	return decl, true
}

func (r *ProgramResolver) modifyStmt(stmt *syntax.Stmt, decl *syntax.DeclClause) error {
	exportStmt := bytes.NewBuffer(nil)

	if err := r.astPrinter.Print(exportStmt, decl); err != nil {
		return errors.WithStack(err)
	}

	stmt.Comments = append(stmt.Comments,
		syntax.Comment{Text: "\n"},
		syntax.Comment{Text: fmt.Sprintf(" %s set in managed env store", decl.Args[0].Name.Value)},
		syntax.Comment{Text: fmt.Sprintf(" %q", exportStmt.String())},
	)

	stmt.Cmd = nil

	return nil
}

// hasExpr walks the AST to check for nested expressions.
func (r *ProgramResolver) hasExpr(node syntax.Node) (found bool) {
	syntax.Walk(node, func(node syntax.Node) bool {
		switch node.(type) {
		case *syntax.CmdSubst, *syntax.Subshell, *syntax.ParamExp, *syntax.ArithmExp, *syntax.ProcSubst, *syntax.ExtGlob, *syntax.BraceExp:
			found = true
			return false
		default:
			return !found
		}
	})
	return
}
