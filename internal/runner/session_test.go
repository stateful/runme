package runner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/pkg/project"
	"github.com/stateful/runme/v3/pkg/project/teststub"
)

func Test_SessionList(t *testing.T) {
	t.Parallel()

	createSession := func() (*Session, error) {
		return NewSession(nil, nil, zap.NewNop())
	}

	t.Run("UpdatedOnCreate", func(t *testing.T) {
		list := NewSessionList()

		session1, err := createSession()
		require.NoError(t, err)
		err = list.Add(session1)
		require.NoError(t, err)

		session2, err := createSession()
		require.NoError(t, err)
		err = list.Add(session2)
		require.NoError(t, err)

		mostRecent, ok := list.MostRecent()
		require.Equal(t, true, ok)
		assert.Equal(t, session2.ID, mostRecent.ID)
	})

	t.Run("GetSession", func(t *testing.T) {
		list := NewSessionList()

		session1, err := createSession()
		require.NoError(t, err)
		err = list.Add(session1)
		require.NoError(t, err)

		session2, err := createSession()
		require.NoError(t, err)

		assert.NotEqual(t, session1.ID, session2.ID)

		err = list.Add(session2)
		require.NoError(t, err)

		found, ok := list.Get(session1)
		require.Equal(t, true, ok)
		assert.Equal(t, session1, found)

		mostRecent, ok := list.MostRecent()
		require.Equal(t, true, ok)
		assert.Equal(t, session1.ID, mostRecent.ID)
	})

	t.Run("CreateAndAddEntry", func(t *testing.T) {
		list := NewSessionList()

		session1, err := list.CreateAndAdd(createSession)
		require.NoError(t, err)

		session2, err := list.CreateAndAdd(createSession)
		require.NoError(t, err)

		assert.NotEqual(t, session1.ID, session2.ID)

		sessions := list.List()

		expected := []string{session1.ID, session2.ID}
		actual := []string{}

		for _, session := range sessions {
			actual = append(actual, session.ID)
		}

		assert.Equal(t, expected, actual)
	})

	t.Run("DeleteEntry", func(t *testing.T) {
		list := NewSessionList()

		session1, err := list.CreateAndAdd(createSession)
		require.NoError(t, err)

		session2, err := list.CreateAndAdd(createSession)
		require.NoError(t, err)

		assert.NotEqual(t, session1.ID, session2.ID)

		{
			sessionList := list.List()
			assert.Equal(t, 2, len(sessionList))
		}

		deleted := list.Delete(session2)
		require.Equal(t, true, deleted)

		{
			sessionList := list.List()
			assert.Equal(t, 1, len(sessionList))

			expected := []string{session1.ID}
			actual := []string{}

			for _, session := range sessionList {
				actual = append(actual, session.ID)
			}

			assert.Equal(t, expected, actual)
		}

		deleted = list.Delete(session1)
		require.Equal(t, true, deleted)

		{
			sessionList := list.List()
			assert.Equal(t, 0, len(sessionList))
		}

		list.Delete(session2)
	})
}

func Test_EnvDirEnv(t *testing.T) {
	temp := t.TempDir()
	testData := teststub.Setup(t, temp)

	proj, err := project.NewDirProject(testData.DirEnvProjectPath(), project.WithEnvDirEnv(true))
	require.NoError(t, err)

	sess, err := NewSession([]string{}, proj, zap.NewNop())
	require.NoError(t, err)

	msg, err := sess.LoadDirEnv(context.Background(), proj, proj.Root())
	require.NoError(t, err)
	require.Contains(t, msg, "direnv: export +PGDATABASE +PGHOST +PGOPTIONS +PGPASSWORD +PGPORT +PGUSER")

	actualEnvs, err := sess.Envs()
	require.NoError(t, err)

	expectedEnvs := []string{
		"PGDATABASE=platform",
		"PGHOST=127.0.0.1",
		"PGOPTIONS=--search_path=account,public",
		"PGPASSWORD=postgres",
		"PGPORT=15430",
		"PGUSER=postgres",
	}
	for _, env := range expectedEnvs {
		require.Contains(t, actualEnvs, env)
	}
}
