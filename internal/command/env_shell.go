package command

import (
	"bytes"
	"io"
)

func setOnShell(shell io.Writer, message, prePath, postPath string) error {
	var err error
	// Prefix commands with a space to avoid polluting the shell history.
	skipShellHistory := " "
	// First, dump all env at the beginning, so that a diff can be calculated.
	_, err = shell.Write([]byte(skipShellHistory + envDumpCommand + " > " + prePath + "\n"))
	if err != nil {
		return err
	}
	// Then, set a trap on EXIT to dump all env at the end.
	_, err = shell.Write(bytes.Join(
		[][]byte{
			[]byte(skipShellHistory + "__cleanup() {\nrv=$?\n" + (envDumpCommand + " > " + postPath) + "\nexit $rv\n}"),
			[]byte(skipShellHistory + "trap -- \"__cleanup\" EXIT"),
			nil, // add a new line at the end
		},
		[]byte{'\n'},
	))
	if err != nil {
		return err
	}
	_, err = shell.Write([]byte(skipShellHistory + "clear\n"))
	if err != nil {
		return err
	}

	if len(message) == 0 {
		return nil
	}

	_, err = shell.Write([]byte(skipShellHistory + message))
	return err
}
