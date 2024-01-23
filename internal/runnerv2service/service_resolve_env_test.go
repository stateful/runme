package runnerv2service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
)

func TestRunnerServiceResolveEnv(t *testing.T) {
	lis, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateRunnerServiceClient(t, lis)

	testCases := []struct {
		name    string
		request *runnerv2alpha1.ResolveEnvRequest
	}{
		{
			name: "WithScript",
			request: &runnerv2alpha1.ResolveEnvRequest{
				Env: []string{"TEST_RESOLVED=value"},
				Source: &runnerv2alpha1.ResolveEnvRequest_Script{
					Script: "export TEST_RESOLVED=default\nexport TEST_UNRESOLVED",
				},
			},
		},
		{
			name: "WithCommands",
			request: &runnerv2alpha1.ResolveEnvRequest{
				Env: []string{"TEST_RESOLVED=value"},
				Source: &runnerv2alpha1.ResolveEnvRequest_Commands{
					Commands: &runnerv2alpha1.ResolveEnvRequest_CommandList{
						Items: []string{"export TEST_RESOLVED=default", "export TEST_UNRESOLVED"},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.ResolveEnv(context.Background(), &runnerv2alpha1.ResolveEnvRequest{
				Env: []string{"TEST_RESOLVED=value"},
				Source: &runnerv2alpha1.ResolveEnvRequest_Script{
					Script: "export TEST_RESOLVED=default\nexport TEST_UNRESOLVED",
				},
			})
			require.NoError(t, err)
			require.Len(t, resp.Items, 2)
			require.EqualValues(
				t,
				&runnerv2alpha1.ResolveEnvResult{
					Result: &runnerv2alpha1.ResolveEnvResult_ResolvedEnv_{
						ResolvedEnv: &runnerv2alpha1.ResolveEnvResult_ResolvedEnv{
							Name:          "TEST_RESOLVED",
							OriginalValue: "default",
							ResolvedValue: "value",
						},
					},
				},
				resp.Items[0],
			)
			require.EqualValues(
				t,
				&runnerv2alpha1.ResolveEnvResult{
					Result: &runnerv2alpha1.ResolveEnvResult_UnresolvedEnv_{
						UnresolvedEnv: &runnerv2alpha1.ResolveEnvResult_UnresolvedEnv{
							Name: "TEST_UNRESOLVED",
						},
					},
				},
				resp.Items[1],
			)
		})
	}
}
