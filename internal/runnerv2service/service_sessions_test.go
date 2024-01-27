//go:build !windows

package runnerv2service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
	"github.com/stateful/runme/internal/project/testdata"
)

// TODO(adamb): add a test case with project.
func TestRunnerServiceSessions(t *testing.T) {
	lis, stop := testStartRunnerServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateRunnerServiceClient(t, lis)

	t.Run("WithEnv", func(t *testing.T) {
		createResp, err := client.CreateSession(context.Background(), &runnerv2alpha1.CreateSessionRequest{})
		require.NoError(t, err)
		require.NotNil(t, createResp.Session)

		createResp, err = client.CreateSession(context.Background(), &runnerv2alpha1.CreateSessionRequest{Env: []string{"TEST1=value1"}})
		require.NoError(t, err)
		require.EqualValues(t, []string{"TEST1=value1"}, createResp.Session.Env)

		getResp, err := client.GetSession(context.Background(), &runnerv2alpha1.GetSessionRequest{Id: createResp.Session.Id})
		require.NoError(t, err)
		require.EqualValues(t, []string{"TEST1=value1"}, getResp.Session.Env)

		updateResp, err := client.UpdateSession(
			context.Background(),
			&runnerv2alpha1.UpdateSessionRequest{Id: createResp.Session.Id, Env: []string{"TEST2=value2"}},
		)
		require.NoError(t, err)
		require.Equal(t, []string{"TEST1=value1", "TEST2=value2"}, updateResp.Session.Env)

		deleteResp, err := client.DeleteSession(context.Background(), &runnerv2alpha1.DeleteSessionRequest{Id: updateResp.Session.Id})
		require.NoError(t, err)
		require.NotNil(t, deleteResp)

		getResp, err = client.GetSession(context.Background(), &runnerv2alpha1.GetSessionRequest{Id: createResp.Session.Id})
		require.Error(t, err)
		require.Nil(t, getResp)
	})

	t.Run("WithProject", func(t *testing.T) {
		projectPath := testdata.GitProjectPath()
		createResp, err := client.CreateSession(
			context.Background(),
			&runnerv2alpha1.CreateSessionRequest{Project: &runnerv2alpha1.Project{Root: projectPath, EnvLoadOrder: []string{".env"}}},
		)
		require.NoError(t, err)
		require.NotNil(t, createResp.Session)
		require.EqualValues(t, []string{"PROJECT_ENV_FROM_DOTFILE=1"}, createResp.Session.Env)
	})

	t.Run("WithProjectInvalid", func(t *testing.T) {
		_, err := client.CreateSession(
			context.Background(),
			&runnerv2alpha1.CreateSessionRequest{Project: &runnerv2alpha1.Project{Root: "/non/existing/path"}},
		)
		require.Error(t, err)
	})
}
