package command

import "io"

func buildShellEnvCollector(w io.Writer) (shellEnvCollector, error) {
	return newFileShellEnvCollector(w)
}
