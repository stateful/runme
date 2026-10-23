//go:build !windows

package runnerv2service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	runnerv2 "github.com/runmedev/runme/v3/api/gen/proto/go/runme/runner/v2"
	"github.com/runmedev/runme/v3/command"
	"github.com/runmedev/runme/v3/command/testdata"
	"github.com/runmedev/runme/v3/internal/testutils"
	"github.com/runmedev/runme/v3/internal/testutils/log"
	"github.com/runmedev/runme/v3/internal/ulid"
)

func Test_matchesOpinionatedEnvVarNaming(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		assert.True(t, matchesOpinionatedEnvVarNaming("TEST"))
		assert.True(t, matchesOpinionatedEnvVarNaming("ABC"))
		assert.True(t, matchesOpinionatedEnvVarNaming("TEST_ABC"))
		assert.True(t, matchesOpinionatedEnvVarNaming("ABC_123"))
	})

	t.Run("lowercase is invalid", func(t *testing.T) {
		assert.False(t, matchesOpinionatedEnvVarNaming("test"))
		assert.False(t, matchesOpinionatedEnvVarNaming("abc"))
		assert.False(t, matchesOpinionatedEnvVarNaming("test_abc"))
		assert.False(t, matchesOpinionatedEnvVarNaming("abc_123"))
	})

	t.Run("too short", func(t *testing.T) {
		assert.False(t, matchesOpinionatedEnvVarNaming("AB"))
		assert.False(t, matchesOpinionatedEnvVarNaming("T"))
	})

	t.Run("numbers only is invalid", func(t *testing.T) {
		assert.False(t, matchesOpinionatedEnvVarNaming("123"))
		assert.False(t, matchesOpinionatedEnvVarNaming("8761123"))
		assert.False(t, matchesOpinionatedEnvVarNaming("138761123"))
	})

	t.Run("invalid characters", func(t *testing.T) {
		assert.False(t, matchesOpinionatedEnvVarNaming("ABC_%^!"))
		assert.False(t, matchesOpinionatedEnvVarNaming("&^%$"))
		assert.False(t, matchesOpinionatedEnvVarNaming("A@#$%"))
	})
}

func TestRunnerServiceServerExecute_EmptyConfigErrors(t *testing.T) {
	t.Parallel()

	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	stream, err := client.Execute(context.Background())
	require.NoError(t, err)

	err = stream.Send(
		&runnerv2.ExecuteRequest{},
	)
	require.NoError(t, err)

	// Assert error response.
	resp, err := stream.Recv()
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestRunnerServiceServerExecute_Response(t *testing.T) {
	t.Parallel()

	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	stream, err := client.Execute(context.Background())
	require.NoError(t, err)

	err = stream.Send(
		&runnerv2.ExecuteRequest{
			Config: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
						Items: []string{
							"echo test | tee >(cat >&2)",
						},
					},
				},
				Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
			},
		},
	)
	require.NoError(t, err)

	// Assert first response which contains PID.
	resp, err := stream.Recv()
	assert.NoError(t, err)
	assert.Greater(t, resp.Pid.Value, uint32(1))
	assert.Nil(t, resp.ExitCode)

	// Collect second and third responses.
	var (
		out      bytes.Buffer
		mimeType string
	)

	resp, err = stream.Recv()
	assert.NoError(t, err)
	assert.Nil(t, resp.ExitCode)
	assert.Nil(t, resp.Pid)
	_, err = out.Write(resp.StdoutData)
	assert.NoError(t, err)
	_, err = out.Write(resp.StderrData)
	assert.NoError(t, err)
	if resp.MimeType != "" {
		mimeType = resp.MimeType
	}

	resp, err = stream.Recv()
	assert.NoError(t, err)
	assert.Nil(t, resp.ExitCode)
	assert.Nil(t, resp.Pid)
	_, err = out.Write(resp.StdoutData)
	assert.NoError(t, err)
	_, err = out.Write(resp.StderrData)
	assert.NoError(t, err)
	if resp.MimeType != "" {
		mimeType = resp.MimeType
	}
	// Assert the second and third responses.
	assert.Contains(t, mimeType, "text/plain")
	assert.Equal(t, "test\ntest\n", out.String())

	// Assert fourth response.
	resp, err = stream.Recv()
	assert.NoError(t, err)
	assert.Equal(t, uint32(0), resp.GetExitCode().GetValue())
	assert.Nil(t, resp.GetPid())
}

