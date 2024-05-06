package command

import (
	"testing"

	runnerv2alpha1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/runner/v2alpha1"
)

func TestInlineCommand(t *testing.T) {
	t.Parallel()

	t.Run("Echo", func(t *testing.T) {
		t.Parallel()

		cfg := &ProgramConfig{
			ProgramName: "echo",
			Arguments:   []string{"-n", "test"},
			Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
		}

		testExecuteCommand(t, cfg, nil, "test", "")
	})

	t.Run("EchoInteractive", func(t *testing.T) {
		t.Parallel()

		cfg := &ProgramConfig{
			ProgramName: "echo",
			Arguments:   []string{"-n", "test"},
			Interactive: true,
			Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
		}

		testExecuteCommand(t, cfg, nil, "test", "")
	})
}
