//go:build !windows
// +build !windows

package runnerv2client

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/runmedev/runme/v3/internal/command"
	"github.com/runmedev/runme/v3/internal/testutils/runnerservice"
	runnerv2 "github.com/runmedev/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

func TestClient_ExecuteProgram_InputInteractive(t *testing.T) {
	t.Parallel()

	lis, stop := runnerservice.New(t)
	t.Cleanup(stop)

	client := createClient(t, lis)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	cfg := &command.ProgramConfig{
		ProgramName: "bash",
		Source: &runnerv2.ProgramConfig_Commands{
			Commands: &runnerv2.ProgramConfig_CommandList{
				Items: []string{
					"read -r name",
					"echo $name",
				},
			},
		},
		Interactive: true,
		Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
	}
	stdout := new(bytes.Buffer)
	err := client.ExecuteProgram(
		ctx,
		cfg,
		ExecuteProgramOptions{
			Stdin:  io.NopCloser(bytes.NewBufferString("test-input-interactive\n")),
			Stdout: stdout,
		},
	)
	require.NoError(t, err)
	// Using [require.Contains] because on Linux the input is repeated.
	// Unclear why it passes fine on macOS.
	require.Contains(t, stdout.String(), "test-input-interactive\r\n")
}
