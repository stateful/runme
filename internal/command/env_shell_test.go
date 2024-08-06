//go:build !windows
// +build !windows

package command

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetOnShell(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)

	err := setOnShell(buf, "prePath", "postPath")
	require.NoError(t, err)

	expected := " " +
		envDumpCommand +
		" > prePath\n __cleanup() {\nrv=$?\n" +
		envDumpCommand +
		" > postPath\nexit $rv\n}\n trap -- \"__cleanup\" EXIT\n"

	require.EqualValues(t, expected, buf.String())
}