func TestRunnerServiceServerExecute_MixedOutputShortProcess(t *testing.T) {
	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	for i := range 10 {
		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		resultC := make(chan executeResult)
		go getExecuteResult(stream, resultC)

		stdout := fmt.Sprintf("stdout-%02d", i)
		stderr := fmt.Sprintf("stderr-%02d", i)
		err = stream.Send(
			&runnerv2.ExecuteRequest{
				Config: &runnerv2.ProgramConfig{
					ProgramName: "bash",
					Source: &runnerv2.ProgramConfig_Commands{
						Commands: &runnerv2.ProgramConfig_CommandList{
							Items: []string{
								fmt.Sprintf("printf %q", stdout),
								fmt.Sprintf("printf %q >&2", stderr),
							},
						},
					},
					Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
				},
			},
		)
		require.NoError(t, err)

		result := <-resultC
		require.NoError(t, result.Err)
		require.Equal(t, 0, result.ExitCode)
		require.Equal(t, stdout, string(result.Stdout))
		require.Equal(t, stderr, string(result.Stderr))
	}
}

func TestRunnerServiceServerExecute_StoreLastStdout(t *testing.T) {
	t.Parallel()

	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	sessionResp, err := client.CreateSession(context.Background(), &runnerv2.CreateSessionRequest{})
	require.NoError(t, err)
	require.NotNil(t, sessionResp.Session)

	stream1, err := client.Execute(context.Background())
	require.NoError(t, err)

	result1C := make(chan executeResult)
	go getExecuteResult(stream1, result1C)

	err = stream1.Send(
		&runnerv2.ExecuteRequest{
			Config: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
						Items: []string{
							"echo test | tee >(cat >&2)",
						},
					},
				},
			},
			SessionId:        sessionResp.GetSession().GetId(),
			StoreStdoutInEnv: true,
		},
	)
	assert.NoError(t, err)

	result1 := <-result1C
	assert.NoError(t, result1.Err)
	assert.EqualValues(t, 0, result1.ExitCode)
	assert.Equal(t, "test\n", string(result1.Stdout))
	assert.Contains(t, result1.MimeType, "text/plain")

	// subsequent req to check last stored value
	stream2, err := client.Execute(context.Background())
	require.NoError(t, err)

	result2C := make(chan executeResult)
	go getExecuteResult(stream2, result2C)

	err = stream2.Send(
		&runnerv2.ExecuteRequest{
			Config: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
						Items: []string{
							"echo $__",
						},
					},
				},
			},
			SessionId: sessionResp.GetSession().GetId(),
		},
	)
	assert.NoError(t, err)

	result2 := <-result2C
	assert.NoError(t, result2.Err)
	assert.EqualValues(t, 0, result2.ExitCode)
	assert.Equal(t, "test\n", string(result2.Stdout))
	assert.Contains(t, result2.MimeType, "text/plain")
}

func TestRunnerServiceServerExecute_LargeOutput(t *testing.T) {
	t.Parallel()

	temp := t.TempDir()
	fileName := filepath.Join(temp, "large_output.json")
	_, err := testdata.UngzipToFile(testdata.Users1MGzip, fileName)
	require.NoError(t, err)

	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	stream, err := client.Execute(context.Background())
	require.NoError(t, err)

	resultC := make(chan executeResult)
	go getExecuteResult(stream, resultC)

	err = stream.Send(
		&runnerv2.ExecuteRequest{
			Config: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
						Items: []string{
							"cat " + fileName,
						},
					},
				},
			},
		},
	)
	assert.NoError(t, err)

	result := <-resultC
	assert.NoError(t, result.Err)
	assert.EqualValues(t, 0, result.ExitCode)
	fileSize, err := os.Stat(fileName)
	assert.NoError(t, err)
	assert.EqualValues(t, fileSize.Size(), len(result.Stdout))
}

func TestRunnerServiceServerExecute_LastStdoutExceedsEnvLimit(t *testing.T) {
	t.Parallel()

	temp := t.TempDir()
	fileName := filepath.Join(temp, "large_output.json")
	_, err := testdata.UngzipToFile(testdata.Users1MGzip, fileName)
	require.NoError(t, err)

	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	sessionResp, err := client.CreateSession(context.Background(), &runnerv2.CreateSessionRequest{})
	require.NoError(t, err)
	require.NotNil(t, sessionResp.Session)

	stream1, err := client.Execute(context.Background())
	require.NoError(t, err)

	result1C := make(chan executeResult)
	go getExecuteResult(stream1, result1C)

	err = stream1.Send(
		&runnerv2.ExecuteRequest{
			Config: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
						Items: []string{
							"cat " + fileName,
						},
					},
				},
			},
			SessionId:        sessionResp.GetSession().GetId(),
			StoreStdoutInEnv: true,
		},
	)
	assert.NoError(t, err)

	result1 := <-result1C
	assert.NoError(t, result1.Err)
	assert.EqualValues(t, 0, result1.ExitCode)

	// subsequent req to check last stored value
	stream2, err := client.Execute(context.Background())
	require.NoError(t, err)

	result2C := make(chan executeResult)
	go getExecuteResult(stream2, result2C)

	err = stream2.Send(
		&runnerv2.ExecuteRequest{
			Config: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
						Items: []string{
							"echo -n $" + command.StoreStdoutEnvName,
						},
					},
				},
			},
			SessionId: sessionResp.GetSession().GetId(),
		},
	)
	assert.NoError(t, err)

	result2 := <-result2C
	assert.NoError(t, result2.Err)
	assert.EqualValues(t, 0, result2.ExitCode)
	expected, err := os.ReadFile(fileName)
	require.NoError(t, err)
	got := result2.Stdout // stdout is trimmed and should be the suffix of the complete output
	assert.Greater(t, len(got), 0)
	assert.True(t, bytes.HasSuffix(expected, got))
}

