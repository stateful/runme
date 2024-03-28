package cmd

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func runEnvCommand(...string) *exec.Cmd {
	return exec.Command("env", "-0")
}

func Test_cmdEnvironment(t *testing.T) {
	t.Run("Dump", func(t *testing.T) {
		t.Parallel()

		cmd := runEnvCommand()

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

		old := osEnviron
		defer func() { osEnviron = old }()

		osEnviron = func() []string {
			return env
		}

		assert.NoError(t, err)
		assert.Equal(t, "", stderr.String())
		assert.Equal(t, true, strings.HasPrefix(stdout.String(), "A=1\x00B=2\x00C=3\x00"))
		assert.Equal(t, "A=1\x00B=2\x00C=3", getDumpedEnvironment())
	})
}
