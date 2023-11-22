package projectservice_test

import (
	"context"
	"io"
	"net"
	"path/filepath"
	"runtime"
	"testing"

	projectv1 "github.com/stateful/runme/internal/gen/proto/go/runme/project/v1"
	"github.com/stateful/runme/internal/project/projectservice"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestProjectServiceServerLoad(t *testing.T) {
	t.Parallel()

	lis, stop := testStartProjectServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateProjectServiceClient(t, lis)

	req := &projectv1.LoadRequest{
		Kind: &projectv1.LoadRequest_Directory{
			Directory: &projectv1.DirectoryProjectOptions{
				Path:               filepath.Join(testdataDir(), "git-project"),
				RespectGitignore:   true,
				IgnoreFilePatterns: []string{"ignored.md"},
				FindRepoUpward:     true,
			},
		},
	}
	loadClient, err := client.Load(context.Background(), req)
	require.NoError(t, err)

	for {
		_, err := loadClient.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}
}

func testStartProjectServiceServer(t *testing.T) (
	interface{ Dial() (net.Conn, error) },
	func(),
) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	server := grpc.NewServer()
	service := projectservice.NewProjectServiceServer(logger)
	projectv1.RegisterProjectServiceServer(server, service)

	lis := bufconn.Listen(1024 << 10)

	go server.Serve(lis)

	return lis, server.Stop
}

func testCreateProjectServiceClient(
	t *testing.T,
	lis interface{ Dial() (net.Conn, error) },
) (*grpc.ClientConn, projectv1.ProjectServiceClient) {
	conn, err := grpc.Dial(
		"",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, projectv1.NewProjectServiceClient(conn)
}

// TODO(adamb): a better approach is to store "testdata" during build time.
func testdataDir() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Join(
		filepath.Dir(b),
		"..",
		"testdata",
	)
}
