//go:build !windows

package command

import (
	"testing"

	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

func TestInlineCommand_Interactive(t *testing.T) {
	t.Parallel()
	cfg := &ProgramConfig{
		ProgramName: "echo",
		Arguments:   []string{"-n", "test"},
		Interactive: true,
		Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
	}
	testExecuteCommand(t, cfg, nil, "test", "")
}