func TestRunnerServiceServerExecute_StoreKnownName(t *testing.T) {
	t.Parallel()

	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	sessionResp, err := client.CreateSession(context.Background(), &runnerv2.CreateSessionRequest{})
	require.NoError(t, err)
	require.NotNil(t, sessionResp.Session)

	stream1, err := client.Execute(context.Background())
	require.NoError(t, err)

	result1C := make(chan executeResult)
	go getExecuteResult(stream1, result1C)

	err = stream1.Send(
		&runnerv2.ExecuteRequest{
			Config: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
						Items: []string{
							"echo test | tee >(cat >&2)",
						},
					},
				},
				KnownName: "TEST_VAR",
			},
			SessionId:        sessionResp.GetSession().GetId(),
			StoreStdoutInEnv: true,
		},
	)
	assert.NoError(t, err)

	result := <-result1C
	assert.NoError(t, result.Err)
	assert.EqualValues(t, 0, result.ExitCode)
	assert.Equal(t, "test\n", string(result.Stdout))
	assert.Contains(t, result.MimeType, "text/plain")

	// subsequent req to check last stored value
	stream2, err := client.Execute(context.Background())
	require.NoError(t, err)

	result2C := make(chan executeResult)
	go getExecuteResult(stream2, result2C)

	err = stream2.Send(
		&runnerv2.ExecuteRequest{
			Config: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
						Items: []string{
							"echo $TEST_VAR",
						},
					},
				},
			},
			SessionId: sessionResp.GetSession().GetId(),
		},
	)
	assert.NoError(t, err)

	result = <-result2C
	assert.NoError(t, result.Err)
	assert.EqualValues(t, 0, result.ExitCode)
	assert.Equal(t, "test\n", string(result.Stdout))
	assert.Contains(t, result.MimeType, "text/plain")
}

func TestRunnerServiceServerExecute_KnownNameEnv(t *testing.T) {
	t.Parallel()

	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	run := func(t *testing.T, knownName string) string {
		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		resultC := make(chan executeResult)
		go getExecuteResult(stream, resultC)

		err = stream.Send(
			&runnerv2.ExecuteRequest{
				Config: &runnerv2.ProgramConfig{
					ProgramName: "bash",
					Source: &runnerv2.ProgramConfig_Commands{
						Commands: &runnerv2.ProgramConfig_CommandList{
							Items: []string{"echo \"name=[$RUNME_NAME]\""},
						},
					},
					KnownName: knownName,
				},
			},
		)
		require.NoError(t, err)

		result := <-resultC
		require.NoError(t, result.Err)
		require.EqualValues(t, 0, result.ExitCode)
		return string(result.Stdout)
	}

	assert.Equal(t, "name=[my-cell]\n", run(t, "my-cell"))
	assert.Equal(t, "name=[]\n", run(t, ""))
}

