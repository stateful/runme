package command

import (
	"bytes"
	"io"
)

func setOnShell(shell io.Writer, message []byte, prePath, postPath string) error {
	var err error
	// First, dump all env at the beginning, so that a diff can be calculated.
	_, err = shell.Write([]byte(" " + envDumpCommand + " > " + prePath + "\n"))
	if err != nil {
		return err
	}
	// Then, set a trap on EXIT to dump all env at the end.
	_, err = shell.Write(bytes.Join(
		[][]byte{
			[]byte(" __cleanup() {\nrv=$?\n" + (envDumpCommand + " > " + postPath) + "\nexit $rv\n}"),
			[]byte(" trap -- \"__cleanup\" EXIT"),
			nil, // add a new line at the end
		},
		[]byte{'\n'},
	))
	if err != nil {
		return err
	}
	_, err = shell.Write([]byte(" clear\n"))
	if err != nil {
		return err
	}
	_, err = shell.Write(message)
	return err
}
