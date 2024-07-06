package projectservice_test

import (
	"context"
	"io"
	"net"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/stateful/runme/v3/internal/project/projectservice"
	projectv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/project/v1"
	"github.com/stateful/runme/v3/pkg/project/teststub"
	"github.com/stateful/runme/v3/pkg/project/testutils"
)

func TestProjectServiceServer_Load(t *testing.T) {
	t.Parallel()

	lis, stop := testStartProjectServiceServer(t)
	t.Cleanup(stop)
	_, client := testCreateProjectServiceClient(t, lis)

	t.Run("GitProject", func(t *testing.T) {
		t.Parallel()

		temp := t.TempDir()
		testData := teststub.Setup(t, temp)

		req := &projectv1.LoadRequest{
			Kind: &projectv1.LoadRequest_Directory{
				Directory: &projectv1.DirectoryProjectOptions{
					Path:                 testData.GitProjectPath(),
					SkipGitignore:        false,
					IgnoreFilePatterns:   testutils.IgnoreFilePatternsWithDefaults("ignored.md"),
					SkipRepoLookupUpward: false,
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

		temp := t.TempDir()
		testData := teststub.Setup(t, temp)

		req := &projectv1.LoadRequest{
			Kind: &projectv1.LoadRequest_File{
				File: &projectv1.FileProjectOptions{
					Path: testData.ProjectFilePath(),
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

func TestProjectServiceServer_Load_ClientConnClosed(t *testing.T) {
	t.Parallel()

	temp := t.TempDir()
	testData := teststub.Setup(t, temp)

	lis, stop := testStartProjectServiceServer(t)
	t.Cleanup(stop)
	clientConn, client := testCreateProjectServiceClient(t, lis)

	req := &projectv1.LoadRequest{
		Kind: &projectv1.LoadRequest_File{
			File: &projectv1.FileProjectOptions{
				Path: testData.ProjectFilePath(),
			},
		},
	}

	loadClient, err := client.Load(context.Background(), req)
	require.NoError(t, err)

	errc := make(chan error, 1)
	go func() {
		errc <- clientConn.Close()
	}()

	for {
		_, err := loadClient.Recv()
		if err != nil {
			require.Equal(t, codes.Canceled, status.Code(err))
			break
		}
	}

	require.NoError(t, <-errc)
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
