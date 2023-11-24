package projectservice_test

import (
	"context"
	"io"
	"net"
	"testing"

	"github.com/pkg/errors"
	projectv1 "github.com/stateful/runme/internal/gen/proto/go/runme/project/v1"
	"github.com/stateful/runme/internal/project/projectservice"
	"github.com/stateful/runme/internal/project/testdata"
	"github.com/stateful/runme/internal/project/testutils"
	"github.com/stretchr/testify/assert"
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

	t.Run("GitProject", func(t *testing.T) {
		t.Parallel()

		req := &projectv1.LoadRequest{
			Kind: &projectv1.LoadRequest_Directory{
				Directory: &projectv1.DirectoryProjectOptions{
					Path:               testdata.GitProjectPath(),
					RespectGitignore:   true,
					IgnoreFilePatterns: testutils.IgnoreFilePatternsWithDefaults("ignored.md"),
					FindRepoUpward:     true,
				},
			},
		}

		loadClient, err := client.Load(context.Background(), req)
		require.NoError(t, err)

		eventTypes, err := collectLoadEventTypes(loadClient)
		require.NoError(t, err)
		assert.Len(t, eventTypes, len(testutils.GitProjectLoadOnlyNotIgnoredFilesEvents))
	})

	t.Run("FileProject", func(t *testing.T) {
		t.Parallel()

		req := &projectv1.LoadRequest{
			Kind: &projectv1.LoadRequest_File{
				File: &projectv1.FileProjectOptions{
					Path: testdata.ProjectFilePath(),
				},
			},
		}

		loadClient, err := client.Load(context.Background(), req)
		require.NoError(t, err)

		eventTypes, err := collectLoadEventTypes(loadClient)
		require.NoError(t, err)
		assert.Len(t, eventTypes, len(testutils.FileProjectEvents))
	})
}

func collectLoadEventTypes(client projectv1.ProjectService_LoadClient) ([]projectv1.LoadEventType, error) {
	var eventTypes []projectv1.LoadEventType

	for {
		resp, err := client.Recv()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, errors.WithStack(err)
		}

		eventTypes = append(eventTypes, resp.Type)
	}

	return eventTypes, nil
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
