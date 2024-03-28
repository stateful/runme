package term

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystem(t *testing.T) {
	term := System()
	require.NotNil(t, term)
	require.Equal(t, os.Stdin, term.In())
	require.Equal(t, os.Stdout, term.Out())
	require.Equal(t, os.Stderr, term.ErrOut())
}

func TestFromIO(t *testing.T) {
	// Using system IO should work.
	term := FromIO(os.Stdin, os.Stdout, os.Stderr)
	require.NotNil(t, term)
	require.Equal(t, os.Stdin.Fd(), term.In().(*os.File).Fd())
	require.Equal(t, os.Stdout.Fd(), term.Out().(*os.File).Fd())
	require.Equal(t, os.Stderr.Fd(), term.ErrOut().(*os.File).Fd())

	// Using readers and writers should work too.
	term = FromIO(new(bytes.Buffer), new(bytes.Buffer), nil)
	require.NotNil(t, term)
	require.NotNil(t, term.In())
	require.NotNil(t, term.Out())
	require.Nil(t, term.ErrOut())
}

func TestTerm_TTY(t *testing.T) {
	term := System()
	require.Equal(t, isTerminal(os.Stdout), term.IsTTY())
}

func TestTerm_Size(t *testing.T) {
	term := System()
	if term.IsTTY() {
		w, h, err := term.Size()
		require.NoError(t, err)
		require.Greater(t, w, 0)
		require.Greater(t, h, 0)
	}
}
