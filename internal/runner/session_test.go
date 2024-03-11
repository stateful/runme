package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func Test_SessionList(t *testing.T) {
	t.Parallel()

	createSession := func() (*Session, error) {
		return NewSession(nil, nil, zap.NewNop())
	}

	t.Run("UpdatedOnCreate", func(t *testing.T) {
		list, err := NewSessionList()
		require.NoError(t, err)

		session1, err := createSession()
		require.NoError(t, err)
		list.AddSession(session1)

		session2, err := createSession()
		require.NoError(t, err)
		list.AddSession(session2)

		mostRecent, ok := list.MostRecent()
		require.Equal(t, true, ok)
		assert.Equal(t, session2.ID, mostRecent.ID)
	})

	t.Run("GetSession", func(t *testing.T) {
		list, err := NewSessionList()
		require.NoError(t, err)

		session1, err := createSession()
		require.NoError(t, err)
		list.AddSession(session1)

		session2, err := createSession()
		require.NoError(t, err)

		assert.NotEqual(t, session1.ID, session2.ID)

		list.AddSession(session2)

		found, ok := list.GetSession(session1.ID)
		require.Equal(t, true, ok)
		assert.Equal(t, session1, found)

		mostRecent, ok := list.MostRecent()
		require.Equal(t, true, ok)
		assert.Equal(t, session1.ID, mostRecent.ID)
	})

	t.Run("CreateAndAddSession", func(t *testing.T) {
		list, err := NewSessionList()
		require.NoError(t, err)

		session1, err := list.CreateAndAddSession(createSession)
		require.NoError(t, err)

		session2, err := list.CreateAndAddSession(createSession)
		require.NoError(t, err)

		assert.NotEqual(t, session1.ID, session2.ID)

		sessions, err := list.ListSessions()
		require.NoError(t, err)

		expected := []string{session1.ID, session2.ID}
		actual := []string{}

		for _, session := range sessions {
			actual = append(actual, session.ID)
		}

		assert.Equal(t, expected, actual)
	})

	t.Run("DeleteSession", func(t *testing.T) {
		list, err := NewSessionList()
		require.NoError(t, err)

		session1, err := list.CreateAndAddSession(createSession)
		require.NoError(t, err)

		session2, err := list.CreateAndAddSession(createSession)
		require.NoError(t, err)

		assert.NotEqual(t, session1.ID, session2.ID)

		{
			sessionList, err := list.ListSessions()
			require.NoError(t, err)
			assert.Equal(t, 2, len(sessionList))
		}

		deleted := list.DeleteSession(session2.ID)
		require.Equal(t, true, deleted)

		{
			sessionList, err := list.ListSessions()
			require.NoError(t, err)
			assert.Equal(t, 1, len(sessionList))

			expected := []string{session1.ID}
			actual := []string{}

			for _, session := range sessionList {
				actual = append(actual, session.ID)
			}

			assert.Equal(t, expected, actual)
		}

		deleted = list.DeleteSession(session1.ID)
		require.Equal(t, true, deleted)

		{
			sessionList, err := list.ListSessions()
			require.NoError(t, err)
			assert.Equal(t, 0, len(sessionList))
		}

		list.DeleteSession(session2.ID)
	})
}
