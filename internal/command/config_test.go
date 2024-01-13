package command

import (
	"testing"

	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
	"github.com/stretchr/testify/require"
)

func TestNormalizeConfigForDryRun(t *testing.T) {
	cfg := &Config{
		ProgramName: "echo",
		Arguments:   []string{"-n", "test"},
		Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
	}
	cfg, err := NormalizeConfigForDryRun(cfg)
	require.NoError(t, err)
	require.Equal(t, "/bin/echo", cfg.ProgramName)
	require.Equal(t, []string{"-n", "test"}, cfg.Arguments)

	// TODO(adamb): add more cases: bash inline, bash file, python file, etc.
}
