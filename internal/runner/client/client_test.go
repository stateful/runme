package client

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stateful/runme/internal/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDirectory(t *testing.T) {
	_, b, _, _ := runtime.Caller(0)
	root := filepath.Clean(
		filepath.Join(
			filepath.Dir(b),
			filepath.FromSlash("../../../"),
		),
	)

	projectRoot := filepath.Join(root, "examples/frontmatter/cwd")

	// repo path
	rp := func(rel string) string {
		return filepath.Join(root, filepath.FromSlash(rel))
	}

	proj, err := project.NewDirProject(projectRoot)
	require.NoError(t, err)

	tasks, err := project.LoadTasks(context.Background(), proj)
	require.NoError(t, err)

	taskMap := make(map[string]string, len(tasks))

	for _, task := range tasks {
		resolved := ResolveDirectory(root, task)
		taskMap[task.CodeBlock.Name()] = resolved
	}

	if runtime.GOOS == "windows" {
		assert.Equal(t, rp("examples\\frontmatter\\cwd"), taskMap["none-pwd"])
		assert.Equal(t, rp("examples\\frontmatter"), taskMap["none-rel-pwd"])

		assert.Equal(t, root, taskMap["relative-pwd"])
		assert.Equal(t, rp("../"), taskMap["relative-rel-pwd"])
	} else {
		assert.Equal(t, map[string]string{
			"absolute-pwd":     "/tmp",
			"absolute-rel-pwd": "/",
			"absolute-abs-pwd": "/opt",

			"none-pwd":     rp("examples/frontmatter/cwd"),
			"none-rel-pwd": rp("examples/frontmatter"),
			"none-abs-pwd": "/opt",

			"relative-pwd":     root,
			"relative-rel-pwd": rp("../"),
			"relative-abs-pwd": "/opt",
		}, taskMap)
	}
}
