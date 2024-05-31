package server

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/stateful/runme/v3/internal/command"
	runmetls "github.com/stateful/runme/v3/internal/tls"
)

func TestServer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	factory := command.NewFactory(command.WithLogger(logger))

	t.Run("tcp", func(t *testing.T) {
		cfg := &Config{
			Address: "localhost:0",
		}
		s, err := New(cfg, factory, logger)
		require.NoError(t, err)
		errc := make(chan error, 1)
		go func() {
			errc <- s.Serve()
		}()

		testConnectivity(t, s.Addr(), insecure.NewCredentials())

		s.Shutdown()
		require.NoError(t, <-errc)
	})

	t.Run("tcp with tls", func(t *testing.T) {
		dir := t.TempDir()
		cfg := &Config{
			Address:    "localhost:0",
			CertFile:   filepath.Join(dir, "cert.pem"),
			KeyFile:    filepath.Join(dir, "key.pem"),
			TLSEnabled: true,
		}
		s, err := New(cfg, factory, logger)
		require.NoError(t, err)
		errc := make(chan error, 1)
		go func() {
			errc <- s.Serve()
		}()

		tlsConfig, err := runmetls.LoadClientConfig(cfg.CertFile, cfg.KeyFile)
		require.NoError(t, err)

		addr := s.Addr()
		if runtime.GOOS == "windows" {
			addr = strings.TrimPrefix(addr, "unix://")
		}
		testConnectivity(t, addr, credentials.NewTLS(tlsConfig))

		s.Shutdown()
		require.NoError(t, <-errc)
	})
}

func testConnectivity(t *testing.T, addr string, creds credentials.TransportCredentials) {
	t.Helper()

	var err error

	for i := 0; i < 5; i++ {
		var (
			conn *grpc.ClientConn
			resp *healthv1.HealthCheckResponse
		)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		conn, err = grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(creds))
		if err != nil {
			goto wait
		}

		resp, err = healthv1.NewHealthClient(conn).Check(ctx, &healthv1.HealthCheckRequest{})
		if err != nil || resp.Status != healthv1.HealthCheckResponse_SERVING {
			goto wait
		}

		cancel()
		break

	wait:
		cancel()
		<-time.After(time.Millisecond * 100)
	}

	require.NoError(t, err)
}