func TestRunnerServiceServerExecute_Configs(t *testing.T) {
	t.Parallel()

	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	testCases := []struct {
		name           string
		programConfig  *runnerv2.ProgramConfig
		inputData      []byte
		expectedOutput string
	}{
		{
			name: "Basic",
			programConfig: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
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
			programConfig: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
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
			programConfig: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
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
			programConfig: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
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
			programConfig: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
						Items: []string{
							"read name",
							"echo \"My name is $name\"",
						},
					},
				},
				Interactive: true,
			},
			inputData:      []byte("Frank\n"),
			expectedOutput: "Frank\r\nMy name is Frank\r\n",
		},
		{
			name: "Python",
			programConfig: &runnerv2.ProgramConfig{
				ProgramName: "",
				LanguageId:  "py",
				Source: &runnerv2.ProgramConfig_Script{
					Script: "print('test')",
				},
				Interactive: true,
			},
			expectedOutput: "test\r\n",
		},
		{
			name: "Clojure",
			programConfig: &runnerv2.ProgramConfig{
				ProgramName: "",
				LanguageId:  "clojure",
				Source: &runnerv2.ProgramConfig_Script{
					Script: "(println \"Hello, World!\")",
				},
			},
			expectedOutput: "Hello, World!\n",
		},
		{
			name: "Javascript",
			programConfig: &runnerv2.ProgramConfig{
				ProgramName: "node",
				Source: &runnerv2.ProgramConfig_Script{
					Script: "console.log('1');\nconsole.log('2');",
				},
			},
			expectedOutput: "1\n2\n",
		},
		{
			name: "Javascript_inferred",
			programConfig: &runnerv2.ProgramConfig{
				ProgramName: "",
				LanguageId:  "js",
				Source: &runnerv2.ProgramConfig_Script{
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

			resultC := make(chan executeResult)
			go getExecuteResult(stream, resultC)

			req := &runnerv2.ExecuteRequest{
				Config: tc.programConfig,
			}
			if tc.inputData != nil {
				req.InputData = tc.inputData
			}

			err = stream.Send(req)
			assert.NoError(t, err)

			result := <-resultC
			assert.NoError(t, result.Err)
			assert.Equal(t, tc.expectedOutput, string(result.Stdout))
			assert.EqualValues(t, 0, result.ExitCode)
		})
	}
}

func TestRunnerServiceServerExecute_CommandMode_Terminal(t *testing.T) {
	t.Parallel()

	const terminalReadyMarker = "__RUNME_TERMINAL_READY__"

	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	sessResp, err := client.CreateSession(context.Background(), &runnerv2.CreateSessionRequest{})
	require.NoError(t, err)

	// Step 1: execute the first command in the terminal mode with bash,
	// then write a line that exports an environment variable.
	{
		execCtx, execCancel := context.WithTimeout(context.Background(), executeResultTimeout)
		defer execCancel()

		execStream, err := client.Execute(execCtx)
		require.NoError(t, err)

		err = execStream.Send(
			&runnerv2.ExecuteRequest{
				Config: &runnerv2.ProgramConfig{
					ProgramName: "bash",
					Source: &runnerv2.ProgramConfig_Commands{
						Commands: &runnerv2.ProgramConfig_CommandList{
							Items: []string{
								"bash",
							},
						},
					},
					Mode: runnerv2.CommandMode_COMMAND_MODE_TERMINAL,
				},
				SessionId: sessResp.GetSession().GetId(),
			},
		)
		require.NoError(t, err)

		// Wait until the shell has consumed the terminal bootstrap commands.
		err = execStream.Send(
			&runnerv2.ExecuteRequest{InputData: []byte("printf '" + terminalReadyMarker + "\\n'\n")},
		)
		require.NoError(t, err)
		requireExecuteStdoutContains(t, execStream, terminalReadyMarker)

		// Export some variables so that it can be tested if they are collected.
		err = execStream.Send(
			&runnerv2.ExecuteRequest{InputData: []byte("export TEST_ENV=TEST_VALUE\nexit\n")},
		)
		require.NoError(t, err)

		resultC := make(chan executeResult, 1)
		go getExecuteResult(execStream, resultC)
		result := requireExecuteResult(t, resultC)
		require.NoError(t, result.Err)
	}

	// Step 2: execute the second command which will try to get the value of
	// the exported environment variable from the step 1.
	{
		execCtx, execCancel := context.WithTimeout(context.Background(), executeResultTimeout)
		defer execCancel()

		execStream, err := client.Execute(execCtx)
		require.NoError(t, err)

		resultC := make(chan executeResult, 1)
		go getExecuteResult(execStream, resultC)

		err = execStream.Send(
			&runnerv2.ExecuteRequest{
				Config: &runnerv2.ProgramConfig{
					ProgramName: "bash",
					Source: &runnerv2.ProgramConfig_Commands{
						Commands: &runnerv2.ProgramConfig_CommandList{
							Items: []string{
								"echo -n $TEST_ENV",
							},
						},
					},
					Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
				},
				SessionId: sessResp.GetSession().GetId(),
			},
		)
		require.NoError(t, err)

		result := requireExecuteResult(t, resultC)
		require.NoError(t, result.Err)
		require.Equal(t, "TEST_VALUE", string(result.Stdout))
	}
}

