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
		list := newSessionList()

		session1, err := createSession()
		require.NoError(t, err)
		err = list.Add(session1)
		require.NoError(t, err)

		session2, err := createSession()
		require.NoError(t, err)
		err = list.Add(session2)
		require.NoError(t, err)

		mostRecent, ok := list.Newest()
		require.Equal(t, true, ok)
		assert.Equal(t, session2.ID, mostRecent.ID)
	})

	t.Run("GetSession", func(t *testing.T) {
		list := newSessionList()

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

		newest, ok := list.Newest()
		require.Equal(t, true, ok)
		assert.Equal(t, session1.ID, newest.ID)
	})

	t.Run("CreateAndAddEntry", func(t *testing.T) {
		list := newSessionList()

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
		list := newSessionList()

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
