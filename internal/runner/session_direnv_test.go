//go:build !windows

package runner

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/stateful/runme/v3/pkg/project"
	"github.com/stateful/runme/v3/pkg/project/teststub"
)

func Test_EnvDirEnv(t *testing.T) {
	temp := t.TempDir()
	testData := teststub.Setup(t, temp)

	proj, err := project.NewDirProject(testData.DirEnvProjectPath(), project.WithEnvDirEnv(true))
	require.NoError(t, err)

	logBuf := &bytes.Buffer{}
	writer := zapcore.AddSync(logBuf)
	encCfg := zap.NewDevelopmentEncoderConfig()
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encCfg), writer, zap.DebugLevel)
	logger := zap.New(core)

	sess, err := NewSession([]string{}, proj, logger)
	require.NoError(t, err)

	actualEnvs, err := sess.Envs()
	require.NoError(t, err)

	require.NoError(t, err)
	require.Contains(t, logBuf.String(), "direnv: export +PGDATABASE +PGHOST +PGOPTIONS +PGPASSWORD +PGPORT +PGUSER")

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
