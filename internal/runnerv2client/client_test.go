package runnerv2client

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/runnerv2service"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

func init() {
	command.SetEnvDumpCommand("env -0")
}

func TestClient_ExecuteProgram(t *testing.T) {
	t.Parallel()

	lis, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)

	t.Run("OutputWithSession", func(t *testing.T) {
		t.Parallel()

		client := testCreateClient(t, lis)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		sessionResp, err := client.CreateSession(
			ctx,
			&runnerv2.CreateSessionRequest{
				Env: []string{"TEST=test-output-with-session-env"},
			},
		)
		require.NoError(t, err)

		cfg := &command.ProgramConfig{
			ProgramName: "bash",
			Source: &runnerv2.ProgramConfig_Commands{
				Commands: &runnerv2.ProgramConfig_CommandList{
					Items: []string{
						"echo -n $TEST",
						">&2 echo -n test-output-with-session-stderr",
					},
				},
			},
			Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
		}
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		err = client.ExecuteProgram(
			ctx,
			cfg,
			ExecuteProgramOptions{
				SessionID: sessionResp.GetSession().GetId(),
				Stdout:    stdout,
				Stderr:    stderr,
			},
		)
		require.NoError(t, err)
		require.Equal(t, "test-output-with-session-env", stdout.String())
		require.Equal(t, "test-output-with-session-stderr", stderr.String())
	})

	t.Run("InputNonInteractive", func(t *testing.T) {
		t.Parallel()

		client := testCreateClient(t, lis)
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
			Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
		}
		stdout := new(bytes.Buffer)
		err := client.ExecuteProgram(
			ctx,
			cfg,
			ExecuteProgramOptions{
				InputData: []byte("test-input-non-interactive\n"),
				Stdout:    stdout,
			},
		)
		require.NoError(t, err)
		require.Equal(t, "test-input-non-interactive\n", stdout.String())
	})

	t.Run("InputInteractive", func(t *testing.T) {
		t.Parallel()

		client := testCreateClient(t, lis)
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
		require.Contains(t, "test-input-interactive\r\n", stdout.String())
	})
}

// TODO(adamb): it's copied from internal/runnerv2service.
func testStartRunnerServiceServer(t *testing.T) (*bufconn.Listener, func()) {
	t.Helper()

	logger := zaptest.NewLogger(t)
	factory := command.NewFactory(command.WithLogger(logger))

	server := grpc.NewServer()

	runnerService, err := runnerv2service.NewRunnerService(factory, logger)
	require.NoError(t, err)
	runnerv2.RegisterRunnerServiceServer(server, runnerService)

	lis := bufconn.Listen(1024 << 10)
	go server.Serve(lis)

	return lis, server.Stop
}

func testCreateClient(t *testing.T, lis *bufconn.Listener) *Client {
	client, err := New(
		"passthrough://bufconn",
		zaptest.NewLogger(t),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return client
}
