//go:build !windows

package runner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/pkg/project"
	"github.com/stateful/runme/v3/pkg/project/teststub"
)

func Test_EnvDirEnv(t *testing.T) {
	temp := t.TempDir()
	testData := teststub.Setup(t, temp)

	proj, err := project.NewDirProject(testData.DirEnvProjectPath(), project.WithEnvDirEnv(true))
	require.NoError(t, err)

	sess, err := NewSession([]string{}, proj, zap.NewNop())
	require.NoError(t, err)

	msg, err := sess.LoadDirEnv(context.Background(), proj, proj.Root())
	require.NoError(t, err)
	require.Contains(t, msg, "direnv: export +PGDATABASE +PGHOST +PGOPTIONS +PGPASSWORD +PGPORT +PGUSER")

	actualEnvs, err := sess.Envs()
	require.NoError(t, err)

	expectedEnvs := []string{
		"PGDATABASE=platform",
		"PGHOST=127.0.0.1",
		"PGOPTIONS=--search_path=account,public",
		"PGPASSWORD=postgres",
		"PGPORT=15430",
		"PGUSER=postgres",
	}
	for _, env := range expectedEnvs {
		require.Contains(t, actualEnvs, env)
	}
}
