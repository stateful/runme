package project

import (
	"context"
	"regexp"

	"github.com/pkg/errors"
	"github.com/stateful/runme/v3/internal/document"
)

// Task is a project-specific struct.
// It's a `document.CodeBlock` with a path
// to the document it belongs to.
//
// Instance of `Document` can be retrieved
// from `document.CodeBlock`.
type Task struct {
	CodeBlock       *document.CodeBlock `json:"code_block"`
	DocumentPath    string              `json:"document_path"`
	RelDocumentPath string              `json:"rel_document_path"`
}

func (t Task) ID() string {
	return t.DocumentPath + ":" + t.CodeBlock.Name()
}

// LoadFiles returns a list of file names found in the project.
func LoadFiles(ctx context.Context, p *Project) ([]string, error) {
	eventc := make(chan LoadEvent)

	go p.Load(ctx, eventc, true)

	var (
		result []string
		err    error
	)

	for event := range eventc {
		switch event.Type {
		case LoadEventError:
			data := ExtractDataFromLoadEvent[LoadEventErrorData](event)
			if err != nil {
				err = data.Err
			}
		case LoadEventFoundFile:
			data := ExtractDataFromLoadEvent[LoadEventFoundFileData](event)
			result = append(result, data.Path)
		}
	}

	return result, err
}

func LoadTasks(ctx context.Context, p *Project) ([]Task, error) {
	eventc := make(chan LoadEvent)

	go p.Load(ctx, eventc, false)

	var (
		result []Task
		err    error
	)

	for event := range eventc {
		switch event.Type {
		case LoadEventError:
			data := ExtractDataFromLoadEvent[LoadEventErrorData](event)
			if err != nil {
				err = data.Err
			}
		case LoadEventFoundTask:
			data := ExtractDataFromLoadEvent[LoadEventFoundTaskData](event)
			result = append(result, data.Task)
		}
	}

	return result, err
}

type Filter func(Task) (bool, error)

var ErrReturnEarly = errors.New("return early")

func FilterTasksByFn(tasks []Task, fns ...Filter) (result []Task, err error) {
	for _, task := range tasks {
		var errFn error

		include := true

		for _, fn := range fns {
			var ok bool

			ok, errFn = fn(task)
			if !ok {
				include = false
				break
			}
			if errFn != nil {
				break
			}
		}

		if include {
			result = append(result, task)
		}

		if errFn != nil {
			if !errors.Is(errFn, ErrReturnEarly) {
				err = errFn
			}
			return
		}
	}

	return
}

func FilterTasksByID(tasks []Task, expr string) (result []Task, err error) {
	matcher, err := compileRegex(expr)
	if err != nil {
		return nil, err
	}

	return FilterTasksByFn(tasks, func(task Task) (bool, error) {
		return matcher.MatchString(task.ID()), nil
	})
}

func FilterTasksByFilename(tasks []Task, expr string) (result []Task, err error) {
	matcher, err := compileRegex(expr)
	if err != nil {
		return nil, err
	}

	return FilterTasksByFn(tasks, func(task Task) (bool, error) {
		return matcher.MatchString(task.DocumentPath), nil
	})
}

func FilterTasksByExactTaskName(tasks []Task, name string) (result []Task, err error) {
	return FilterTasksByFn(tasks, func(task Task) (bool, error) {
		return task.CodeBlock.Name() == name, nil
	})
}

func CompileRegex(query string) (Matcher, error) {
	return compileRegex(query)
}

func compileRegex(query string) (Matcher, error) {
	if query == "" {
		return &matchAll{}, nil
	}
	reg, err := regexp.Compile(query)
	return reg, errors.Wrapf(err, "invalid regexp %q", query)
}

type Matcher interface {
	MatchString(string) bool
}

type matchAll struct{}

func (matchAll) MatchString(string) bool { return true }