func TestRunnerServiceServerExecute_PathEnvInSession(t *testing.T) {
	t.Parallel()

	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	sessionResp, err := client.CreateSession(context.Background(), &runnerv2.CreateSessionRequest{})
	require.NoError(t, err)

	// Run the first request with the default PATH.
	{
		execStream, err := client.Execute(context.Background())
		require.NoError(t, err)

		resultC := make(chan executeResult)
		go getExecuteResult(execStream, resultC)

		err = execStream.Send(
			&runnerv2.ExecuteRequest{
				Config: &runnerv2.ProgramConfig{
					ProgramName: "echo",
					Arguments:   []string{"-n", "test"},
					Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
				},
				SessionId: sessionResp.GetSession().GetId(),
			},
		)
		require.NoError(t, err)
		require.Equal(t, "test", string((<-resultC).Stdout))
	}

	// Provide a PATH in the session. It will be an empty dir so
	// the echo command will not be found.
	{
		tmpDir := t.TempDir()
		_, err := client.UpdateSession(context.Background(), &runnerv2.UpdateSessionRequest{
			Id:  sessionResp.GetSession().GetId(),
			Env: []string{"PATH=" + tmpDir},
		})
		require.NoError(t, err)
	}

	// This time the request will fail because the echo command is not found.
	{
		execStream, err := client.Execute(context.Background())
		require.NoError(t, err)

		result := make(chan executeResult)
		go getExecuteResult(execStream, result)

		err = execStream.Send(
			&runnerv2.ExecuteRequest{
				Config: &runnerv2.ProgramConfig{
					ProgramName: "echo",
					Arguments:   []string{"-n", "test"},
				},
				SessionId: sessionResp.GetSession().GetId(),
			},
		)
		require.NoError(t, err)
		require.ErrorContains(t, (<-result).Err, "failed program lookup \"echo\"")
	}
}

func TestRunnerServiceServerExecute_WithInput(t *testing.T) {
	t.Parallel()

	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	t.Run("ContinuousInput", func(t *testing.T) {
		t.Parallel()

		execStream, err := client.Execute(context.Background())
		require.NoError(t, err)

		resultC := make(chan executeResult)
		go getExecuteResult(execStream, resultC)

		err = execStream.Send(
			&runnerv2.ExecuteRequest{
				Config: &runnerv2.ProgramConfig{
					ProgramName: "bash",
					Source: &runnerv2.ProgramConfig_Commands{
						Commands: &runnerv2.ProgramConfig_CommandList{
							Items: []string{
								"cat - | tr a-z A-Z",
							},
						},
					},
					Interactive: true,
				},
				InputData: []byte("a\n"),
			},
		)
		require.NoError(t, err)

		for _, data := range [][]byte{[]byte("b\n"), []byte("c\n"), []byte("d\n"), {0x04}} {
			req := &runnerv2.ExecuteRequest{InputData: data}
			err = execStream.Send(req)
			assert.NoError(t, err)
		}

		result := <-resultC
		assert.NoError(t, result.Err)
		assert.EqualValues(t, 0, result.ExitCode)
		// Validate the output by asserting that lowercase letters precede uppercase letters.
		for _, c := range "abcd" {
			idxLower := bytes.IndexRune(result.Stdout, c)
			idxUpper := bytes.IndexRune(result.Stdout, unicode.ToUpper(c))
			assert.Greater(t, idxLower, -1)
			assert.Greater(t, idxUpper, -1)
			assert.True(t, idxUpper > idxLower)
		}
	})

	t.Run("SimulateCtrlC", func(t *testing.T) {
		t.Parallel()

		execStream, err := client.Execute(context.Background())
		require.NoError(t, err)

		resultC := make(chan executeResult)
		go getExecuteResult(execStream, resultC)

		err = execStream.Send(
			&runnerv2.ExecuteRequest{
				Config: &runnerv2.ProgramConfig{
					ProgramName: "bash",
					Source: &runnerv2.ProgramConfig_Commands{
						Commands: &runnerv2.ProgramConfig_CommandList{
							Items: []string{
								"bash",
							},
						},
					},
					Interactive: true,
				},
			},
		)
		require.NoError(t, err)

		time.Sleep(time.Millisecond * 500)
		err = execStream.Send(&runnerv2.ExecuteRequest{InputData: []byte("sleep 30\n")})
		assert.NoError(t, err)

		// cancel sleep
		time.Sleep(time.Millisecond * 500)
		err = execStream.Send(&runnerv2.ExecuteRequest{InputData: []byte{0x03}})
		assert.NoError(t, err)

		time.Sleep(time.Millisecond * 500)
		err = execStream.Send(&runnerv2.ExecuteRequest{InputData: []byte{0x04}})
		assert.NoError(t, err)

		result := <-resultC
		// TODO(adamb): This should be a specific gRPC error rather than Unknown.
		require.Error(t, result.Err)
		assert.NotZero(t, result.ExitCode)
	})

	t.Run("CloseSendDirection", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		resultC := make(chan executeResult)
		go getExecuteResult(stream, resultC)

		err = stream.Send(&runnerv2.ExecuteRequest{
			Config: &runnerv2.ProgramConfig{
				ProgramName: "sleep",
				Arguments:   []string{"30"},
				Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
			},
		})
		require.NoError(t, err)

		// Close the send direction.
		err = stream.CloseSend()
		assert.NoError(t, err)

		result := <-resultC
		// TODO(adamb): This should be a specific gRPC error rather than Unknown.
		require.NotNil(t, result.Err)
		assert.Contains(t, result.Err.Error(), "signal: interrupt")
		assert.Equal(t, 130, result.ExitCode)
	})
}

