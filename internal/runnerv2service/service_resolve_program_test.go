//go:build !windows

package runnerv2service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

// todo(sebastian): port test cases from v1
func TestRunnerServiceResolveProgram(t *testing.T) {
	lis, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateRunnerServiceClient(t, lis)

	testCases := []struct {
		name    string
		request *runnerv2.ResolveProgramRequest
	}{
		{
			name: "WithScript",
			request: &runnerv2.ResolveProgramRequest{
				Env:        []string{"TEST_RESOLVED=value"},
				LanguageId: "bash",
				Source: &runnerv2.ResolveProgramRequest_Script{
					Script: "export TEST_RESOLVED=default\nexport TEST_UNRESOLVED",
				},
			},
		},
		{
			name: "WithCommands",
			request: &runnerv2.ResolveProgramRequest{
				Env:        []string{"TEST_RESOLVED=value"},
				LanguageId: "bash",
				Source: &runnerv2.ResolveProgramRequest_Commands{
					Commands: &runnerv2.ResolveProgramCommandList{
						Lines: []string{"export TEST_RESOLVED=default", "export TEST_UNRESOLVED"},
					},
				},
			},
		},
		{
			name: "WithAdditionalEnv",
			request: &runnerv2.ResolveProgramRequest{
				Env:        []string{"TEST_RESOLVED=value", "TEST_EXTRA=value"},
				LanguageId: "bash",
				Source: &runnerv2.ResolveProgramRequest_Commands{
					Commands: &runnerv2.ResolveProgramCommandList{
						Lines: []string{"export TEST_RESOLVED=default", "export TEST_UNRESOLVED"},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.ResolveProgram(context.Background(), tc.request)
			require.NoError(t, err)
			require.Len(t, resp.Vars, 2)
			require.Equal(t, resp.Commands, (*runnerv2.ResolveProgramCommandList)(nil))
			require.Greater(t, len(resp.Script), 0)
			require.EqualValues(
				t,
				&runnerv2.ResolveProgramResponse_VarResult{
					Name:          "TEST_RESOLVED",
					OriginalValue: "default",
					ResolvedValue: "value",
					Status:        runnerv2.ResolveProgramResponse_STATUS_RESOLVED,
				},
				resp.Vars[0],
			)
			require.EqualValues(
				t,
				&runnerv2.ResolveProgramResponse_VarResult{
					Name:   "TEST_UNRESOLVED",
					Status: runnerv2.ResolveProgramResponse_STATUS_UNSPECIFIED,
				},
				resp.Vars[1],
			)
		})
	}
}

func TestRunnerResolveProgram_CommandsWithNewLines(t *testing.T) {
	// TODO(adamb): enable it when we find a solution for merging commands and splitting them back.
	t.Skip("the problem is unknown and needs to be fixed")

	lis, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateRunnerServiceClient(t, lis)

	request := &runnerv2.ResolveProgramRequest{
		Env:        []string{"FILE_NAME=my-file.txt"},
		LanguageId: "bash",
		Source: &runnerv2.ResolveProgramRequest_Commands{
			Commands: &runnerv2.ResolveProgramCommandList{
				Lines: []string{
					"export FILE_NAME=default.txt",
					"cat >\"$FILE_NAME\" <<EOF",
					"Some content with\nnew line",
					"EOF",
				},
			},
		},
	}

	resp, err := client.ResolveProgram(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, resp.Vars, 1)
	require.Equal(t, resp.Commands, (*runnerv2.ResolveProgramCommandList)(nil))
	require.Greater(t, len(resp.Script), 0)
	require.True(
		t,
		proto.Equal(
			&runnerv2.ResolveProgramResponse_VarResult{
				Name:          "FILE_NAME",
				Status:        runnerv2.ResolveProgramResponse_STATUS_RESOLVED,
				OriginalValue: "default.txt",
				ResolvedValue: "my-file.txt",
			},
			resp.Vars[0],
		),
	)
	require.EqualValues(
		t,
		[]string{
			"#",
			"# FILE_NAME set in managed env store",
			"# \"export FILE_NAME=default.txt\"",
			"",
			"cat >\"$FILE_NAME\" <<EOF",
			"Some content with\nnew line",
			"EOF",
		},
		resp.Commands.Lines,
	)
}

func TestRunnerResolveProgram_OnlyShellLanguages(t *testing.T) {
	lis, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateRunnerServiceClient(t, lis)

	t.Run("Javascript passed as script", func(t *testing.T) {
		script := "console.log('test');"
		request := &runnerv2.ResolveProgramRequest{
			Env:        []string{"TEST_RESOLVED=value"},
			LanguageId: "javascript",
			Source: &runnerv2.ResolveProgramRequest_Script{
				Script: script,
			},
		}

		resp, err := client.ResolveProgram(context.Background(), request)
		require.NoError(t, err)
		require.Len(t, resp.Vars, 0)
		require.Equal(t, script, resp.Script)
	})

	t.Run("Python passed as commands", func(t *testing.T) {
		script := "print('test')"
		request := &runnerv2.ResolveProgramRequest{
			LanguageId: "py",
			Source: &runnerv2.ResolveProgramRequest_Commands{
				Commands: &runnerv2.ResolveProgramCommandList{Lines: []string{script}},
			},
		}

		resp, err := client.ResolveProgram(context.Background(), request)
		require.NoError(t, err)
		require.Len(t, resp.Vars, 0)
		require.Equal(t, script, resp.Script)
	})
}
