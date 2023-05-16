package project

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/cmark"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pfs = projectDir()

func Test_CodeBlocks(t *testing.T) {
	t.Run("LookupWithFile", func(t *testing.T) {
		lfs, err := pfs.Chroot("../")
		require.NoError(t, err)

		blocks := make(CodeBlocks, 0)

		for _, file := range []string{"TEST.md", "TEST2.md"} {
			bytes, err := util.ReadFile(lfs, file)
			require.NoError(t, err)

			doc := document.New(bytes, cmark.Render)
			node, _, err := doc.Parse()
			require.NoError(t, err)

			parsedBlocks := document.CollectCodeBlocks(node)

			for _, block := range parsedBlocks {
				blocks = append(blocks, CodeBlock{
					Block: block,
					File:  file,
				})
			}
		}

		{
			res, err := blocks.LookupWithFile("TEST", "echo-hi")
			require.NoError(t, err)
			assert.Equal(t, 2, len(res))

			for _, fileBlock := range res {
				assert.Equal(t, "echo-hi", fileBlock.Block.Name())
			}
		}

		{
			res, err := blocks.LookupWithFile("TEST.md", "echo-hi")
			require.NoError(t, err)
			assert.Equal(t, 1, len(res))
		}
	})
}

func Test_directoryGitProject(t *testing.T) {
	pfs.MkdirAll(".git", os.FileMode(0o700))
	defer util.RemoveAll(pfs, ".git")

	dotgitFs, err := pfs.Chroot(".git")
	require.NoError(t, err)

	storage := filesystem.NewStorage(dotgitFs, nil)

	_, err = git.Init(storage, nil)
	require.NoError(t, err)

	proj, err := NewDirectoryProject(pfs.Root(), true, true)
	require.NoError(t, err)
	require.NotNil(t, proj.repo)

	wt, err := proj.repo.Worktree()
	require.NoError(t, err)
	t.Log(wt.Filesystem.Root())

	util.WriteFile(pfs, ".gitignore", []byte("IGNORED.md\nignored"), os.FileMode(int(0o700)))
	defer pfs.Remove(".gitignore")

	t.Run("LoadEnvs", func(t *testing.T) {
		proj.SetEnvLoadOrder([]string{".env.local", ".env"})

		envs, err := proj.LoadEnvs()
		require.NoError(t, err)

		assert.Equal(t, map[string]string{
			"SECRET_1": "secret1_overriden",
			"SECRET_2": "secret2",
			"SECRET_3": "secret3",
		}, envs)
	})

	t.Run("LoadTasks", func(t *testing.T) {
		tasks, err := proj.LoadTasks()
		require.NoError(t, err)

		assert.Equal(t, 2, len(tasks))

		blocks := make(map[string]CodeBlock)

		for _, task := range tasks {
			blocks[task.Block.Name()] = task
		}

		assert.Equal(
			t,
			convertLine("echo hello"),
			string(blocks["echo-hello"].Block.Content()),
		)

		assert.Equal(
			t,
			convertLine("echo chao"),
			string(blocks["echo-chao"].Block.Content()),
		)

		assert.Equal(
			t,
			"README.md",
			string(blocks["echo-hello"].File),
		)

		assert.Equal(
			t,
			convertFilePath("src/DOCS.md"),
			string(blocks["echo-chao"].File),
		)
	})
}

func Test_directoryBareProject(t *testing.T) {
	proj, err := NewDirectoryProject(pfs.Root(), false, true)
	require.NoError(t, err)

	t.Run("LoadEnvs", func(t *testing.T) {
		proj.SetEnvLoadOrder([]string{".env.local", ".env"})

		envs, err := proj.LoadEnvs()
		require.NoError(t, err)

		assert.Equal(t, map[string]string{
			"SECRET_1": "secret1_overriden",
			"SECRET_2": "secret2",
			"SECRET_3": "secret3",
		}, envs)
	})

	t.Run("LoadTasks", func(t *testing.T) {
		tasks, err := proj.LoadTasks()
		require.NoError(t, err)

		assert.Equal(t, 4, len(tasks))

		blocks := make(map[string]CodeBlock)

		for _, task := range tasks {
			blocks[fmt.Sprintf("%s:%s", task.File, task.Block.Name())] = task
		}

		assert.Equal(
			t,
			convertLine("echo hello"),
			string(blocks["README.md:echo-hello"].Block.Content()),
		)

		assert.Equal(
			t,
			convertLine("echo chao"),
			string(blocks[convertFilePath("src/DOCS.md:echo-chao")].Block.Content()),
		)

		assert.Equal(
			t,
			convertLine("echo ignored"),
			string(blocks["IGNORED.md:echo-ignored"].Block.Content()),
		)

		assert.Equal(
			t,
			convertLine("echo hi"),
			string(blocks[convertFilePath("ignored/README.md:echo-hi")].Block.Content()),
		)
	})
}

func Test_singleFileProject(t *testing.T) {
	proj := NewSingleFileProject(filepath.Join(pfs.Root(), "README.md"), true)

	t.Run("LoadEnvs", func(t *testing.T) {
		envs, err := proj.LoadEnvs()
		require.NoError(t, err)
		assert.Nil(t, envs)
	})

	t.Run("LoadTasks", func(t *testing.T) {
		tasks, err := proj.LoadTasks()
		require.NoError(t, err)

		assert.Equal(t, 1, len(tasks))
		assert.Equal(t, tasks[0].File, "README.md")
	})
}

func projectDir() billy.Filesystem {
	_, b, _, _ := runtime.Caller(0)
	root := filepath.Join(
		filepath.Dir(b),
		"test_project",
	)

	return osfs.New(root)
}

func convertFilePath(p string) string {
	return strings.ReplaceAll(p, "/", string(filepath.Separator))
}

func convertLine(p string) string {
	if runtime.GOOS == "windows" {
		p += "\r"
	}

	return p
}
