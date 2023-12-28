package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stateful/runme/internal/project/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractDataFromLoadEvent(t *testing.T) {
	t.Run("MatchingTypes", func(t *testing.T) {
		event := LoadEvent{
			Type: LoadEventFoundDir,
			Data: LoadEventFoundDirData{
				Path: "/some/path",
			},
		}

		data := ExtractDataFromLoadEvent[LoadEventFoundDirData](event)
		assert.Equal(t, "/some/path", data.Path)
	})

	t.Run("NotMatchingTypes", func(t *testing.T) {
		event := LoadEvent{
			Type: LoadEventFoundDir,
			Data: LoadEventFoundDirData{
				Path: "/some/path",
			},
		}

		require.Panics(t, func() {
			ExtractDataFromLoadEvent[LoadEventStartedWalkData](event)
		})
	})
}

func TestMain(m *testing.M) {
	testdata.PrepareGitProject()
	defer testdata.CleanupGitProject()

	code := m.Run()
	os.Exit(code)
}

func TestNewDirProject(t *testing.T) {
	t.Run("ProperDirProject", func(t *testing.T) {
		projectDir := testdata.DirProjectPath()
		_, err := NewDirProject(projectDir)
		require.NoError(t, err)
	})

	t.Run("ProperGitProject", func(t *testing.T) {
		// git-based project is also a dir-based project.
		gitProjectDir := testdata.GitProjectPath()
		_, err := NewDirProject(gitProjectDir)
		require.NoError(t, err)
	})

	t.Run("UnknownDir", func(t *testing.T) {
		testdataDir := testdata.TestdataPath()
		unknownDir := filepath.Join(testdataDir, "unknown-project")
		_, err := NewDirProject(unknownDir)
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestNewFileProject(t *testing.T) {
	t.Run("UnknownFile", func(t *testing.T) {
		fileProject := filepath.Join(testdata.TestdataPath(), "unknown-file.md")
		_, err := NewFileProject(fileProject)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("NotAbsPath", func(t *testing.T) {
		_, err := NewFileProject("unknown-file.md")
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("ProperFileProject", func(t *testing.T) {
		fileProject := testdata.ProjectFilePath()
		_, err := NewFileProject(fileProject)
		require.NoError(t, err)
	})
}

func TestProjectRoot(t *testing.T) {
	t.Run("GitProject", func(t *testing.T) {
		gitProjectDir := testdata.GitProjectPath()
		p, err := NewDirProject(gitProjectDir)
		require.NoError(t, err)
		assert.Equal(t, gitProjectDir, p.Root())
	})

	t.Run("FileProject", func(t *testing.T) {
		fileProject := testdata.ProjectFilePath()
		p, err := NewFileProject(fileProject)
		require.NoError(t, err)
		assert.Equal(t, testdata.TestdataPath(), p.Root())
	})
}

func TestProjectLoad(t *testing.T) {
	gitProjectDir := testdata.GitProjectPath()

	t.Run("GitProject", func(t *testing.T) {
		p, err := NewDirProject(
			gitProjectDir,
			WithFindRepoUpward(),
			WithIgnoreFilePatterns(".git.bkp"),
			WithIgnoreFilePatterns(".gitignore.bkp"),
		)
		require.NoError(t, err)

		eventc := make(chan LoadEvent)

		events := make([]LoadEvent, 0)
		doneReadingEvents := make(chan struct{})
		go func() {
			defer close(doneReadingEvents)
			for e := range eventc {
				events = append(events, e)
			}
		}()

		p.Load(context.Background(), eventc, false)
		<-doneReadingEvents

		expectedEvents := []LoadEventType{
			LoadEventStartedWalk,
			LoadEventFoundDir,  // "."
			LoadEventFoundFile, // "git-ignored.md"
			LoadEventFoundFile, // "ignored.md"
			LoadEventFoundDir,  // "nested"
			LoadEventFoundFile, // "nested/git-ignored.md"
			LoadEventFoundFile, // "readme.md"
			LoadEventFinishedWalk,
			LoadEventStartedParsingDocument,  // "git-ignored.md"
			LoadEventFinishedParsingDocument, // "git-ignored.md"
			LoadEventFoundTask,
			LoadEventStartedParsingDocument,  // "nested/git-ignored.md"
			LoadEventFinishedParsingDocument, // "nested/git-ignored.md"
			LoadEventFoundTask,
			LoadEventStartedParsingDocument,  // "ignored.md"
			LoadEventFinishedParsingDocument, // "ignored.md"
			LoadEventFoundTask,
			LoadEventStartedParsingDocument,  // "readme.md"
			LoadEventFinishedParsingDocument, // "readme.md"
			LoadEventFoundTask,
			LoadEventFoundTask,
		}
		require.EqualValues(
			t,
			expectedEvents,
			mapLoadEvents(events, func(le LoadEvent) LoadEventType { return le.Type }),
			"collected events: %+v",
			events,
		)
		assert.Equal(
			t,
			LoadEvent{
				Type: LoadEventFoundDir,
				Data: LoadEventFoundDirData{Path: gitProjectDir},
			},
			events[1],
		)
		assert.Equal(
			t,
			LoadEvent{
				Type: LoadEventFoundFile,
				Data: LoadEventFoundFileData{Path: filepath.Join(gitProjectDir, "git-ignored.md")},
			},
			events[2],
		)
		assert.Equal(
			t,
			LoadEvent{
				Type: LoadEventFoundFile,
				Data: LoadEventFoundFileData{Path: filepath.Join(gitProjectDir, "ignored.md")},
			},
			events[3],
		)
		assert.Equal(
			t,
			LoadEvent{
				Type: LoadEventFoundDir,
				Data: LoadEventFoundDirData{Path: filepath.Join(gitProjectDir, "nested")},
			},
			events[4],
		)
		assert.Equal(
			t,
			LoadEvent{
				Type: LoadEventFoundFile,
				Data: LoadEventFoundFileData{Path: filepath.Join(gitProjectDir, "nested", "git-ignored.md")},
			},
			events[5],
		)
		assert.Equal(
			t,
			LoadEvent{
				Type: LoadEventFoundFile,
				Data: LoadEventFoundFileData{Path: filepath.Join(gitProjectDir, "readme.md")},
			},
			events[6],
		)
		assert.Equal(
			t,
			filepath.Join(gitProjectDir, "git-ignored.md"),
			ExtractDataFromLoadEvent[LoadEventFoundTaskData](events[10]).DocumentPath,
		)
		assert.Equal(
			t,
			filepath.Join(gitProjectDir, "ignored.md"),
			ExtractDataFromLoadEvent[LoadEventFoundTaskData](events[13]).DocumentPath,
		)
		assert.Equal(
			t,
			filepath.Join(gitProjectDir, "nested", "git-ignored.md"),
			ExtractDataFromLoadEvent[LoadEventFoundTaskData](events[16]).DocumentPath,
		)
		// Unnamed task
		{
			data := ExtractDataFromLoadEvent[LoadEventFoundTaskData](events[19])

			assert.Equal(t, filepath.Join(gitProjectDir, "readme.md"), data.DocumentPath)
			assert.Equal(t, "echo-hello", data.Name)
			assert.True(t, data.IsNameGenerated)
		}
		// Named task
		{
			data := ExtractDataFromLoadEvent[LoadEventFoundTaskData](events[20])

			assert.Equal(t, filepath.Join(gitProjectDir, "readme.md"), data.DocumentPath)
			assert.Equal(t, "my-task", data.Name)
			assert.False(t, data.IsNameGenerated)
		}
	})

	t.Run("DirProjectWithRespectGitignoreAndIgnorePatterns", func(t *testing.T) {
		p, err := NewDirProject(
			gitProjectDir,
			WithFindRepoUpward(),
			WithRespectGitignore(),
			WithIgnoreFilePatterns(".git.bkp"),
			WithIgnoreFilePatterns(".gitignore.bkp"),
			WithIgnoreFilePatterns("ignored.md"),
		)
		require.NoError(t, err)

		eventc := make(chan LoadEvent)

		events := make([]LoadEvent, 0)
		doneReadingEvents := make(chan struct{})
		go func() {
			defer close(doneReadingEvents)
			for e := range eventc {
				events = append(events, e)
			}
		}()

		p.Load(context.Background(), eventc, false)
		<-doneReadingEvents

		expectedEvents := []LoadEventType{
			LoadEventStartedWalk,
			LoadEventFoundDir,  // "."
			LoadEventFoundDir,  // "nested"
			LoadEventFoundFile, // "readme.md"
			LoadEventFinishedWalk,
			LoadEventStartedParsingDocument,  // "readme.md"
			LoadEventFinishedParsingDocument, // "readme.md"
			LoadEventFoundTask,               // unnamed; echo-hello
			LoadEventFoundTask,               // named; my-task
		}
		require.EqualValues(
			t,
			expectedEvents,
			mapLoadEvents(events, func(le LoadEvent) LoadEventType { return le.Type }),
			"found events: %#+v", events,
		)
	})

	projectDir := testdata.DirProjectPath()

	t.Run("DirProject", func(t *testing.T) {
		p, err := NewDirProject(projectDir)
		require.NoError(t, err)

		eventc := make(chan LoadEvent)

		events := make([]LoadEvent, 0)
		doneReadingEvents := make(chan struct{})
		go func() {
			defer close(doneReadingEvents)
			for e := range eventc {
				events = append(events, e)
			}
		}()

		p.Load(context.Background(), eventc, false)
		<-doneReadingEvents

		expectedEvents := []LoadEventType{
			LoadEventStartedWalk,
			LoadEventFoundDir,  // "."
			LoadEventFoundFile, // "ignored.md"
			LoadEventFoundFile, // "readme.md"
			LoadEventFoundFile, // "session-01HJS35FZ2K0JBWPVAXPMMVTGN.md"
			LoadEventFinishedWalk,
			LoadEventStartedParsingDocument,  // "ignored.md"
			LoadEventFinishedParsingDocument, // "ignored.md"
			LoadEventFoundTask,
			LoadEventStartedParsingDocument,  // "readme.md"
			LoadEventFinishedParsingDocument, // "readme.md"
			LoadEventFoundTask,               // unnamed; echo-hello
			LoadEventFoundTask,               // named; my-task
			LoadEventStartedParsingDocument,  // "session-01HJS35FZ2K0JBWPVAXPMMVTGN.md"
			LoadEventFinishedParsingDocument, // "session-01HJS35FZ2K0JBWPVAXPMMVTGN.md"
		}
		require.EqualValues(
			t,
			expectedEvents,
			mapLoadEvents(events, func(le LoadEvent) LoadEventType { return le.Type }),
		)
	})

	t.Run("DirProjectWithRespectGitignoreAndIgnorePatterns", func(t *testing.T) {
		p, err := NewDirProject(
			projectDir,
			WithIgnoreFilePatterns("ignored.md"),
		)
		require.NoError(t, err)

		eventc := make(chan LoadEvent)

		events := make([]LoadEvent, 0)
		doneReadingEvents := make(chan struct{})
		go func() {
			defer close(doneReadingEvents)
			for e := range eventc {
				events = append(events, e)
			}
		}()

		p.Load(context.Background(), eventc, false)
		<-doneReadingEvents

		expectedEvents := []LoadEventType{
			LoadEventStartedWalk,
			LoadEventFoundDir,  // "."
			LoadEventFoundFile, // "readme.md"
			LoadEventFoundFile, // "session-01HJS35FZ2K0JBWPVAXPMMVTGN.md"
			LoadEventFinishedWalk,
			LoadEventStartedParsingDocument,  // "readme.md"
			LoadEventFinishedParsingDocument, // "readme.md"
			LoadEventFoundTask,               // unnamed; echo-hello
			LoadEventFoundTask,               // named; my-task
			LoadEventStartedParsingDocument,  // "session-01HJS35FZ2K0JBWPVAXPMMVTGN.md"
			LoadEventFinishedParsingDocument, // "session-01HJS35FZ2K0JBWPVAXPMMVTGN.md"
		}
		require.EqualValues(
			t,
			expectedEvents,
			mapLoadEvents(events, func(le LoadEvent) LoadEventType { return le.Type }),
		)
	})

	fileProject := testdata.ProjectFilePath()

	t.Run("FileProject", func(t *testing.T) {
		p, err := NewFileProject(fileProject)
		require.NoError(t, err)

		eventc := make(chan LoadEvent)

		events := make([]LoadEvent, 0)
		doneReadingEvents := make(chan struct{})
		go func() {
			defer close(doneReadingEvents)
			for e := range eventc {
				events = append(events, e)
			}
		}()

		p.Load(context.Background(), eventc, false)
		<-doneReadingEvents

		expectedEvents := []LoadEventType{
			LoadEventStartedWalk,
			LoadEventFoundFile, // "file-project.md"
			LoadEventFinishedWalk,
			LoadEventStartedParsingDocument,  // "file-project.md"
			LoadEventFinishedParsingDocument, // "file-project.md"
			LoadEventFoundTask,
		}
		require.EqualValues(
			t,
			expectedEvents,
			mapLoadEvents(events, func(le LoadEvent) LoadEventType { return le.Type }),
		)

		assert.Equal(
			t,
			LoadEvent{
				Type: LoadEventFoundFile,
				Data: LoadEventFoundFileData{Path: fileProject},
			},
			events[1],
		)
		assert.Equal(
			t,
			fileProject,
			ExtractDataFromLoadEvent[LoadEventFoundTaskData](events[5]).DocumentPath,
		)
	})
}

func mapLoadEvents[T any](events []LoadEvent, fn func(LoadEvent) T) []T {
	result := make([]T, 0, len(events))

	for _, e := range events {
		result = append(result, fn(e))
	}

	return result
}
