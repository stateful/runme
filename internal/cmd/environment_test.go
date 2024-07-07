package cmd

import (
	"bytes"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stretchr/testify/assert"
)

func Test_cmdEnvironment_Dump(t *testing.T) {
	t.Parallel()

	env := []string{
		"A=1",
		"B=2",
		"C=3",
	}

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)

	cmd := runEnvCommand()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = env

	original := newOSEnvironReader
	defer func() { newOSEnvironReader = original }()
	newOSEnvironReader = func() (io.Reader, error) {
		return strings.NewReader(command.EnvSliceToString(env)), nil
	}

	err := cmd.Run()
	assert.NoError(t, err)
	assert.Equal(t, "", stderr.String())
	assert.Equal(t, true, strings.HasPrefix(stdout.String(), "A=1\x00B=2\x00C=3\x00"))
}

func runEnvCommand(...string) *exec.Cmd {
	return exec.Command("env", "-0")
}
