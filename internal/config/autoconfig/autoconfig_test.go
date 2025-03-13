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
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/runmedev/runme/v3/internal/config"
	"github.com/runmedev/runme/v3/internal/runnerv2client"
	"github.com/runmedev/runme/v3/internal/server"
)

func TestInvokeForCommand_Config(t *testing.T) {
	builder := NewBuilder()
	configRootFS := fstest.MapFS{
		"runme.yaml": {
			// It's ok that README.md does not exist as it's not used in this test.
			Data: []byte(fmt.Sprintf("version: v1alpha1\nproject:\n  filename: %s\n", "README.md")),
		},
	}
	err := builder.Decorate(
		func() (*config.Loader, error) {
			return config.NewLoader([]string{"runme.yaml"}, configRootFS), nil
		},
	)
	require.NoError(t, err)
	err = builder.Invoke(func(*config.Config) error { return nil })
	require.NoError(t, err)
}

func TestInvokeForCommand_ServerClient(t *testing.T) {
	t.Run("NoServerInConfig", func(t *testing.T) {
		builder := NewBuilder()
		temp := t.TempDir()

		err := os.WriteFile(filepath.Join(temp, "README.md"), []byte("Hello, World!"), 0o644)
		require.NoError(t, err)

		configRootFS := fstest.MapFS{
			"runme.yaml": {
				Data: []byte(`version: v1alpha1
project:
  filename: ` + filepath.Join(temp, "README.md") + `
server: null
`),
			},
		}
		err = builder.Decorate(
			func() (*config.Loader, error) {
				return config.NewLoader([]string{"runme.yaml"}, configRootFS), nil
			},
		)
		require.NoError(t, err)

		err = builder.Invoke(func(
			server *server.Server,
			client *runnerv2client.Client,
		) error {
			require.Nil(t, server)
			require.Nil(t, client)
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("ServerInConfigWithoutTLS", func(t *testing.T) {
		builder := NewBuilder()
		temp := t.TempDir()

		err := os.WriteFile(filepath.Join(temp, "README.md"), []byte("Hello, World!"), 0o644)
		require.NoError(t, err)

		configRootFS := fstest.MapFS{
			"runme.yaml": {
				Data: []byte(`version: v1alpha1
project:
  filename: ` + filepath.Join(temp, "README.md") + `
`),
			},
		}
		err = builder.Decorate(
			func() (*config.Loader, error) {
				return config.NewLoader([]string{"runme.yaml"}, configRootFS), nil
			},
		)
		require.NoError(t, err)

		err = builder.Invoke(func(
			server *server.Server,
			client *runnerv2client.Client,
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
		builder := NewBuilder()
		temp := t.TempDir()

		err := os.WriteFile(filepath.Join(temp, "README.md"), []byte("Hello, World!"), 0o644)
		require.NoError(t, err)

		configRootFS := fstest.MapFS{
			"runme.yaml": {
				Data: []byte(`version: v1alpha1
project:
  filename: ` + filepath.Join(temp, "README.md") + `
`),
			},
		}
		err = builder.Decorate(
			func() (*config.Loader, error) {
				return config.NewLoader([]string{"runme.yaml"}, configRootFS), nil
			},
		)
		require.NoError(t, err)

		err = builder.Invoke(func(
			server *server.Server,
			client *runnerv2client.Client,
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

func checkHealth(client healthv1.HealthClient) error {
	var (
		resp *healthv1.HealthCheckResponse
		err  error
	)

	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		resp, err = client.Check(ctx, &healthv1.HealthCheckRequest{})
		if err != nil || resp.Status != healthv1.HealthCheckResponse_SERVING {
			cancel()
			time.Sleep(time.Millisecond * 100)
			continue
		}
		cancel()
		break
	}

	return err
}
