package runnerv2service

import "github.com/runmedev/runme/v3/internal/command"

func init() {
	// SetEnvDumpCommandForTesting overrides the default command that dumps the environment variables.
	// Without this line, running tests results in a fork bomb.
	// More: https://github.com/runmedev/runme/issues/730
	command.SetEnvDumpCommandForTesting()
}