func TestRunnerServiceServerExecute_WithSession(t *testing.T) {
	t.Parallel()

	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	t.Run("WithEnvAndMostRecentSessionStrategy", func(t *testing.T) {
		t.Parallel()

		{
			stream, err := client.Execute(context.Background())
			require.NoError(t, err)

			resultC := make(chan executeResult)
			go getExecuteResult(stream, resultC)

			err = stream.Send(
				&runnerv2.ExecuteRequest{
					Config: &runnerv2.ProgramConfig{
						ProgramName: "bash",
						Source: &runnerv2.ProgramConfig_Commands{
							Commands: &runnerv2.ProgramConfig_CommandList{
								Items: []string{
									"echo -n \"$TEST_ENV\"",
									"export TEST_ENV=hello-2",
								},
							},
						},
						Env: []string{"TEST_ENV=hello"},
					},
				},
			)
			require.NoError(t, err)

			result := <-resultC
			assert.NoError(t, result.Err)
			assert.Equal(t, "hello", string(result.Stdout))
		}

		{
			// Execute again with most recent session.
			stream, err := client.Execute(context.Background())
			require.NoError(t, err)

			resultC := make(chan executeResult)
			go getExecuteResult(stream, resultC)

			err = stream.Send(
				&runnerv2.ExecuteRequest{
					Config: &runnerv2.ProgramConfig{
						ProgramName: "bash",
						Source: &runnerv2.ProgramConfig_Commands{
							Commands: &runnerv2.ProgramConfig_CommandList{
								Items: []string{
									"echo -n \"$TEST_ENV\"",
								},
							},
						},
					},
					SessionStrategy: runnerv2.SessionStrategy_SESSION_STRATEGY_MOST_RECENT,
				},
			)
			require.NoError(t, err)

			result := <-resultC
			assert.NoError(t, result.Err)
			assert.Equal(t, "hello-2", string(result.Stdout))
		}
	})
}

func TestRunnerServiceServerExecute_WithStop(t *testing.T) {
	t.Parallel()

	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	stream, err := client.Execute(context.Background())
	require.NoError(t, err)

	resultC := make(chan executeResult)
	go getExecuteResult(stream, resultC)

	err = stream.Send(
		&runnerv2.ExecuteRequest{
			Config: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
						Items: []string{
							"echo 1",
							"sleep 30",
						},
					},
				},
				Interactive: true,
			},
		},
	)
	require.NoError(t, err)

	errC := make(chan error)
	go func() {
		defer close(errC)
		time.Sleep(time.Second)
		err := stream.Send(&runnerv2.ExecuteRequest{
			Stop: runnerv2.ExecuteStop_EXECUTE_STOP_INTERRUPT,
		})
		errC <- err
	}()
	assert.NoError(t, <-errC)

	select {
	case result := <-resultC:
		// TODO(adamb): There should be no error.
		assert.Contains(t, result.Err.Error(), "signal: interrupt")
		assert.Equal(t, 130, result.ExitCode)

		// Send one more request to make sure that the server
		// is still running after sending SIGINT.
		stream, err = client.Execute(context.Background())
		require.NoError(t, err)

		resultC := make(chan executeResult)
		go getExecuteResult(stream, resultC)

		err = stream.Send(
			&runnerv2.ExecuteRequest{
				Config: &runnerv2.ProgramConfig{
					ProgramName: "echo",
					Arguments:   []string{"-n", "1"},
					Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
				},
			},
		)
		require.NoError(t, err)

		result = <-resultC
		assert.Equal(t, "1", string(result.Stdout))
	case <-time.After(5 * time.Second):
		t.Fatal("expected the response early as the command got interrupted")
	}
}

