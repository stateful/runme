package config

import (
	"reflect"

	"github.com/expr-lang/expr"
	"github.com/pkg/errors"
)

const (
	FilterTypeBlock    = "FILTER_TYPE_BLOCK"
	FilterTypeDocument = "FILTER_TYPE_DOCUMENT"
)

type Filter struct {
	Type      string
	Condition string
	Extra     map[string]interface{}
}

// FilterDocumentEnv is the environment with fields corresponding to
// the options documented on https://docs.runme.dev/configuration/document-level.
// Document options are converted to this environment before evaluating the filter.
type FilterDocumentEnv struct {
	Cwd   string `expr:"cwd"`
	Shell string `expr:"shell"`
}

// FilterBlockEnv is the environment with fields corresponding to
// the options documented on https://docs.runme.dev/configuration/cell-level.
// Cell options are converted to this environment before evaluating the filter.
//
// The `expr` tag is used to map the field to the corresponding option.
// Without it, all variables start with capitalized letters.
type FilterBlockEnv struct {
	Background             bool     `expr:"background"`
	CloseTerminalOnSuccess bool     `expr:"close_terminal_on_success"`
	Cwd                    string   `expr:"cwd"`
	ExcludeFromRunAll      bool     `expr:"exclude_from_run_all"`
	Interactive            bool     `expr:"interactive"`
	IsNamed                bool     `expr:"is_named"`
	Language               string   `expr:"language"`
	Name                   string   `expr:"name"`
	PromptEnv              bool     `expr:"prompt_env"`
	Tags                   []string `expr:"tags"`
}

type filterDocumentWithExtraEnv struct {
	FilterDocumentEnv
	Extra map[string]interface{} `expr:"extra"`
}

type filterBlockWithExtraEnv struct {
	FilterBlockEnv
	Extra map[string]interface{} `expr:"extra"`
}

func (f *Filter) Evaluate(env interface{}) (bool, error) {
	var envWithExtra interface{}

	switch env := env.(type) {
	case FilterDocumentEnv:
		envWithExtra = filterDocumentWithExtraEnv{FilterDocumentEnv: env, Extra: f.Extra}
	case FilterBlockEnv:
		envWithExtra = filterBlockWithExtraEnv{FilterBlockEnv: env, Extra: f.Extra}
	default:
		panic("invariant: unsupported env type " + reflect.TypeOf(env).String())
	}

	program, err := expr.Compile(
		f.Condition,
		expr.AsBool(),
		intersection,
	)
	if err != nil {
		return false, errors.Wrap(err, "failed to compile filter program")
	}

	result, err := expr.Run(program, envWithExtra)
	if err != nil {
		return false, errors.Wrap(err, "failed to run filter program")
	}
	return result.(bool), nil
}

var intersection = expr.Function(
	"intersection",
	func(params ...any) (any, error) {
		if len(params) != 2 {
			return nil, errors.New("intersection: expected 2 arguments")
		}

		a, ok := params[0].([]string)
		if !ok {
			return nil, errors.Errorf("intersection: expected first argument to be a list of strings, got %T", params[0])
		}

		b, ok := params[1].([]string)
		if !ok {
			return nil, errors.Errorf("intersection: expected second argument to be a list of strings, got %T", params[1])
		}

		m := make(map[string]bool)
		for _, v := range a {
			m[v] = true
		}

		var result []string
		for _, v := range b {
			if m[v] {
				result = append(result, v)
			}
		}

		return result, nil
	},
	new(func([]string, []string) []string),
)
