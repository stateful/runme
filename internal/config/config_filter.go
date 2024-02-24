package config

import (
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/pkg/errors"
)

const (
	FilterTypeBlock    = "FILTER_TYPE_BLOCK"
	FilterTypeDocument = "FILTER_TYPE_DOCUMENT"
)

type Filter struct {
	Type      string
	Condition string

	once       sync.Once
	program    *vm.Program
	compileErr error
}

// FilterDocumentEnv is the environment with fields corresponding to
// the options documented on https://docs.runme.dev/configuration/document-level.
// Document options are converted to this environment before evaluating the filter.
type FilterDocumentEnv struct {
	Shell string `expr:"shell"`
	Cwd   string `expr:"cwd"`
}

// FilterBlockEnv is the environment with fields corresponding to
// the options documented on https://docs.runme.dev/configuration/cell-level.
// Cell options are converted to this environment before evaluating the filter.
//
// The `expr` tag is used to map the field to the corresponding option.
// Without it, all variables start with capitalized letters.
type FilterBlockEnv struct {
	Language               string   `expr:"language"`
	Name                   string   `expr:"name"`
	Cwd                    string   `expr:"cwd"`
	Interactive            bool     `expr:"interactive"`
	Background             bool     `expr:"background"`
	PromptEnv              bool     `expr:"prompt_env"`
	CloseTerminalOnSuccess bool     `expr:"close_terminal_on_success"`
	ExcludeFromRunAll      bool     `expr:"exclude_from_run_all"`
	Categories             []string `expr:"categories"`
}

func (f *Filter) Evaluate(env interface{}) (bool, error) {
	f.once.Do(func() {
		program, err := expr.Compile(
			f.Condition,
			expr.Env(env),
			expr.AsBool(),
		)
		f.program, f.compileErr = program, errors.Wrap(err, "failed to compile filter program")
	})

	if f.program == nil {
		return false, f.compileErr
	}

	result, err := expr.Run(f.program, env)
	if err != nil {
		return false, errors.Wrap(err, "failed to run filter program")
	}
	return result.(bool), nil
}
