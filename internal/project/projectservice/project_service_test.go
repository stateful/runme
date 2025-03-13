package projectservice_test

import (
	"context"
	"io"
	"net"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"github.com/runmedev/runme/v3/internal/project/projectservice"
	"github.com/runmedev/runme/v3/internal/testutils"
	projectv1 "github.com/runmedev/runme/v3/pkg/api/gen/proto/go/runme/project/v1"
	"github.com/runmedev/runme/v3/pkg/project/teststub"
	projtestutils "github.com/runmedev/runme/v3/pkg/project/testutils"
)

func TestProjectServiceServer_Load(t *testing.T) {
	t.Parallel()

	lis, stop := testStartProjectServiceServer(t)
	t.Cleanup(stop)

	_, client := testutils.NewGRPCClientWithT(t, lis, projectv1.NewProjectServiceClient)

	t.Run("GitProject", func(t *testing.T) {
		t.Parallel()

		temp := t.TempDir()
		testData := teststub.Setup(t, temp)

		req := &projectv1.LoadRequest{
			Kind: &projectv1.LoadRequest_Directory{
				Directory: &projectv1.DirectoryProjectOptions{
					Path:                 testData.GitProjectPath(),
					SkipGitignore:        false,
					IgnoreFilePatterns:   projtestutils.IgnoreFilePatternsWithDefaults("ignored.md"),
					SkipRepoLookupUpward: false,
				},
			},
		}

		loadClient, err := client.Load(context.Background(), req)
		require.NoError(t, err)

		eventTypes, err := collectLoadEventTypes(loadClient)
		require.NoError(t, err)
		assert.Len(t, eventTypes, len(projtestutils.GitProjectLoadOnlyNotIgnoredFilesEvents))
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
		assert.Len(t, eventTypes, len(projtestutils.FileProjectEvents))
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
	t.Helper()

	server := grpc.NewServer()

	service := projectservice.NewProjectServiceServer(zaptest.NewLogger(t))
	projectv1.RegisterProjectServiceServer(server, service)

	lis := bufconn.Listen(1024 << 10)
	go server.Serve(lis)
	return lis, server.Stop
}
