//go:build !windows
// +build !windows

package command

import (
	"testing"

	"github.com/stretchr/testify/assert"

	runnerv2alpha1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2alpha1"
)

func TestInlineShellCommand_CollectEnv(t *testing.T) {
	t.Parallel()

	t.Run("Fifo", func(t *testing.T) {
		envCollectorUseFifo = true
		testInlineShellCommandCollectEnv(t)
	})

	t.Run("NonFifo", func(t *testing.T) {
		envCollectorUseFifo = false
		testInlineShellCommandCollectEnv(t)
	})
}

func testInlineShellCommandCollectEnv(t *testing.T) {
	t.Helper()

	cfg := &ProgramConfig{
		ProgramName: "bash",
		Source: &runnerv2alpha1.ProgramConfig_Script{
			Script: "export TEST_ENV=1",
		},
		Mode: runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
	}
	sess := NewSession()

	testExecuteCommandWithSession(t, cfg, sess, nil, "", "")

	got, ok := sess.GetEnv("TEST_ENV")
	assert.True(t, ok)
	assert.Equal(t, "1", got)
}
