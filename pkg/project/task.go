package project

import (
	"context"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pkg/errors"

	"github.com/stateful/runme/v3/pkg/document"
)

// Task is struct representing a [document.CodeBlock] within the context of a project.
// Instance of [document.Document] can be retrieved from [Task]'s code block.
// [Task] contains absolute and relative path to the document.
type Task struct {
	CodeBlock       *document.CodeBlock `json:"code_block"`
	DocumentPath    string              `json:"document_path"`
	RelDocumentPath string              `json:"rel_document_path"`
}

func (t Task) ID() string {
	return t.RelDocumentPath + ":" + t.CodeBlock.Name()
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

// SortByProximity sorts tasks by promximity to the cwd
func SortByProximity(tasks []Task, cwd string) {
	slices.SortStableFunc(tasks, func(a, b Task) int {
		aRelToCwd := GetRelativePath(cwd, a.DocumentPath)
		bRelToCwd := GetRelativePath(cwd, b.DocumentPath)

		aLevels := len(strings.Split(aRelToCwd, string(filepath.Separator)))
		bLevels := len(strings.Split(bRelToCwd, string(filepath.Separator)))

		return aLevels - bLevels
	})
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