func TestRunnerServiceServerExecute_Winsize(t *testing.T) {
	t.Parallel()

	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	t.Run("DefaultWinsize", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		resultC := make(chan executeResult)
		go getExecuteResult(stream, resultC)

		err = stream.Send(
			&runnerv2.ExecuteRequest{
				Config: &runnerv2.ProgramConfig{
					ProgramName: "bash",
					Source: &runnerv2.ProgramConfig_Commands{
						Commands: &runnerv2.ProgramConfig_CommandList{
							Items: []string{
								"tput lines",
								"tput cols",
							},
						},
					},
					Env:         []string{"TERM=linux"},
					Interactive: true,
				},
			},
		)
		require.NoError(t, err)

		result := <-resultC
		assert.NoError(t, result.Err)
		assert.Equal(t, "24\r\n80\r\n", string(result.Stdout))
		assert.EqualValues(t, 0, result.ExitCode)
	})

	t.Run("SetWinsizeInInitialRequest", func(t *testing.T) {
		t.Parallel()

		stream, err := client.Execute(context.Background())
		require.NoError(t, err)

		resultC := make(chan executeResult)
		go getExecuteResult(stream, resultC)

		err = stream.Send(
			&runnerv2.ExecuteRequest{
				Config: &runnerv2.ProgramConfig{
					ProgramName: "bash",
					Source: &runnerv2.ProgramConfig_Commands{
						Commands: &runnerv2.ProgramConfig_CommandList{
							Items: []string{
								"sleep 3", // wait for the winsize to be set
								"tput lines",
								"tput cols",
							},
						},
					},
					Interactive: true,
					Env:         []string{"TERM=linux"},
				},
				Winsize: &runnerv2.Winsize{
					Cols: 200,
					Rows: 64,
				},
			},
		)
		require.NoError(t, err)

		result := <-resultC
		assert.NoError(t, result.Err)
		assert.Equal(t, "64\r\n200\r\n", string(result.Stdout))
		assert.EqualValues(t, 0, result.ExitCode)
	})
}

func TestRunnerServiceServerExecute_ExecuteRunId(t *testing.T) {
	t.Parallel()

	logger, logFile, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	expectedRunID := ulid.GenerateID()

	stream, err := client.Execute(context.Background())
	require.NoError(t, err)

	resultC := make(chan executeResult)
	go getExecuteResult(stream, resultC)

	err = stream.Send(
		&runnerv2.ExecuteRequest{
			Config: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
						Items: []string{
							"date",
						},
					},
				},
				RunId: expectedRunID,
			},
		},
	)
	assert.NoError(t, err)

	result := <-resultC
	assert.NoError(t, result.Err)
	assert.EqualValues(t, 0, result.ExitCode)

	messages, err := log.ReadLogMessages(logger, logFile)
	require.NoError(t, err)

	for _, msg := range messages {
		if msg.Msg == "received initial request" {
			assert.Equal(t, expectedRunID, msg.RunID)
			return
		}
	}

	assert.Fail(t, "expected run ID not found in log messages")
}

func Test_VarRetentionStrategies(t *testing.T) {
	t.Parallel()
	_, _, lis, stop := startRunnerServiceServer(t)
	t.Cleanup(stop)
	_, client := testutils.NewGRPCClientWithT(t, lis, runnerv2.NewRunnerServiceClient)

	testCases := []struct {
		name           string
		retention      runnerv2.ResolveProgramRequest_Retention
		resolvedVars   int
		resolvedScript string
		expectedStdout string
	}{
		{
			name:           "Unspecified",
			retention:      runnerv2.ResolveProgramRequest_RETENTION_UNSPECIFIED,
			resolvedVars:   3,
			resolvedScript: "# Managed env store retention strategy: first\n\n#\n# VAR1 set in managed env store\n# \"export VAR1=\\\"bar\\\"\"\n\necho $VAR1 #\n# VAR2 set in managed env store\n# \"export VAR2=\\\"foo\\\"\"\n#\n# VAR2 set in managed env store\n# \"export VAR2=\\\"bar\\\"\"\n\necho $VAR2\n",
			expectedStdout: "foo\nbar\n",
		},
		{
			name:           "First",
			resolvedVars:   3,
			retention:      runnerv2.ResolveProgramRequest_RETENTION_FIRST_RUN,
			resolvedScript: "# Managed env store retention strategy: first\n\n#\n# VAR1 set in managed env store\n# \"export VAR1=\\\"bar\\\"\"\n\necho $VAR1 #\n# VAR2 set in managed env store\n# \"export VAR2=\\\"foo\\\"\"\n#\n# VAR2 set in managed env store\n# \"export VAR2=\\\"bar\\\"\"\n\necho $VAR2\n",
			expectedStdout: "foo\nbar\n",
		},
		{
			name:           "Last",
			retention:      runnerv2.ResolveProgramRequest_RETENTION_LAST_RUN,
			resolvedVars:   0,
			resolvedScript: "# Managed env store retention strategy: last\n\nexport VAR1=\"bar\"\necho $VAR1\nexport VAR2=\"foo\"\nexport VAR2=\"bar\"\necho $VAR2\n",
			expectedStdout: "bar\nbar\n",
		},
	}

	for _, tc := range testCases {
		t.Run("Retention_"+tc.name, func(t *testing.T) {
			s, err := client.CreateSession(context.Background(), &runnerv2.CreateSessionRequest{
				Env: []string{"VAR1=foo"},
			})
			require.NoError(t, err)

			sessionID := s.Session.Id

			stream, err := client.Execute(context.Background())
			defer func() { require.NoError(t, stream.CloseSend()) }()
			require.NoError(t, err)

			// skip all prompts for testing
			const mode = runnerv2.ResolveProgramRequest_MODE_SKIP_ALL
			resp, err := client.ResolveProgram(context.Background(), &runnerv2.ResolveProgramRequest{
				SessionId:  sessionID,
				Mode:       mode,
				Retention:  tc.retention,
				LanguageId: "bash",
				Source: &runnerv2.ResolveProgramRequest_Commands{
					Commands: &runnerv2.ResolveProgramCommandList{
						Lines: []string{
							`export VAR1="bar"`,
							`echo $VAR1`,
							`export VAR2="foo"`,
							`export VAR2="bar"`,
							`echo $VAR2`,
						},
					},
				},
			})
			require.NoError(t, err)

			script := resp.GetScript()
			assert.Equal(t, tc.resolvedScript, script)
			assert.Len(t, resp.Vars, tc.resolvedVars)

			// prepend the resolved vars
			var resolvedVars string
			for _, v := range resp.GetVars() {
				if v.GetStatus() != runnerv2.ResolveProgramResponse_STATUS_RESOLVED {
					continue
				}
				resolvedVars += "export " + v.GetName() + "=" + v.GetResolvedValue() + "\n"
			}
			script = resolvedVars + script

			err = stream.Send(&runnerv2.ExecuteRequest{
				Config: &runnerv2.ProgramConfig{
					ProgramName: "bash",
					Interactive: false,
					Source: &runnerv2.ProgramConfig_Script{
						Script: script,
					},
				},
				SessionId:        sessionID,
				StoreStdoutInEnv: false,
			})
			require.NoError(t, err)

			execResult := make(chan executeResult)
			go getExecuteResult(stream, execResult)
			result := <-execResult

			assert.Equal(t, tc.expectedStdout, string(result.Stdout))
		})
	}
}

