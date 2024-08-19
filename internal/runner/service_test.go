//go:build !windows

package runner

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"github.com/stateful/runme/v3/internal/ulid"
	runnerv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v1"
)

var (
	logger  *zap.Logger
	logFile string
)

func testCreateLogger(t *testing.T) *zap.Logger {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	t.Cleanup(func() { _ = logger.Sync() })
	return logger
}

func testStartRunnerServiceServer(t *testing.T) (
	interface{ Dial() (net.Conn, error) },
	func(),
) {
	f, err := os.CreateTemp("", "runmeServiceTestLogs")
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}
	logFile = f.Name()
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close log file: %v", err)
	}
	// N.B. We use a production config because we want to produce JSON logs so that we can
	// read them and verify required log messages are written.
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stderr", logFile}
	newLogger, err := config.Build()
	logger = newLogger

	require.NoError(t, err)
	lis := bufconn.Listen(1024 << 10)
	server := grpc.NewServer()
	runnerService, err := newRunnerService(logger)
	require.NoError(t, err)
	runnerv1.RegisterRunnerServiceServer(server, runnerService)
	go server.Serve(lis)
	return lis, server.Stop
}

func testCreateRunnerServiceClient(
	t *testing.T,
	lis interface{ Dial() (net.Conn, error) },
) (*grpc.ClientConn, runnerv1.RunnerServiceClient) {
	conn, err := grpc.Dial(
		"",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, runnerv1.NewRunnerServiceClient(conn)
}

type executeResult struct {
	Stdout   []byte
	Stderr   []byte
	MimeType string
	ExitCode int
	Err      error
}

func getExecuteResult(
	stream runnerv1.RunnerService_ExecuteClient,
	resultc chan<- executeResult,
) {
	result := executeResult{ExitCode: -1}

	for {
		r, rerr := stream.Recv()
		if rerr != nil {
			if rerr == io.EOF {
				rerr = nil
			}
			result.Err = rerr
			break
		}
		result.Stdout = append(result.Stdout, r.StdoutData...)
		result.Stderr = append(result.Stderr, r.StderrData...)
		if r.MimeType != "" {
			result.MimeType = r.MimeType
		}
		if r.ExitCode != nil {
			result.ExitCode = int(r.ExitCode.Value)
		}
	}

	resultc <- result
}

func Test_runnerService(t *testing.T) {
	lis, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateRunnerServiceClient(t, lis)

	t.Run("Sessions", func(t *testing.T) {
		t.Parallel()

		envs := []string{"TEST_OLD=value1"}
		createSessResp, err := client.CreateSession(
			context.Background(),
			&runnerv1.CreateSessionRequest{Envs: envs},
		)
		require.NoError(t, err)
		assert.NotEmpty(t, createSessResp.Session.Id)
		assert.EqualValues(t, envs, createSessResp.Session.Envs)

		getSessResp, err := client.GetSession(
			context.Background(),
			&runnerv1.GetSessionRequest{Id: createSessResp.Session.Id},
		)
		require.NoError(t, err)
		assert.True(t, proto.Equal(createSessResp.Session, getSessResp.Session))

		_, err = client.DeleteSession(
			context.Background(),
			&runnerv1.DeleteSessionRequest{Id: getSessResp.Session.Id},
		)
		assert.NoError(t, err)

		_, err = client.DeleteSession(
			context.Background(),
			&runnerv1.DeleteSessionRequest{Id: "non-existent"},
		)
		assert.Equal(t, status.Convert(err).Code(), codes.NotFound)
	})

	t.Run("ExecuteBasic", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		knownID := ulid.GenerateID()

		req := &runnerv1.ExecuteRequest{
			KnownId:     knownID,
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Commands:    []string{"echo 1", "sleep 1", "echo 2"},
		}
		err = stream.Send(req)
		assert.NoError(t, err)

		result := <-execResult

		assert.NoError(t, result.Err)
		assert.Equal(t, "1\n2\n", string(result.Stdout))
		assert.EqualValues(t, 0, result.ExitCode)
	})

	t.Run("ExecuteBasicTempFile", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_TEMP_FILE,
			Commands:    []string{"echo 1", "sleep 1", "echo 2"},
		})
		assert.NoError(t, err)

		result := <-execResult

		assert.NoError(t, result.Err)
		assert.Equal(t, "1\n2\n", string(result.Stdout))
		assert.EqualValues(t, 0, result.ExitCode)
	})

	t.Run("ExecuteBasicJavaScript", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_TEMP_FILE,
			LanguageId:  "js",
			Script:      "console.log('1'); console.log('2')",
		})
		assert.NoError(t, err)

		result := <-execResult

		assert.NoError(t, result.Err)
		assert.Equal(t, "1\n2\n", string(result.Stdout))
		assert.EqualValues(t, 0, result.ExitCode)
	})

	t.Run("ExecuteWithTTYBasic", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Tty:         true,
			Commands:    []string{"echo 1", "sleep 1", "echo 2"},
		})
		assert.NoError(t, err)

		result := <-execResult

		assert.NoError(t, result.Err)
		assert.Equal(t, "1\r\n2\r\n", string(result.Stdout))
		assert.EqualValues(t, 0, result.ExitCode)
	})

	t.Run("ExecuteBasicMimeType", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Commands:    []string{`echo '{"field1": "value", "field2": 2}'`},
		})
		assert.NoError(t, err)

		result := <-execResult

		assert.NoError(t, result.Err)
		assert.Equal(t, "{\"field1\": \"value\", \"field2\": 2}\n", string(result.Stdout))
		assert.Contains(t, result.MimeType, "application/json")
		assert.EqualValues(t, 0, result.ExitCode)
	})

	t.Run("Input", func(t *testing.T) {
		if skip, err := strconv.ParseBool(os.Getenv("SKIP_FLAKY")); err == nil && skip {
			t.Skip("skipping flaky test")
		}
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Tty:         true,
			Commands:    []string{"tr a-z x"},
		})
		require.NoError(t, err)

		errc := make(chan error)
		go func() {
			defer close(errc)
			time.Sleep(time.Second)
			err := stream.Send(&runnerv1.ExecuteRequest{
				InputData: []byte("abc\n"),
			})
			errc <- err
			time.Sleep(time.Second)
			err = stream.Send(&runnerv1.ExecuteRequest{
				InputData: []byte{4},
			})
			errc <- err
		}()
		for err := range errc {
			assert.NoError(t, err)
		}
		assert.NoError(t, stream.CloseSend())

		result := <-execResult

		assert.NoError(t, result.Err)
		assert.Regexp(t, "abc\r\nxxx\r\n", string(result.Stdout))
		assert.EqualValues(t, 0, result.ExitCode)
	})

	// The longest accepted line must not have more than 1024 bytes on macOS,
	// including the new line character at the end. Any line longer results in ^G (BELL).
	// It is possible to send more data, but it must be divided in 1024-byte chunks
	// separated by the new line character (\n).
	// More: https://man.freebsd.org/cgi/man.cgi?query=termios&sektion=4
	// On Linux, the limit is 4096 which is described on the termios man page.
	// More: https://man7.org/linux/man-pages/man3/termios.3.html
	if runtime.GOOS == "linux" {
		t.Run("LargeInput", func(t *testing.T) {
			t.Parallel()

			stream, err := client.Execute(context.Background())
			require.NoError(t, err)

			execResult := make(chan executeResult)
			go getExecuteResult(stream, execResult)

			err = stream.Send(&runnerv1.ExecuteRequest{
				ProgramName: "bash",
				CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
				Tty:         true,
				Commands:    []string{"tr a-z x"},
			})
			require.NoError(t, err)

			errc := make(chan error)
			go func() {
				defer close(errc)

				data := make([]byte, 4096)
				for i := 0; i < len(data); i++ {
					data[i] = 'a'
				}
				data[len(data)-1] = '\n'

				time.Sleep(time.Second)
				err := stream.Send(&runnerv1.ExecuteRequest{
					InputData: data,
				})
				errc <- err
				time.Sleep(time.Second)
				err = stream.Send(&runnerv1.ExecuteRequest{
					InputData: []byte{4},
				})
				errc <- err
			}()
			for err := range errc {
				assert.NoError(t, err)
			}
			assert.NoError(t, stream.CloseSend())

			result := <-execResult

			assert.NoError(t, result.Err)
			assert.Len(t, result.Stdout, 4097*2) // \n => \r\n
			assert.EqualValues(t, 0, result.ExitCode)
		})
	}

	t.Run("EnvsPersistence", func(t *testing.T) {
		t.Parallel()

		createSessResp, err := client.CreateSession(
			context.Background(),
			&runnerv1.CreateSessionRequest{
				Envs: []string{"SESSION=session1"},
			},
		)
		require.NoError(t, err)

		// First, execute using the session provided env variable SESSION.
		{
			stream, err := client.Execute(context.Background())
			require.NoError(t, err)

			execResult := make(chan executeResult)
			go getExecuteResult(stream, execResult)

			err = stream.Send(&runnerv1.ExecuteRequest{
				SessionId:   createSessResp.Session.Id,
				Envs:        []string{"EXEC_PROVIDED=execute1"},
				ProgramName: "bash",
				CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
				Commands: []string{
					"echo $SESSION $EXEC_PROVIDED",
					"export EXEC_EXPORTED=execute2",
				},
			})
			require.NoError(t, err)

			result := <-execResult

			assert.NoError(t, result.Err)
			assert.Equal(t, "session1 execute1\n", string(result.Stdout))
			assert.EqualValues(t, 0, result.ExitCode)
		}

		// Execute again using the newly exported env EXEC_EXPORTED.
		{
			stream, err := client.Execute(context.Background())
			require.NoError(t, err)

			execResult := make(chan executeResult)
			go getExecuteResult(stream, execResult)

			err = stream.Send(&runnerv1.ExecuteRequest{
				SessionId:   createSessResp.Session.Id,
				ProgramName: "bash",
				CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
				Commands: []string{
					"echo $EXEC_EXPORTED",
				},
			})
			require.NoError(t, err)

			result := <-execResult

			assert.NoError(t, result.Err)
			assert.Equal(t, "execute2\n", string(result.Stdout))
			assert.EqualValues(t, 0, result.ExitCode)
		}

		// Validate that the envs got persistent in the session.
		sessResp, err := client.GetSession(
			context.Background(),
			&runnerv1.GetSessionRequest{Id: createSessResp.Session.Id},
		)
		require.NoError(t, err)
		assert.EqualValues(
			t,
			// Session.Envs is sorted alphabetically
			[]string{"EXEC_EXPORTED=execute2", "EXEC_PROVIDED=execute1", "SESSION=session1"},
			sessResp.Session.Envs,
		)
	})

	t.Run("ExecuteWithPathEnvInSession", func(t *testing.T) {
		t.Parallel()

		// Run the first request with the default PATH.
		{
			stream, err := client.Execute(context.Background())
			require.NoError(t, err)

			result := make(chan executeResult)
			go getExecuteResult(stream, result)

			req := &runnerv1.ExecuteRequest{
				ProgramName: "echo",
				Arguments:   []string{"-n", "test"},
			}
			err = stream.Send(req)
			require.NoError(t, err)
			require.Equal(t, "test", string((<-result).Stdout))
		}

		// Provide a PATH in the session. It will be an empty dir so
		// the echo command will not be found.
		tmpDir := t.TempDir()
		sessionResp, err := client.CreateSession(
			context.Background(),
			&runnerv1.CreateSessionRequest{
				Envs: []string{"PATH=" + tmpDir},
			},
		)
		require.NoError(t, err)

		// This time the request will fail because the echo command is not found.
		{
			stream, err := client.Execute(context.Background())
			require.NoError(t, err)

			result := make(chan executeResult)
			go getExecuteResult(stream, result)

			req := &runnerv1.ExecuteRequest{
				ProgramName: "echo",
				Arguments:   []string{"-n", "test"},
				SessionId:   sessionResp.Session.Id,
			}
			err = stream.Send(req)
			require.NoError(t, err)
			require.ErrorContains(t, (<-result).Err, "unable to locate program \"echo\"")
		}
	})

	t.Run("ExecuteWithTTYCloseSendDirection", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Tty:         true,
			Commands:    []string{"sleep 1"},
		})
		assert.NoError(t, err)
		assert.NoError(t, stream.CloseSend())

		result := <-execResult

		assert.NoError(t, result.Err)
		assert.EqualValues(t, 0, result.ExitCode)
	})

	t.Run("ExecuteWithTTYSendEOT", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Tty:         true, // without TTY it won't work
			Commands:    []string{"sleep 30"},
		})
		assert.NoError(t, err)

		errc := make(chan error)
		go func() {
			defer close(errc)
			time.Sleep(time.Second)
			err := stream.Send(&runnerv1.ExecuteRequest{
				InputData: []byte{3},
			})
			errc <- err
		}()
		for err := range errc {
			assert.NoError(t, err)
		}

		result := <-execResult

		assert.NoError(t, result.Err)
		assert.EqualValues(t, 130, result.ExitCode)
	})

	// ExecuteClientCancel is similar to "ExecuteCloseSendDirection" but the client cancels
	// the connection. When running without TTY, this and sending ExecuteRequest.stop are
	// the only ways to interrupt a program.
	t.Run("ExecuteClientCancel", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stream, err := client.Execute(ctx)
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Commands:    []string{"sleep 30"},
		})
		assert.NoError(t, err)

		// Cancel instead of cleanly exiting the command on the server.
		go func() {
			time.Sleep(time.Second)
			cancel()
		}()

		result := <-execResult

		assert.Equal(t, status.Convert(result.Err).Code(), codes.Canceled)
	})

	// This test simulates a situation when a client starts a program
	// with TTY and does not know when it exists. The program should
	// return on its own after the command is done.
	t.Run("ExecuteWithTTYExitSuccess", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stream, err := client.Execute(ctx)
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Tty:         true,
			Commands:    []string{"sleep 1"},
		})
		assert.NoError(t, err)

		result := <-execResult

		assert.NoError(t, result.Err)
		assert.EqualValues(t, 0, result.ExitCode)
	})

	t.Run("ExecuteExitSuccess", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stream, err := client.Execute(ctx)
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Tty:         false,
			Commands:    []string{"sleep 1"},
		})
		assert.NoError(t, err)

		result := <-execResult

		assert.NoError(t, result.Err)
		assert.EqualValues(t, 0, result.ExitCode)
	})

	if _, err := exec.LookPath("python3"); err == nil {
		t.Run("ExecutePythonServer", func(t *testing.T) {
			t.Skip("quarantine flaky test")
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stream, err := client.Execute(ctx)
			require.NoError(t, err)

			execResult := make(chan executeResult)
			go getExecuteResult(stream, execResult)

			err = stream.Send(&runnerv1.ExecuteRequest{
				ProgramName: "bash",
				CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
				Tty:         true,
				Commands:    []string{"python3 -m http.server 0"},
			})
			assert.NoError(t, err)

			errc := make(chan error)
			go func() {
				defer close(errc)
				time.Sleep(time.Second)
				err := stream.Send(&runnerv1.ExecuteRequest{
					InputData: []byte{3},
				})
				errc <- err
			}()
			for err := range errc {
				assert.NoError(t, err)
			}

			result := <-execResult

			assert.NoError(t, result.Err)
			assert.EqualValues(t, 0, result.ExitCode, "expected exit code; stdout: %s; stderr: %s", result.Stdout, result.Stderr)
		})
	}

	t.Run("ExecuteSendRequestStop", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Tty:         false, // no TTY; only way to interrupt it is to send ExecuteRequest.stop or cancel the stream
			Commands:    []string{"sleep 30"},
		})
		assert.NoError(t, err)

		errc := make(chan error)
		go func() {
			defer close(errc)
			time.Sleep(time.Second)
			err := stream.Send(&runnerv1.ExecuteRequest{
				Stop: runnerv1.ExecuteStop_EXECUTE_STOP_INTERRUPT,
			})
			errc <- err
		}()
		for err := range errc {
			assert.NoError(t, err)
		}

		result := <-execResult

		assert.NoError(t, result.Err)
		assert.EqualValues(t, 130, result.ExitCode)
	})

	t.Run("ExecuteMultilineEnvExport", func(t *testing.T) {
		t.Parallel()

		session, err := client.CreateSession(context.Background(), &runnerv1.CreateSessionRequest{})
		require.NoError(t, err)

		sessionID := session.Session.Id

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Directory:   "../..",
			Commands: []string{
				"export LICENSE=$(cat LICENSE)",
			},
			SessionId: sessionID,
		})
		require.NoError(t, err)

		{
			execResult := make(chan executeResult)
			go getExecuteResult(stream, execResult)
			result := <-execResult
			require.EqualValues(t, 0, result.ExitCode)
		}

		_, _ = client.GetSession(context.Background(), &runnerv1.GetSessionRequest{
			Id: sessionID,
		})

		stream, err = client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Directory:   "../..",
			Commands: []string{
				"echo \"LICENSE: $LICENSE\"",
			},
			SessionId: sessionID,
		})
		assert.NoError(t, err)

		result := <-execResult

		assert.NoError(t, result.Err)
		expected, err := os.ReadFile("../../LICENSE")
		require.NoError(t, err)
		assert.Equal(t, "LICENSE: "+string(expected), string(result.Stdout))
		assert.EqualValues(t, 0, result.ExitCode)
	})

	t.Run("ExecuteWinsizeDefault", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Commands: []string{
				"tput lines -T linux",
				"tput cols -T linux",
			},
			Tty: true,
		})
		require.NoError(t, err)

		result := <-execResult
		assert.EqualValues(t, 0, result.ExitCode)
		assert.EqualValues(t, "24\r\n80\r\n", string(result.Stdout))
	})

	t.Run("ExecuteWinsizeSet", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Commands: []string{
				"tput lines -T linux",
				"tput cols -T linux",
			},
			Winsize: &runnerv1.Winsize{
				Cols: 200,
				Rows: 64,
			},
			Tty: true,
		})
		require.NoError(t, err)

		result := <-execResult
		assert.EqualValues(t, 0, result.ExitCode)
		assert.EqualValues(t, "64\r\n200\r\n", string(result.Stdout))
	})

	t.Run("ExecuteWinsizeChange", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Commands: []string{
				"read",
				"tput lines -T linux",
				"tput cols -T linux",
			},
			Tty: true,
		})
		require.NoError(t, err)

		stream.Send(&runnerv1.ExecuteRequest{
			Winsize: &runnerv1.Winsize{
				Cols: 150,
				Rows: 56,
			},
		})

		stream.Send(&runnerv1.ExecuteRequest{
			InputData: []byte("\n"),
		})

		result := <-execResult
		assert.EqualValues(t, 0, result.ExitCode)
		assert.EqualValues(t, "\r\n56\r\n150\r\n", string(result.Stdout))
	})

	t.Run("ExecuteSessionsMostRecent", func(t *testing.T) {
		ctx := context.Background()

		createSession := func(id string) string {
			resp, err := client.CreateSession(ctx, &runnerv1.CreateSessionRequest{
				Envs: []string{
					// fmt.Sprint("SESSION_NUM=%s", id),
					"SESSION_NUM=" + id,
				},
			})
			require.NoError(t, err)
			return resp.Session.Id
		}

		getSessionNum := func(sessionId string) string {
			stream, err := client.Execute(context.Background())
			require.NoError(t, err)

			execResult := make(chan executeResult)
			go getExecuteResult(stream, execResult)

			strategy := runnerv1.SessionStrategy_SESSION_STRATEGY_MOST_RECENT

			if sessionId != "" {
				strategy = runnerv1.SessionStrategy_SESSION_STRATEGY_UNSPECIFIED
			}

			err = stream.Send(&runnerv1.ExecuteRequest{
				ProgramName:     "bash",
				CommandMode:     runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
				SessionId:       sessionId,
				SessionStrategy: strategy,
				Commands: []string{
					"echo $SESSION_NUM",
				},
			})

			require.NoError(t, err)

			result := <-execResult
			return string(result.Stdout)
		}

		session1 := createSession("1")
		session2 := createSession("2")
		session3 := createSession("3")

		// create pushes priority
		assert.Equal(t, "3\n", getSessionNum(""))

		// executing pushes priority
		assert.Equal(t, getSessionNum(session2), "2\n")
		assert.Equal(t, getSessionNum(""), "2\n")

		// deleting removes from stack
		client.DeleteSession(ctx, &runnerv1.DeleteSessionRequest{Id: session2})
		assert.Equal(t, getSessionNum(""), "3\n")
		client.DeleteSession(ctx, &runnerv1.DeleteSessionRequest{Id: session3})
		assert.Equal(t, getSessionNum(""), "1\n")

		// creates new session if empty
		client.DeleteSession(ctx, &runnerv1.DeleteSessionRequest{Id: session1})
		assert.Equal(t, getSessionNum(""), "\n")
	})

	t.Run("ExecuteBackgroundPID", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())

		stream, err := client.Execute(ctx)
		require.NoError(t, err)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Commands: []string{
				"sleep 10",
			},
			Background: true,
			Tty:        true,
		})
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		msg, err := stream.Recv()
		require.NoError(t, err)

		require.NotNil(t, msg.Pid)
		cancel()

		pid := msg.Pid.Pid

		err = syscall.Kill(int(pid), syscall.SIGTERM)
		assert.NoError(t, err)
	})

	t.Run("ExecuteBackgroundStop", func(t *testing.T) {
		t.Parallel()
		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			CommandMode: runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			Commands: []string{
				"sleep 1000",
			},
			Background: true,
			Tty:        true,
		})
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		stream.Send(&runnerv1.ExecuteRequest{
			Stop: runnerv1.ExecuteStop_EXECUTE_STOP_INTERRUPT,
		})

		result := <-execResult

		require.NotEqual(
			t,
			0,
			result.ExitCode,
		)
	})

	t.Run("ExecuteStoreLastOutput", func(t *testing.T) {
		s, err := client.CreateSession(context.Background(), &runnerv1.CreateSessionRequest{})
		require.NoError(t, err)

		sessionID := s.Session.Id

		{
			stream, err := client.Execute(context.Background())
			require.NoError(t, err)

			err = stream.Send(&runnerv1.ExecuteRequest{
				ProgramName: "bash",
				Commands: []string{
					"printf 'null byte test: \\0'",
				},
				Tty:             true,
				SessionId:       sessionID,
				StoreLastOutput: true,
				CommandMode:     runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			})
			require.NoError(t, err)

			execResult := make(chan executeResult)
			go getExecuteResult(stream, execResult)
			result := <-execResult

			assert.Equal(t, "null byte test: \000", string(result.Stdout))
		}

		{
			stream, err := client.Execute(context.Background())
			require.NoError(t, err)

			err = stream.Send(&runnerv1.ExecuteRequest{
				ProgramName: "bash",
				Commands: []string{
					"echo -n \"$__\"",
				},
				Tty:             true,
				SessionId:       sessionID,
				StoreLastOutput: true,
				CommandMode:     runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL,
			})
			require.NoError(t, err)

			execResult := make(chan executeResult)
			go getExecuteResult(stream, execResult)
			result := <-execResult

			assert.Equal(t, "null byte test: ", string(result.Stdout))
		}
	})
}

