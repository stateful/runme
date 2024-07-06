package beta

import (
	"github.com/gobwas/glob"
	"github.com/pkg/errors"

	"github.com/stateful/runme/v3/pkg/project"
)

func createProjectFilterFromPatterns(patterns []string) (project.Filter, error) {
	if len(patterns) == 0 {
		return func(t project.Task) (bool, error) { return true, nil }, nil
	}

	globs, err := parseGlobs(patterns)
	if err != nil {
		return nil, err
	}

	return func(t project.Task) (bool, error) {
		for _, g := range globs {
			if g.Match(t.CodeBlock.Name()) {
				return true, nil
			}
		}
		return false, nil
	}, nil
}

func parseGlobs(patterns []string) ([]glob.Glob, error) {
	globs := make([]glob.Glob, 0, len(patterns))
	for _, item := range patterns {
		g, err := glob.Compile(item)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		globs = append(globs, g)
	}
	return globs, nil
}
