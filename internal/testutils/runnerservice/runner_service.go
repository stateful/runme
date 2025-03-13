package runnerservice

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"github.com/runmedev/runme/v3/internal/command"
	"github.com/runmedev/runme/v3/internal/runnerv2service"
	runnerv2 "github.com/runmedev/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

func New(t *testing.T) (_ *bufconn.Listener, stop func()) {
	t.Helper()

	logger := zaptest.NewLogger(t)
	factory := command.NewFactory(command.WithLogger(logger))

	runnerService, err := runnerv2service.NewRunnerService(factory, logger)
	require.NoError(t, err)

	server := grpc.NewServer()
	runnerv2.RegisterRunnerServiceServer(server, runnerService)

	lis := bufconn.Listen(1 << 20) // 1 MB
	go server.Serve(lis)

	return lis, server.Stop
}