// Duplicated in testutils/runnerservice/runner_service.go for other packages.
func startRunnerServiceServer(t *testing.T) (logger *zap.Logger, logFile string, _ *bufconn.Listener, stop func()) {
	t.Helper()

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
	config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	logger, err = config.Build()
	require.NoError(t, err)

	factory := command.NewFactory(command.WithLogger(logger))

	runnerService, err := NewRunnerService(factory, logger)
	require.NoError(t, err)

	server := grpc.NewServer(
		grpc.MaxRecvMsgSize(msgBufferSize*2),
		grpc.MaxSendMsgSize(msgBufferSize*2),
	)
	runnerv2.RegisterRunnerServiceServer(server, runnerService)

	lis := bufconn.Listen(1 << 20) // 1 MB
	go server.Serve(lis)

	stop = func() {
		server.Stop()
		logger.Sync()
	}

	return logger, logFile, lis, stop
}

type executeResult struct {
	Err      error
	ExitCode int
	MimeType string
	Stderr   []byte
	Stdout   []byte
}

const executeResultTimeout = 30 * time.Second

func requireExecuteResult(t *testing.T, resultC <-chan executeResult) executeResult {
	t.Helper()

	select {
	case result := <-resultC:
		return result
	case <-time.After(executeResultTimeout):
		t.Fatal("timed out waiting for execute result")
		return executeResult{}
	}
}

func requireExecuteStdoutContains(
	t *testing.T,
	stream runnerv2.RunnerService_ExecuteClient,
	expected string,
) {
	t.Helper()

	var stdout bytes.Buffer
	for {
		resp, err := stream.Recv()
		require.NoError(t, err, "timed out waiting for stdout to contain %q; stdout: %q", expected, stdout.String())

		_, _ = stdout.Write(resp.StdoutData)
		if bytes.Contains(stdout.Bytes(), []byte(expected)) {
			return
		}
	}
}

func getExecuteResult(
	stream runnerv2.RunnerService_ExecuteClient,
	resultc chan<- executeResult,
) {
	result := executeResult{
		ExitCode: -1,
	}
	bufStdout := new(bytes.Buffer)
	bufStderr := new(bytes.Buffer)

	for {
		r, rerr := stream.Recv()
		if rerr != nil {
			if rerr == io.EOF {
				rerr = nil
			}
			result.Err = rerr
			break
		}
		_, _ = bufStdout.Write(r.StdoutData)
		_, _ = bufStderr.Write(r.StderrData)
		if r.MimeType != "" {
			result.MimeType = r.MimeType
		}
		if r.ExitCode != nil {
			result.ExitCode = int(r.ExitCode.Value)
		}
	}

	result.Stdout = bufStdout.Bytes()
	result.Stderr = bufStderr.Bytes()

	resultc <- result
}
