package main

import (
	"fmt"
	"os"

	"github.com/stateful/runme/internal/cmd"
	"github.com/stateful/runme/internal/version"
)

func root() int {
	root := cmd.Root()
	root.Version = fmt.Sprintf("%s (%s) on %s", version.BuildVersion, version.Commit, version.BuildDate)
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	return 0
}

func main() {
	os.Exit(root())
}
