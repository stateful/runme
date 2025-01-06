package autoconfig

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/server"
)

func TestInvokeForCommand_Config(t *testing.T) {
	// Create a fake filesystem and set it in [config.Loader].
	err := InvokeForCommand(func(loader *config.Loader) error {
		fsys := fstest.MapFS{
			"runme.yaml": {
				Data: []byte(fmt.Sprintf("version: v1alpha1\nproject:\n  filename: %s\n", "README.md")),
			},
		}
		loader.SetConfigRootPath(fsys)
		return nil
	})
	require.NoError(t, err)

	err = InvokeForCommand(func(
		*config.Config,
	) error {
		return nil
	})
	require.NoError(t, err)
}

func TestInvokeForCommand_ServerClient(t *testing.T) {
	tmp := t.TempDir()
	readme := filepath.Join(tmp, "README.md")
	err := os.WriteFile(readme, []byte("Hello, World!"), 0644)
	require.NoError(t, err)

	t.Run("NoServerInConfig", func(t *testing.T) {
		err := InvokeForCommand(func(loader *config.Loader) error {
			fsys := fstest.MapFS{
				"runme.yaml": {
					Data: []byte(fmt.Sprintf("version: v1alpha1\nproject:\n  filename: %s\n", readme)),
				},
			}
			loader.SetConfigRootPath(fsys)
			return nil
		})
		require.NoError(t, err)

		err = InvokeForCommand(func(
			server *server.Server,
			client *grpc.ClientConn,
		) error {
			require.Nil(t, server)
			require.Nil(t, client)
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("ServerInConfigWithoutTLS", func(t *testing.T) {
		err := InvokeForCommand(func(loader *config.Loader) error {
			fsys := fstest.MapFS{
				"runme.yaml": {
					Data: []byte(`version: v1alpha1
project:
  filename: ` + readme + `
server:
  address: localhost:0
  tls:
    enabled: false
`),
				},
			}
			loader.SetConfigRootPath(fsys)
			return nil
		})
		require.NoError(t, err)

		err = InvokeForCommand(func(
			server *server.Server,
			client *grpc.ClientConn,
		) error {
			require.NotNil(t, server)
			require.NotNil(t, client)

			var g errgroup.Group

			g.Go(func() error {
				return server.Serve()
			})

			g.Go(func() error {
				defer server.Shutdown()
				return checkHealth(client)
			})

			return g.Wait()
		})
		require.NoError(t, err)
	})

	t.Run("ServerInConfigWithTLS", func(t *testing.T) {
		// Use a temp dir to store the TLS files.
		err = DecorateRoot(func() (UserConfigDir, error) {
			return UserConfigDir(tmp), nil
		})
		require.NoError(t, err)

		err := InvokeForCommand(func(loader *config.Loader) error {
			fsys := fstest.MapFS{
				"runme.yaml": {
					Data: []byte(`version: v1alpha1
project:
  filename: ` + readme + `
server:
  address: 127.0.0.1:0
  tls:
    enabled: true
`),
				},
			}
			loader.SetConfigRootPath(fsys)
			return nil
		})
		require.NoError(t, err)

		err = InvokeForCommand(func(
			server *server.Server,
			client *grpc.ClientConn,
		) error {
			require.NotNil(t, server)
			require.NotNil(t, client)

			var g errgroup.Group

			g.Go(func() error {
				return server.Serve()
			})

			g.Go(func() error {
				defer server.Shutdown()
				return errors.WithMessage(checkHealth(client), "failed to check health")
			})

			return g.Wait()
		})
		require.NoError(t, err)
	})
}

func checkHealth(conn *grpc.ClientConn) error {
	client := healthv1.NewHealthClient(conn)

	var (
		resp *healthv1.HealthCheckResponse
		err  error
	)

	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		resp, err = client.Check(ctx, &healthv1.HealthCheckRequest{})
		if err != nil || resp.Status != healthv1.HealthCheckResponse_SERVING {
			cancel()
			time.Sleep(time.Second)
			continue
		}
		cancel()
		break
	}

	return err
}