func Test_readLoop(t *testing.T) {
	const dataSize = 10 * 1024 * 1024

	stdout := make([]byte, dataSize)
	stderr := make([]byte, dataSize)
	results := make(chan output)
	stdoutN, stderrN := 0, 0

	done := make(chan struct{})
	go func() {
		for data := range results {
			stdoutN += len(data.Stdout)
			stderrN += len(data.Stderr)
		}
		close(done)
	}()

	err := readLoop(bytes.NewReader(stdout), bytes.NewReader(stderr), results)
	assert.NoError(t, err)
	close(results)
	<-done
	assert.Equal(t, dataSize, stdoutN)
	assert.Equal(t, dataSize, stderrN)
}

func Test_runnerConformsOpinionatedEnvVarNaming(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		assert.True(t, runnerConformsOpinionatedEnvVarNaming("TEST"))
		assert.True(t, runnerConformsOpinionatedEnvVarNaming("ABC"))
		assert.True(t, runnerConformsOpinionatedEnvVarNaming("TEST_ABC"))
		assert.True(t, runnerConformsOpinionatedEnvVarNaming("ABC_123"))
	})

	t.Run("lowercase is invalid", func(t *testing.T) {
		assert.False(t, runnerConformsOpinionatedEnvVarNaming("test"))
		assert.False(t, runnerConformsOpinionatedEnvVarNaming("abc"))
		assert.False(t, runnerConformsOpinionatedEnvVarNaming("test_abc"))
		assert.False(t, runnerConformsOpinionatedEnvVarNaming("abc_123"))
	})

	t.Run("too short", func(t *testing.T) {
		assert.False(t, runnerConformsOpinionatedEnvVarNaming("AB"))
		assert.False(t, runnerConformsOpinionatedEnvVarNaming("T"))
	})

	t.Run("numbers only is invalid", func(t *testing.T) {
		assert.False(t, runnerConformsOpinionatedEnvVarNaming("123"))
		assert.False(t, runnerConformsOpinionatedEnvVarNaming("8761123"))
		assert.False(t, runnerConformsOpinionatedEnvVarNaming("138761123"))
	})

	t.Run("invalid characters", func(t *testing.T) {
		assert.False(t, runnerConformsOpinionatedEnvVarNaming("ABC_%^!"))
		assert.False(t, runnerConformsOpinionatedEnvVarNaming("&^%$"))
		assert.False(t, runnerConformsOpinionatedEnvVarNaming("A@#$%"))
	})
}
