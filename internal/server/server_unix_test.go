//go:build !windows

package server

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/stateful/runme/v3/internal/command"
)

func TestServerUnixSocket(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "runme.sock")
	cfg := &Config{
		Address: "unix://" + sock,
	}
	logger := zaptest.NewLogger(t)
	factory := command.NewFactory(command.WithLogger(logger))
	s, err := New(cfg, factory, logger)
	require.NoError(t, err)
	errc := make(chan error, 1)
	go func() {
		err := s.Serve()
		errc <- err
	}()

	testConnectivity(t, cfg.Address, insecure.NewCredentials())

	s.Shutdown()
	require.NoError(t, <-errc)
}
