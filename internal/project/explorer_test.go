package project

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExplorer_Files(t *testing.T) {
	gitProject := filepath.Join(testdataDir(), "git-project")

	p, err := NewDirProject(gitProject, WithRespectGitignore())
	require.NoError(t, err)

	explorer := NewExplorer(context.Background(), p)
	files, err := explorer.Files()
	require.NoError(t, err)
	assert.EqualValues(t, []string{
		filepath.Join(gitProject, "ignored.md"),
		filepath.Join(gitProject, "readme.md"),
	}, files)
}

func TestExplorer_Tasks(t *testing.T) {
	gitProject := filepath.Join(testdataDir(), "git-project")

	p, err := NewDirProject(gitProject, WithRespectGitignore())
	require.NoError(t, err)

	explorer := NewExplorer(context.Background(), p)
	tasks, err := explorer.Tasks()
	require.NoError(t, err)
	assert.Len(t, tasks, 2)
}
