//go:build !windows

package runnerv2service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
)

// todo(sebastian): port test cases from v1
func TestRunnerServiceResolveProgram(t *testing.T) {
	lis, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateRunnerServiceClient(t, lis)

	testCases := []struct {
		name    string
		request *runnerv2alpha1.ResolveProgramRequest
	}{
		{
			name: "WithScript",
			request: &runnerv2alpha1.ResolveProgramRequest{
				Env: []string{"TEST_RESOLVED=value"},
				Source: &runnerv2alpha1.ResolveProgramRequest_Script{
					Script: "export TEST_RESOLVED=default\nexport TEST_UNRESOLVED",
				},
			},
		},
		{
			name: "WithCommands",
			request: &runnerv2alpha1.ResolveProgramRequest{
				Env: []string{"TEST_RESOLVED=value"},
				Source: &runnerv2alpha1.ResolveProgramRequest_Commands{
					Commands: &runnerv2alpha1.ResolveProgramCommandList{
						Lines: []string{"export TEST_RESOLVED=default", "export TEST_UNRESOLVED"},
					},
				},
			},
		},
		{
			name: "WithAdditionalEnv",
			request: &runnerv2alpha1.ResolveProgramRequest{
				Env: []string{"TEST_RESOLVED=value", "TEST_EXTRA=value"},
				Source: &runnerv2alpha1.ResolveProgramRequest_Commands{
					Commands: &runnerv2alpha1.ResolveProgramCommandList{
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
			require.EqualValues(
				t,
				&runnerv2alpha1.ResolveProgramResponse_VarResult{
					Name:          "TEST_RESOLVED",
					OriginalValue: "default",
					ResolvedValue: "value",
					Status:        runnerv2alpha1.ResolveProgramResponse_STATUS_RESOLVED,
				},
				resp.Vars[0],
			)
			require.EqualValues(
				t,
				&runnerv2alpha1.ResolveProgramResponse_VarResult{
					Name:   "TEST_UNRESOLVED",
					Status: runnerv2alpha1.ResolveProgramResponse_STATUS_UNSPECIFIED,
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

	request := &runnerv2alpha1.ResolveProgramRequest{
		Env: []string{"FILE_NAME=my-file.txt"},
		Source: &runnerv2alpha1.ResolveProgramRequest_Commands{
			Commands: &runnerv2alpha1.ResolveProgramCommandList{
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
	require.True(
		t,
		proto.Equal(
			&runnerv2alpha1.ResolveProgramResponse_VarResult{
				Name:          "FILE_NAME",
				Status:        runnerv2alpha1.ResolveProgramResponse_STATUS_RESOLVED,
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
			"# FILE_NAME set in smart env store",
			"# \"export FILE_NAME=default.txt\"",
			"",
			"cat >\"$FILE_NAME\" <<EOF",
			"Some content with\nnew line",
			"EOF",
		},
		resp.Commands.Lines,
	)
}
