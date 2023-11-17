package project

import (
	"fmt"
	"regexp"

	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
)

type CodeBlock struct {
	*document.CodeBlock
	Filename string
}

func (b CodeBlock) ID() string {
	return b.Filename + ":" + b.Name()
}

func (b CodeBlock) IsEmpty() bool {
	return b == CodeBlock{}
}

type CodeBlocks []CodeBlock

func (blocks CodeBlocks) LookupByID(query string) (CodeBlocks, error) {
	if query == "" {
		return blocks, nil
	}

	matcher, err := compileQuery(query)
	if err != nil {
		return nil, err
	}

	var result CodeBlocks

	for _, b := range blocks {
		if matcher.MatchString(b.ID()) {
			continue
		}
		result = append(result, b)
	}

	return result, nil
}

func (blocks CodeBlocks) LookupByName(name string) CodeBlocks {
	var result CodeBlocks

	for _, b := range blocks {
		if b.Name() == name {
			result = append(result, b)
		}
	}

	return result
}

type ErrCodeBlockFileNotFound struct {
	queryFile string
}

func (e ErrCodeBlockFileNotFound) Error() string {
	return fmt.Sprintf("unable to find file in project matching regex %q", e.FailedFileQuery())
}

func (e ErrCodeBlockFileNotFound) FailedFileQuery() string {
	return e.queryFile
}

type ErrCodeBlockNameNotFound struct {
	queryName string
}

func (e ErrCodeBlockNameNotFound) Error() string {
	return fmt.Sprintf("unable to find any script named %q", e.queryName)
}

func (e ErrCodeBlockNameNotFound) FailedNameQuery() string {
	return e.queryName
}

func IsCodeBlockNotFoundError(err error) bool {
	return errors.As(err, &ErrCodeBlockNameNotFound{}) || errors.As(err, &ErrCodeBlockFileNotFound{})
}

func (blocks CodeBlocks) LookupWithFileAndName(queryFile, name string) (CodeBlocks, error) {
	if queryFile == "" {
		return blocks.LookupByName(name), nil
	}

	matcher, err := compileQuery(queryFile)
	if err != nil {
		return nil, err
	}

	var result CodeBlocks

	foundFile := false

	for _, b := range blocks {
		if !matcher.MatchString(b.Filename) {
			continue
		}

		foundFile = true

		if name != b.Name() {
			continue
		}

		result = append(result, b)
	}

	if len(result) == 0 {
		if !foundFile {
			return nil, ErrCodeBlockFileNotFound{queryFile: queryFile}
		}

		return nil, ErrCodeBlockNameNotFound{queryName: name}
	}

	return result, nil
}

func DocumentCodeBlocksFromBlocks(blocks CodeBlocks) []*document.CodeBlock {
	result := make([]*document.CodeBlock, 0, len(blocks))
	for _, b := range blocks {
		result = append(result, b.CodeBlock)
	}
	return result
}

func compileQuery(query string) (*regexp.Regexp, error) {
	reg, err := regexp.Compile(query)
	if err != nil {
		return nil, errors.Wrapf(err, "failed compiling query %q to regexp: %v", query, err)
	}
	return reg, nil
}

func FilterBlocks(blocks CodeBlocks, fn func(CodeBlock) (bool, bool)) CodeBlocks {
	var result CodeBlocks

	for _, b := range blocks {
		include, finish := fn(b)

		if include {
			result = append(result, b)
		}

		if finish {
			break
		}
	}

	return result
}
