package command

import (
	"io"
)

const StoreStdoutEnvName = "__"

func CreateEnv(key, value string) string {
	return createEnv(key, value)
}

func createEnv(key, value string) string {
	return key + "=" + value
}

type FileBasedEnvSetter struct {
	dumpCommand string
	prePath     string
	postPath    string
}

func NewFileBasedEnvSetter(prePath, postPath string) *FileBasedEnvSetter {
	return &FileBasedEnvSetter{
		dumpCommand: envDumpCommand,
		prePath:     prePath,
		postPath:    postPath,
	}
}

func (s *FileBasedEnvSetter) SetOnShell(shell io.Writer) error {
	return setOnShell(shell, s.dumpCommand, true, s.prePath, s.postPath)
}

func setOnShell(
	shell io.Writer,
	dumpCommand string,
	skipShellHistory bool,
	prePath string,
	postPath string,
) error {
	prefix := ""
	if skipShellHistory {
		// Prefix commands with a space to avoid polluting the shell history.
		prefix = " "
	}

	w := bulkWriter{Writer: shell}

	// First, dump all env at the beginning, so that a diff can be calculated.
	w.Write([]byte(prefix + dumpCommand + " > " + prePath + "\n"))
	// Then, set a trap on EXIT to dump all env at the end.
	w.Write([]byte(prefix + "__cleanup() {\nrv=$?\n" + (envDumpCommand + " > " + postPath) + "\nexit $rv\n}\n"))
	w.Write([]byte(prefix + "trap -- \"__cleanup\" EXIT\n"))

	_, err := w.Done()
	return err
}
