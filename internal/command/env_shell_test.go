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

	err := setOnShell(buf, envDumpCommand, false, false, false, "prePath", "postPath")
	require.NoError(t, err)

	expected := (envDumpCommand + " > prePath\n" +
		"__cleanup() {\n" +
		"rv=$?\n" +
		envDumpCommand + " > postPath\n" +
		"exit $rv\n}\n" +
		"trap -- \"__cleanup\" EXIT\n")

	require.EqualValues(t, expected, buf.String())
}

func TestSetOnShell_SkipShellHistory(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)

	err := setOnShell(buf, envDumpCommand, true, false, false, "prePath", "postPath")
	require.NoError(t, err)

	expected := (" " + envDumpCommand + " > prePath\n" +
		" __cleanup() {\n" +
		"rv=$?\n" +
		envDumpCommand + " > postPath\n" +
		"exit $rv\n}\n" +
		" trap -- \"__cleanup\" EXIT\n")

	require.EqualValues(t, expected, buf.String())
}
