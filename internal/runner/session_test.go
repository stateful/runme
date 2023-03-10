package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SessionList(t *testing.T) {
	t.Parallel()

	createSession := func() *Session {
		return NewSession(nil, nil)
	}

	t.Run("UpdatedOnCreate", func(t *testing.T) {
		list, err := NewSessionList()
		require.NoError(t, err)

		session1 := createSession()
		list.AddSession(session1)

		session2 := createSession()
		list.AddSession(session2)

		mostRecent, ok := list.MostRecent()
		require.Equal(t, true, ok)
		assert.Equal(t, session2.ID, mostRecent.ID)
	})

	t.Run("GetSession", func(t *testing.T) {
		list, err := NewSessionList()
		require.NoError(t, err)

		session1 := createSession()
		list.AddSession(session1)

		session2 := createSession()
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

		session1 := list.CreateAndAddSession(createSession)
		session2 := list.CreateAndAddSession(createSession)

		sessions, err := list.ListSessions()
		require.NoError(t, err)

		assert.Equal(t, []*Session{session1, session2}, sessions)
	})

	t.Run("DeleteSession", func(t *testing.T) {
		list, err := NewSessionList()
		require.NoError(t, err)

		session1 := list.CreateAndAddSession(createSession)
		session2 := list.CreateAndAddSession(createSession)

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
			assert.Equal(t, session1, sessionList[0])
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
