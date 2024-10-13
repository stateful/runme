//go:build !windows
// +build !windows

package command

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScriptEnvSetter(t *testing.T) {
	t.Parallel()

	prePath := "/tmp/pre-path"
	postPath := "/tmp/post-path"

	t.Run("WithDebug", func(t *testing.T) {
		setter := NewScriptEnvSetter(prePath, postPath, true)
		buf := new(bytes.Buffer)

		err := setter.SetOnShell(buf)
		require.NoError(t, err)

		expected := "#!/bin/sh\n" +
			"set -euxo pipefail\n" +
			"env -0 > /tmp/pre-path\n" +
			"__cleanup() {\nrv=$?\nenv -0 > /tmp/post-path\nexit $rv\n}\n" +
			"trap -- \"__cleanup\" EXIT\n" +
			"set +euxo pipefail\n"
		require.EqualValues(t, expected, buf.String())
	})

	t.Run("WithoutDebug", func(t *testing.T) {
		setter := NewScriptEnvSetter(prePath, postPath, false)
		buf := new(bytes.Buffer)

		err := setter.SetOnShell(buf)
		require.NoError(t, err)

		expected := "#!/bin/sh\n" +
			"env -0 > /tmp/pre-path\n" +
			"__cleanup() {\nrv=$?\nenv -0 > /tmp/post-path\nexit $rv\n}\n" +
			"trap -- \"__cleanup\" EXIT\n"
		require.EqualValues(t, expected, buf.String())
	})
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
