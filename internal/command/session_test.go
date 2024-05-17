package command

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestSessionList(t *testing.T) {
	l, err := NewSessionList()
	require.NoError(t, err)

	var g errgroup.Group

	for i := 0; i < 10; i++ {
		g.Go(func() error {
			s := NewSession()
			l.Add(s)
			return nil
		})
	}

	require.NoError(t, g.Wait())
	require.Len(t, l.items.Keys(), 10)

	s := NewSession()
	l.Add(s)

	newest, ok := l.Newest()
	require.True(t, ok)
	require.Equal(t, s.ID, newest.ID)
}
