package project

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDirProject(t *testing.T) {
	testdataDir := testdataDir()

	t.Run("ProperDirProject", func(t *testing.T) {
		projectDir := filepath.Join(testdataDir, "dir-project")
		_, err := NewDirProject(projectDir)
		require.NoError(t, err)
	})

	t.Run("ProperGitProject", func(t *testing.T) {
		// git-based project is also a dir-based project.
		gitProjectDir := filepath.Join(testdataDir, "git-project")
		_, err := NewDirProject(gitProjectDir)
		require.NoError(t, err)
	})

	t.Run("UnknownDir", func(t *testing.T) {
		unknownDir := filepath.Join(testdataDir, "unknown-project")
		_, err := NewDirProject(unknownDir)
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestNewFileProject(t *testing.T) {
	testdataDir := testdataDir()

	t.Run("UnknownFile", func(t *testing.T) {
		fileProject := filepath.Join(testdataDir, "unknown-file.md")
		_, err := NewFileProject(fileProject)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("ProperFileProject", func(t *testing.T) {
		fileProject := filepath.Join(testdataDir, "file-project.md")
		_, err := NewFileProject(fileProject)
		require.NoError(t, err)
	})
}

func TestProjectLoad(t *testing.T) {
	testdataDir := testdataDir()
	gitProjectDir := filepath.Join(testdataDir, "git-project")

	t.Run("GitProject", func(t *testing.T) {
		p, err := NewDirProject(gitProjectDir, WithFindRepoUpward())
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
			LoadEventFoundFile, // "readme.md"
			LoadEventFinishedWalk,
			LoadEventStartedParsingDocument,  // "git-ignored.md"
			LoadEventFinishedParsingDocument, // "git-ignored.md"
			LoadEventFoundTask,
			LoadEventStartedParsingDocument,  // "ignored.md"
			LoadEventFinishedParsingDocument, // "ignored.md"
			LoadEventFoundTask,
			LoadEventStartedParsingDocument,  // "readme.md"
			LoadEventFinishedParsingDocument, // "readme.md"
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
				Type: LoadEventFoundDir,
				Data: gitProjectDir,
			},
			events[1],
		)
		assert.Equal(
			t,
			LoadEvent{
				Type: LoadEventFoundFile,
				Data: filepath.Join(gitProjectDir, "git-ignored.md"),
			},
			events[2],
		)
		assert.Equal(
			t,
			LoadEvent{
				Type: LoadEventFoundFile,
				Data: filepath.Join(gitProjectDir, "ignored.md"),
			},
			events[3],
		)
		assert.Equal(
			t,
			LoadEvent{
				Type: LoadEventFoundFile,
				Data: filepath.Join(gitProjectDir, "readme.md"),
			},
			events[4],
		)
		assert.Equal(
			t,
			filepath.Join(gitProjectDir, "git-ignored.md"),
			dataFromLoadEvent[CodeBlock](events[8]).Filename,
		)
		assert.Equal(
			t,
			filepath.Join(gitProjectDir, "ignored.md"),
			dataFromLoadEvent[CodeBlock](events[11]).Filename,
		)
		assert.Equal(
			t,
			filepath.Join(gitProjectDir, "readme.md"),
			dataFromLoadEvent[CodeBlock](events[14]).Filename,
		)
	})

	t.Run("DirProjectWithRespectGitignoreAndIgnorePatterns", func(t *testing.T) {
		p, err := NewDirProject(
			gitProjectDir,
			WithFindRepoUpward(),
			WithRespectGitignore(),
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
			LoadEventFinishedWalk,
			LoadEventStartedParsingDocument,  // "readme.md"
			LoadEventFinishedParsingDocument, // "readme.md"
			LoadEventFoundTask,
		}
		require.EqualValues(
			t,
			expectedEvents,
			mapLoadEvents(events, func(le LoadEvent) LoadEventType { return le.Type }),
		)
	})

	projectDir := filepath.Join(testdataDir, "dir-project")

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
			LoadEventFoundFile, // "git-ignored.md"
			LoadEventFoundFile, // "ignored.md"
			LoadEventFoundFile, // "readme.md"
			LoadEventFinishedWalk,
			LoadEventStartedParsingDocument,  // "git-ignored.md"
			LoadEventFinishedParsingDocument, // "git-ignored.md"
			LoadEventFoundTask,
			LoadEventStartedParsingDocument,  // "ignored.md"
			LoadEventFinishedParsingDocument, // "ignored.md"
			LoadEventFoundTask,
			LoadEventStartedParsingDocument,  // "readme.md"
			LoadEventFinishedParsingDocument, // "readme.md"
			LoadEventFoundTask,
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
			WithRespectGitignore(),
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
			LoadEventFinishedWalk,
			LoadEventStartedParsingDocument,  // "readme.md"
			LoadEventFinishedParsingDocument, // "readme.md"
			LoadEventFoundTask,
		}
		require.EqualValues(
			t,
			expectedEvents,
			mapLoadEvents(events, func(le LoadEvent) LoadEventType { return le.Type }),
		)
	})

	fileProject := filepath.Join(testdataDir, "file-project.md")

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
				Data: fileProject,
			},
			events[1],
		)
		assert.Equal(
			t,
			fileProject,
			dataFromLoadEvent[CodeBlock](events[5]).Filename,
		)
	})
}

// TODO(adamb): a better approach is to store "testdata" during build time.
func testdataDir() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Join(
		filepath.Dir(b),
		"testdata",
	)
}

func mapLoadEvents[T any](events []LoadEvent, fn func(LoadEvent) T) []T {
	result := make([]T, 0, len(events))

	for _, e := range events {
		result = append(result, fn(e))
	}

	return result
}

func dataFromLoadEvent[T any](e LoadEvent) T {
	return e.Data.(T)
}
