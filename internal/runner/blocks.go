package runner

import (
	// "github.com/stateful/runme/internal/cmd"
	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/internal/project"
)

func GetBlocks(proj project.Project, AllowUnnamed bool) ([]*runnerv1.Uri, error) {

	var allBlocks []*runnerv1.Uri
	uri := &runnerv1.Uri{
		Scheme:   "fileOne",
		Path:     "/home/runner/.runme/blocks/runnerOne",
		FsPath:   "/home/runner/.runme/blocks/runnerOne",
		Query:    "TestTwo",
		Fragment: "runnerOne",
	}
	uri2 := &runnerv1.Uri{
		Scheme:   "SchemaTest",
		Path:     "/home/runner/.runme/blocks/runnerTwo",
		FsPath:   "/home/runner/.runme/blocks/runnerTwo",
		Query:    "TestTwo",
		Fragment: "runnerThwo",
	}
	allBlocks = append(allBlocks, uri)
	allBlocks = append(allBlocks, uri2)

	// allBlocks, err := loadTasks(proj, cmd.OutOrStdout(), cmd.InOrStdin(), true)
	// if err != nil {
	// 	return err
	// }
	return allBlocks, nil
}
