//go:build !windows

package runnerv2service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

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
				&runnerv2alpha1.ResolveProgramResponse_VarsResult{
					Name:          "TEST_RESOLVED",
					OriginalValue: "default",
					ResolvedValue: "value",
					Status:        runnerv2alpha1.ResolveProgramResponse_VARS_PROMPT_RESOLVED,
				},
				resp.Vars[0],
			)
			require.EqualValues(
				t,
				&runnerv2alpha1.ResolveProgramResponse_VarsResult{
					Name:   "TEST_UNRESOLVED",
					Status: runnerv2alpha1.ResolveProgramResponse_VARS_PROMPT_MESSAGE,
				},
				resp.Vars[1],
			)
		})
	}
}
