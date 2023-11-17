package project

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
)

// Task is a project-specific struct.
// It's a `document.CodeBlock` with a path
// to the document it belongs to.
//
// Instance of `Document` can be retrieved
// from `document.CodeBlock`.
type Task struct {
	CodeBlock    *document.CodeBlock
	DocumentPath string
}

func (t Task) ID() string {
	return t.DocumentPath + ":" + t.CodeBlock.Name()
}

func (t Task) String() string {
	return t.ID()
}

func (t Task) Format(s fmt.State, verb rune) {
	_, _ = s.Write([]byte("project.Task#" + t.ID()))
}

// LoadFiles returns a list of file names found in the project.
//
// TODO(adamb): figure out a strategy for error handling. There are
// two options: (1) stop and return the error immediately,
// (2) continue and return first (or all) error and the list of file names.
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

// SortTasks sorts tasks in ascending nested order.
func SortTasks(tasks []Task) []Task {
	tasksByFile := make(map[string][]Task)

	var files []string

	for _, task := range tasks {
		if arr, ok := tasksByFile[task.DocumentPath]; ok {
			tasksByFile[task.DocumentPath] = append(arr, task)
			continue
		}

		tasksByFile[task.DocumentPath] = []Task{task}
		files = append(files, task.DocumentPath)
	}

	sort.SliceStable(files, func(i, j int) bool {
		return getFileDepth(files[i]) < getFileDepth(files[j])
	})

	result := make([]Task, 0, len(tasks))

	for _, file := range files {
		result = append(result, tasksByFile[file]...)
	}

	return result
}

func getFileDepth(fp string) int {
	return len(strings.Split(fp, string(filepath.Separator)))
}

var ErrReturnEarly = errors.New("return early")

func FilterTasks(tasks []Task, fn func(Task) (bool, error)) (result []Task, err error) {
	for _, task := range tasks {
		include, errFn := fn(task)

		if include {
			result = append(result, task)
		}

		if errFn != nil {
			if !errors.Is(errFn, ErrReturnEarly) {
				err = errFn
			}
			break
		}
	}

	return
}

type Matcher interface {
	MatchString(string) bool
}

type matchAll struct{}

func (matchAll) MatchString(string) bool { return true }

func compileQuery(query string) (Matcher, error) {
	if query == "" {
		return &matchAll{}, nil
	}
	reg, err := regexp.Compile(query)
	return reg, errors.Wrapf(err, "invalid regexp %q", query)
}

func FilterTasksByID(tasks []Task, query string) ([]Task, error) {
	queryMatcher, err := compileQuery(query)
	if err != nil {
		return nil, err
	}

	var results []Task

	for _, task := range tasks {
		if !queryMatcher.MatchString(task.ID()) {
			continue
		}

		results = append(results, task)
	}

	return results, nil
}

func FilterTasksByFileAndTaskName(tasks []Task, queryFile string, queryName string) ([]Task, error) {
	fileMatcher, err := compileQuery(queryFile)
	if err != nil {
		return nil, err
	}

	var results []Task

	foundFile := false

	for _, task := range tasks {
		if !fileMatcher.MatchString(task.DocumentPath) {
			continue
		}

		foundFile = true

		// This is expected that the task name query is
		// matched exactly.
		if queryName != task.CodeBlock.Name() {
			continue
		}

		results = append(results, task)
	}

	if len(results) == 0 {
		if !foundFile {
			return nil, &ErrTaskWithFilenameNotFound{queryFile: queryFile}
		}
		return nil, &ErrTaskWithNameNotFound{queryName: queryName}
	}

	return results, nil
}

type ErrTaskWithFilenameNotFound struct {
	queryFile string
}

func (e ErrTaskWithFilenameNotFound) Error() string {
	return fmt.Sprintf("unable to find file in project matching regex %q", e.queryFile)
}

type ErrTaskWithNameNotFound struct {
	queryName string
}

func (e ErrTaskWithNameNotFound) Error() string {
	return fmt.Sprintf("unable to find any script named %q", e.queryName)
}

func IsTaskNotFoundError(err error) bool {
	return errors.As(err, &ErrTaskWithFilenameNotFound{}) || errors.As(err, &ErrTaskWithNameNotFound{})
}
