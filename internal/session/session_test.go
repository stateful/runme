package session

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestSessionList(t *testing.T) {
	seedEnv := os.Environ()

	t.Run("ConcurrentAdd", func(t *testing.T) {
		l := NewSessionList()

		var g errgroup.Group

		for i := 0; i < 10; i++ {
			g.Go(func() error {
				s, err := New(WithSeedEnv(seedEnv))
				require.NoError(t, err)
				err = l.Add(s)
				require.NoError(t, err)
				return nil
			})
		}

		require.NoError(t, g.Wait())
		require.Equal(t, l.Size(), 10)
	})

	t.Run("AddAndRetrieveNewest", func(t *testing.T) {
		l := NewSessionList()

		s, err := New(WithSeedEnv(seedEnv))
		require.NoError(t, err)
		err = l.Add(s)
		require.NoError(t, err)

		newest, ok := l.Newest()
		require.True(t, ok)
		require.Equal(t, s.ID, newest.ID)
	})

	t.Run("EvictOnOverflow", func(t *testing.T) {
		l := NewSessionList()

		for i := 0; i < SessionListCapacity+10; i++ {
			s, err := New(WithSeedEnv(seedEnv))
			require.NoError(t, err)
			err = l.Add(s)
			require.NoError(t, err)
		}

		require.Equal(t, l.Size(), SessionListCapacity)
	})
}
