package cmd

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/stateful/runme/internal/executable"
	"github.com/stretchr/testify/assert"
)

func runEnvCommand(arg ...string) *exec.Cmd {
	args := []string{"env"}
	args = append(args, arg...)

	return exec.Command(executable.GetRunmeExecutablePath(), args...)
}

func Test_cmdEnvironment(t *testing.T) {
	t.Run("Dump", func(t *testing.T) {
		t.Parallel()

		cmd := runEnvCommand("dump")

		stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)

		cmd.Stdout = stdout
		cmd.Stderr = stderr

		env := []string{
			"A=1",
			"B=2",
			"C=3",
		}

		cmd.Env = env

		err := cmd.Run()

		assert.NoError(t, err)
		assert.Equal(t, "", stderr.String())
		assert.Equal(t, strings.Join(env, "\000"), stdout.String())
	})
}
