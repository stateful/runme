//go:build !windows

package runnerv2service

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/stateful/runme/internal/command"
	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
)

func init() {
	// Set to false to disable sending signals to process groups in tests.
	// This can be turned on if setSysProcAttrPgid() is called in Start().
	command.SignalToProcessGroup = false

	command.EnvDumpCommand = "env -0"
}

func TestRunnerServiceServerExecute(t *testing.T) {
	lis, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateRunnerServiceClient(t, lis)

	stream, err := client.Execute(context.Background())
	require.NoError(t, err)

	req := &runnerv2alpha1.ExecuteRequest{
		Config: &runnerv2alpha1.ProgramConfig{
			ProgramName: "bash",
			Source: &runnerv2alpha1.ProgramConfig_Commands{
				Commands: &runnerv2alpha1.ProgramConfig_CommandList{
					Items: []string{
						"echo test | tee >(cat >&2)",
					},
				},
			},
		},
	}

	err = stream.Send(req)
	require.NoError(t, err)

	// Assert first response.
	resp, err := stream.Recv()
	assert.NoError(t, err)
	assert.Greater(t, resp.Pid.Value, uint32(1))
	assert.Nil(t, resp.ExitCode)

	// Assert second response.
	resp, err = stream.Recv()
	assert.NoError(t, err)
	assert.Equal(t, "test\n", string(resp.StdoutData))
	assert.Nil(t, resp.ExitCode)
	assert.Nil(t, resp.Pid)

	// Assert third response.
	resp, err = stream.Recv()
	assert.NoError(t, err)
	assert.Equal(t, "test\n", string(resp.StderrData))
	assert.Nil(t, resp.ExitCode)
	assert.Nil(t, resp.Pid)

	// Assert fourth response.
	resp, err = stream.Recv()
	assert.NoError(t, err)
	assert.Equal(t, uint32(0), resp.ExitCode.Value)
	assert.Nil(t, resp.Pid)
}

func TestRunnerServiceServerExecuteConfigs(t *testing.T) {
	lis, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateRunnerServiceClient(t, lis)

	testCases := []struct {
		name           string
		programConfig  *runnerv2alpha1.ProgramConfig
		inputData      []byte
		expectedOutput string
	}{
		{
			name: "Basic",
			programConfig: &runnerv2alpha1.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2alpha1.ProgramConfig_Commands{
					Commands: &runnerv2alpha1.ProgramConfig_CommandList{
						Items: []string{
							"echo test",
						},
					},
				},
			},
			expectedOutput: "test\n",
		},
		{
			name: "BasicInteractive",
			programConfig: &runnerv2alpha1.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2alpha1.ProgramConfig_Commands{
					Commands: &runnerv2alpha1.ProgramConfig_CommandList{
						Items: []string{
							"echo test",
						},
					},
				},
				Interactive: true,
			},
			expectedOutput: "test\r\n",
		},
		{
			name: "BasicSleep",
			programConfig: &runnerv2alpha1.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2alpha1.ProgramConfig_Commands{
					Commands: &runnerv2alpha1.ProgramConfig_CommandList{
						Items: []string{
							"echo 1",
							"sleep 1",
							"echo 2",
						},
					},
				},
			},
			expectedOutput: "1\n2\n",
		},
		{
			name: "BasicInput",
			programConfig: &runnerv2alpha1.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2alpha1.ProgramConfig_Commands{
					Commands: &runnerv2alpha1.ProgramConfig_CommandList{
						Items: []string{
							"read name",
							"echo \"My name is $name\"",
						},
					},
				},
				Interactive: false,
			},
			inputData:      []byte("Frank\n"),
			expectedOutput: "My name is Frank\n",
		},
		{
			name: "BasicInputInteractive",
			programConfig: &runnerv2alpha1.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2alpha1.ProgramConfig_Commands{
					Commands: &runnerv2alpha1.ProgramConfig_CommandList{
						Items: []string{
							"read name",
							"echo \"My name is $name\"",
						},
					},
				},
				Interactive: true,
			},
			inputData:      []byte("Frank\n"),
			expectedOutput: "My name is Frank\r\n",
		},
		{
			name: "Python",
			programConfig: &runnerv2alpha1.ProgramConfig{
				ProgramName: "py",
				Source: &runnerv2alpha1.ProgramConfig_Script{
					Script: "print('test')",
				},
				Interactive: true,
			},
			expectedOutput: "test\r\n",
		},
		{
			name: "JavaScript",
			programConfig: &runnerv2alpha1.ProgramConfig{
				ProgramName: "js",
				Source: &runnerv2alpha1.ProgramConfig_Script{
					Script: "console.log('1');\nconsole.log('2');",
				},
			},
			expectedOutput: "1\n2\n",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stream, err := client.Execute(context.Background())
			require.NoError(t, err)

			execResult := make(chan executeResult)
			go getExecuteResult(stream, execResult)

			req := &runnerv2alpha1.ExecuteRequest{
				Config: tc.programConfig,
			}

			if tc.inputData != nil {
				req.InputData = tc.inputData
			}

			err = stream.Send(req)
			assert.NoError(t, err)

			result := <-execResult

			assert.NoError(t, result.Err)
			assert.Equal(t, tc.expectedOutput, string(result.Stdout))
			assert.EqualValues(t, 0, result.ExitCode)
		})
	}
}

