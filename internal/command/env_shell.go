package command

import "io"

func setupEnvCollectionInShell(shell io.Writer, prePath, postPath string) error {
	var err error
	// First, dump all env at the beginning, so that a diff can be calculated.
	_, err = shell.Write([]byte(EnvDumpCommand + " > " + prePath + "\n"))
	if err != nil {
		return err
	}
	// Then, set a trap on EXIT to dump all env at the end.
	_, err = setTrap(shell, (EnvDumpCommand + " > " + postPath + "\n"))
	return err
}

func setTrap(w io.Writer, cmd string) (int, error) {
	bw := bulkWriter{Writer: w}
	bw.Write([]byte("__cleanup() {\nrv=$?\n" + cmd + "\nexit $rv\n}\n"))
	bw.Write([]byte("trap -- \"__cleanup\" EXIT\n"))
	return bw.Done()
}
