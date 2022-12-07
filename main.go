package main

import (
	"fmt"
	"os"

	"github.com/stateful/runme/internal/cmd"
)

// These are variables so that they can be set during the build time.
var (
	BuildDate    = "unknown"
	BuildVersion = "0.0.0"
	Commit       = "unknown"
)

func root() int {
	root := cmd.Root()
	root.Version = fmt.Sprintf("stateful %s (%s) on %s", BuildVersion, Commit, BuildDate)
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	return 0
}

func main() {
	os.Exit(root())
}