func TestRunnerServiceServerExecute_Input(t *testing.T) {
	lis, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateRunnerServiceClient(t, lis)

	t.Run("ContinuousInput", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv2alpha1.ExecuteRequest{
			Config: &runnerv2alpha1.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2alpha1.ProgramConfig_Commands{
					Commands: &runnerv2alpha1.ProgramConfig_CommandList{
						Items: []string{
							"cat - | tr a-z A-Z",
						},
					},
				},
				Interactive: true,
			},
			InputData: []byte("a\n"),
		})
		require.NoError(t, err)

		for _, data := range [][]byte{[]byte("b\n"), []byte("c\n"), []byte("d\n"), {0x04}} {
			req := &runnerv2alpha1.ExecuteRequest{InputData: data}
			err = stream.Send(req)
			assert.NoError(t, err)
		}

		result := <-execResult

		assert.NoError(t, result.Err)
		assert.Equal(t, "A\r\nB\r\nC\r\nD\r\n", string(result.Stdout))
		assert.EqualValues(t, 0, result.ExitCode)
	})

	t.Run("SimulateCtrlC", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv2alpha1.ExecuteRequest{
			Config: &runnerv2alpha1.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2alpha1.ProgramConfig_Commands{
					Commands: &runnerv2alpha1.ProgramConfig_CommandList{
						Items: []string{
							"bash",
						},
					},
				},
				Interactive: true,
			},
			InputData: []byte("a\n"),
		})
		require.NoError(t, err)

		time.Sleep(time.Millisecond * 500)
		err = stream.Send(&runnerv2alpha1.ExecuteRequest{InputData: []byte("sleep 30")})
		assert.NoError(t, err)

		// cancel sleep
		time.Sleep(time.Millisecond * 500)
		err = stream.Send(&runnerv2alpha1.ExecuteRequest{InputData: []byte{0x03}})
		assert.NoError(t, err)

		// terminate shell
		time.Sleep(time.Millisecond * 500)
		err = stream.Send(&runnerv2alpha1.ExecuteRequest{InputData: []byte{0x04}})
		assert.NoError(t, err)

		result := <-execResult

		// TODO(adamb): This should be a specific gRPC error rather than Unknown.
		assert.Contains(t, result.Err.Error(), "exit status 130")
		assert.Equal(t, 130, result.ExitCode)
	})

	t.Run("CloseSendDirection", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv2alpha1.ExecuteRequest{
			Config: &runnerv2alpha1.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2alpha1.ProgramConfig_Commands{
					Commands: &runnerv2alpha1.ProgramConfig_CommandList{
						Items: []string{
							"sleep 30",
						},
					},
				},
			},
		})
		require.NoError(t, err)

		// Close the send direction.
		assert.NoError(t, stream.CloseSend())

		result := <-execResult
		// TODO(adamb): This should be a specific gRPC error rather than Unknown.
		assert.Contains(t, result.Err.Error(), "signal: interrupt")
		assert.Equal(t, 130, result.ExitCode)
	})
}

func TestRunnerServiceServerExecute_WithSession(t *testing.T) {
	lis, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateRunnerServiceClient(t, lis)

	t.Run("WithEnvAndMostRecentSessionStrategy", func(t *testing.T) {
		{
			stream, err := client.Execute(context.Background())
			require.NoError(t, err)

			execResult := make(chan executeResult)
			go getExecuteResult(stream, execResult)

			err = stream.Send(&runnerv2alpha1.ExecuteRequest{
				Config: &runnerv2alpha1.ProgramConfig{
					ProgramName: "bash",
					Source: &runnerv2alpha1.ProgramConfig_Commands{
						Commands: &runnerv2alpha1.ProgramConfig_CommandList{
							Items: []string{
								"echo -n \"$TEST_ENV\"",
								"export TEST_ENV=hello-2",
							},
						},
					},
					Env: []string{"TEST_ENV=hello"},
				},
			})
			require.NoError(t, err)

			result := <-execResult

			assert.NoError(t, result.Err)
			assert.Equal(t, "hello", string(result.Stdout))
		}

		{
			// Execute again with most recent session.
			stream, err := client.Execute(context.Background())
			require.NoError(t, err)

			execResult := make(chan executeResult)
			go getExecuteResult(stream, execResult)

			err = stream.Send(&runnerv2alpha1.ExecuteRequest{
				Config: &runnerv2alpha1.ProgramConfig{
					ProgramName: "bash",
					Source: &runnerv2alpha1.ProgramConfig_Commands{
						Commands: &runnerv2alpha1.ProgramConfig_CommandList{
							Items: []string{
								"echo -n \"$TEST_ENV\"",
							},
						},
					},
				},
				SessionStrategy: runnerv2alpha1.SessionStrategy_SESSION_STRATEGY_MOST_RECENT,
			})
			require.NoError(t, err)

			result := <-execResult

			assert.NoError(t, result.Err)
			assert.Equal(t, "hello-2", string(result.Stdout))
		}
	})
}

