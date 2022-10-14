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

func main() {
	root := cmd.Root()
	root.Version = fmt.Sprintf("stateful %s (%s) on %s", BuildVersion, Commit, BuildDate)

	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}
