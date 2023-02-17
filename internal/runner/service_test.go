//go:build !windows

package runner

import (
	"bytes"
	"context"
	"io"
	"net"
	"os/exec"
	"runtime"
	"testing"
	"time"

	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
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
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	lis := bufconn.Listen(2048)
	server := grpc.NewServer()
	runnerv1.RegisterRunnerServiceServer(server, newRunnerService(logger))
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
	Responses []*runnerv1.ExecuteResponse
	Err       error
}

func getExecuteResult(
	stream runnerv1.RunnerService_ExecuteClient,
	result chan<- executeResult,
) {
	var (
		resps []*runnerv1.ExecuteResponse
		err   error
	)

	for {
		r, rerr := stream.Recv()
		if rerr != nil {
			if rerr == io.EOF {
				rerr = nil
			}
			err = rerr
			break
		}
		resps = append(resps, r)
	}

	result <- executeResult{Responses: resps, Err: err}
}

func Test_runnerService(t *testing.T) {
	t.Parallel()

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

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			Commands:    []string{"echo 1", "sleep 1", "echo 2"},
		})
		assert.NoError(t, err)

		result := <-execResult

		assert.NoError(t, result.Err)
		require.Len(t, result.Responses, 3)
		assert.Equal(t, "1\n", string(result.Responses[0].StdoutData))
		assert.Equal(t, "2\n", string(result.Responses[1].StdoutData))
		assert.EqualValues(t, 0, result.Responses[2].ExitCode.Value)
	})

	t.Run("ExecuteWithTTYBasic", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			Tty:         true,
			Commands:    []string{"echo 1", "sleep 1", "echo 2"},
		})
		assert.NoError(t, err)

		result := <-execResult

		assert.NoError(t, result.Err)
		require.Len(t, result.Responses, 3)
		assert.Equal(t, "1\r\n", string(result.Responses[0].StdoutData))
		assert.Equal(t, "2\r\n", string(result.Responses[1].StdoutData))
		assert.EqualValues(t, 0, result.Responses[2].ExitCode.Value)
	})

	t.Run("Input", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
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
		require.Len(t, result.Responses, 2)
		assert.Equal(t, "xxx\r\n", string(result.Responses[0].StdoutData))
		assert.EqualValues(t, 0, result.Responses[1].ExitCode.Value)
	})

	// The longest accepted line must not have more than 1024 bytes on macOS,
	// including the new line character at the end. Any line longer results in ^G (BELL).
	// It is possible to send more data, but it must be divided in 1024-byte chunks
	// separated by the new line character (\n).
	// On Linux, the limit is 4096 which is described on the termios man page.
	// More: https://man7.org/linux/man-pages/man3/termios.3.html
	// TODO(adamb): sort out the root cause of this limitation.
	if runtime.GOOS == "linux" {
		t.Run("LargeInput", func(t *testing.T) {
			t.Parallel()

			stream, err := client.Execute(context.Background())
			require.NoError(t, err)

			execResult := make(chan executeResult)
			go getExecuteResult(stream, execResult)

			err = stream.Send(&runnerv1.ExecuteRequest{
				ProgramName: "bash",
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
			require.Len(t, result.Responses, 2)
			assert.Len(t, string(result.Responses[0].StdoutData), 4097) // \n => \r\n
			assert.EqualValues(t, 0, result.Responses[1].ExitCode.Value)
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
				Commands: []string{
					"echo $SESSION $EXEC_PROVIDED",
					"export EXEC_EXPORTED=execute2",
				},
			})
			require.NoError(t, err)

			result := <-execResult

			assert.NoError(t, result.Err)
			require.Len(t, result.Responses, 2)
			assert.Equal(t, "session1 execute1\n", string(result.Responses[0].StdoutData))
			assert.EqualValues(t, 0, result.Responses[1].ExitCode.Value)
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
				Commands: []string{
					"echo $EXEC_EXPORTED",
				},
			})
			require.NoError(t, err)

			result := <-execResult

			assert.NoError(t, result.Err)
			require.Len(t, result.Responses, 2)
			assert.Equal(t, "execute2\n", string(result.Responses[0].StdoutData))
			assert.EqualValues(t, 0, result.Responses[1].ExitCode.Value)
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

	t.Run("ExecuteWithTTYCloseSendDirection", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
			Tty:         true,
			Commands:    []string{"sleep 1"},
		})
		assert.NoError(t, err)
		assert.NoError(t, stream.CloseSend())

		result := <-execResult

		require.NoError(t, result.Err)
		require.NotEmpty(t, result.Responses)
		assert.EqualValues(t, 0, result.Responses[len(result.Responses)-1].ExitCode.Value)
	})

	t.Run("ExecuteWithTTYSendEOT", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv1.ExecuteRequest{
			ProgramName: "bash",
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

		require.NoError(t, result.Err)
		require.NotEmpty(t, result.Responses)
		assert.EqualValues(t, 130, result.Responses[len(result.Responses)-1].ExitCode.Value)
	})

	// ExecuteClientCancel is similar to "ExecuteCloseSendDirection" but the client cancels
	// the connection. When running wihout TTY, this and sending ExecuteRequest.stop are
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
			Tty:         true,
			Commands:    []string{"sleep 1"},
		})
		assert.NoError(t, err)

		result := <-execResult

		require.NoError(t, result.Err)
		require.NotEmpty(t, result.Responses)
		assert.EqualValues(t, 0, result.Responses[len(result.Responses)-1].ExitCode.Value)
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
			Tty:         false,
			Commands:    []string{"sleep 1"},
		})
		assert.NoError(t, err)

		result := <-execResult

		require.NoError(t, result.Err)
		require.NotEmpty(t, result.Responses)
		assert.EqualValues(t, 0, result.Responses[len(result.Responses)-1].ExitCode.Value)
	})

	if _, err := exec.LookPath("python3"); err == nil {
		t.Run("ExecutePythonServer", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stream, err := client.Execute(ctx)
			require.NoError(t, err)

			execResult := make(chan executeResult)
			go getExecuteResult(stream, execResult)

			err = stream.Send(&runnerv1.ExecuteRequest{
				ProgramName: "bash",
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

			require.NoError(t, result.Err)
			require.NotEmpty(t, result.Responses)
			assert.EqualValues(t, 0, result.Responses[len(result.Responses)-1].ExitCode.Value)
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

		require.NoError(t, result.Err)
		require.NotEmpty(t, result.Responses)
		assert.EqualValues(t, 130, result.Responses[len(result.Responses)-1].ExitCode.Value)
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