func TestRunnerServiceServerExecute_WithStop(t *testing.T) {
	lis, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateRunnerServiceClient(t, lis)

	stream, err := client.Execute(context.Background())
	require.NoError(t, err)

	execResult := make(chan executeResult)
	go getExecuteResult(stream, execResult)

	err = stream.Send(&runnerv2alpha1.ExecuteRequest{
		Config: &runnerv2alpha1.ProgramConfig{
			ProgramName: "bash",
			Source: &runnerv2alpha1.ProgramConfig_Commands{
				Commands: &runnerv2alpha1.ProgramConfig_CommandList{
					Items: []string{
						"echo 1",
						"sleep 30",
					},
				},
			},
			Interactive: true,
		},
		InputData: []byte("a\n"),
	})
	require.NoError(t, err)

	errc := make(chan error)
	go func() {
		defer close(errc)
		time.Sleep(500 * time.Millisecond)
		err := stream.Send(&runnerv2alpha1.ExecuteRequest{
			Stop: runnerv2alpha1.ExecuteStop_EXECUTE_STOP_INTERRUPT,
		})
		errc <- err
	}()
	require.NoError(t, <-errc)

	result := <-execResult

	// TODO(adamb): There should be no error.
	assert.Contains(t, result.Err.Error(), "signal: interrupt")
	assert.Equal(t, 130, result.ExitCode)
}

func TestRunnerServiceServerExecute_Winsize(t *testing.T) {
	lis, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateRunnerServiceClient(t, lis)

	t.Run("DefaultWinsize", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv2alpha1.ExecuteRequest{
			Config: &runnerv2alpha1.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2alpha1.ProgramConfig_Commands{
					Commands: &runnerv2alpha1.ProgramConfig_CommandList{
						Items: []string{
							"tput lines",
							"tput cols",
						},
					},
				},
				Env:         []string{"TERM=linux"},
				Interactive: true,
			},
		})
		require.NoError(t, err)

		result := <-execResult

		assert.NoError(t, result.Err)
		assert.Equal(t, "24\r\n80\r\n", string(result.Stdout))
		assert.EqualValues(t, 0, result.ExitCode)
	})

	t.Run("SetWinsizeInInitialRequest", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		execResult := make(chan executeResult)
		go getExecuteResult(stream, execResult)

		err = stream.Send(&runnerv2alpha1.ExecuteRequest{
			Config: &runnerv2alpha1.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2alpha1.ProgramConfig_Commands{
					Commands: &runnerv2alpha1.ProgramConfig_CommandList{
						Items: []string{
							"tput lines",
							"tput cols",
						},
					},
				},
				Interactive: true,
				Env:         []string{"TERM=linux"},
			},
			Winsize: &runnerv2alpha1.Winsize{
				Cols: 200,
				Rows: 64,
			},
		})
		require.NoError(t, err)

		result := <-execResult

		assert.NoError(t, result.Err)
		assert.Equal(t, "64\r\n200\r\n", string(result.Stdout))
		assert.EqualValues(t, 0, result.ExitCode)
	})
}

func testStartRunnerServiceServer(t *testing.T) (
	interface{ Dial() (net.Conn, error) },
	func(),
) {
	t.Helper()

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	lis := bufconn.Listen(1024 << 10)
	server := grpc.NewServer()
	runnerService, err := newRunnerService(logger)
	require.NoError(t, err)

	runnerv2alpha1.RegisterRunnerServiceServer(server, runnerService)
	go server.Serve(lis)

	return lis, server.Stop
}

func testCreateRunnerServiceClient(
	t *testing.T,
	lis interface{ Dial() (net.Conn, error) },
) (*grpc.ClientConn, runnerv2alpha1.RunnerServiceClient) {
	t.Helper()

	conn, err := grpc.Dial(
		"",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	return conn, runnerv2alpha1.NewRunnerServiceClient(conn)
}

type executeResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Err      error
}

func getExecuteResult(
	stream runnerv2alpha1.RunnerService_ExecuteClient,
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
		if r.ExitCode != nil {
			result.ExitCode = int(r.ExitCode.Value)
		}
	}

	resultc <- result
}
