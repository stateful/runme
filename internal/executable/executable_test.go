package executable

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_executable(t *testing.T) {
	execPath := GetRunmeExecutablePath()
	runme, err := exec.LookPath(execPath)

	require.NoError(t, err)

	cmd := exec.Command(runme, "--version")

	buf, buferr := new(bytes.Buffer), new(bytes.Buffer)

	cmd.Stdout = buf
	cmd.Stderr = buferr

	err = cmd.Run()
	assert.NoError(t, err)
	assert.Equal(t, buf.String(), "runme version 0.0.0 (unknown) on unknown\n")
	assert.Empty(t, buferr.String())
}
