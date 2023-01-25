package runner

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func Test_executeCmd(t *testing.T) {
	cmd, err := newCommand(
		&commandConfig{
			ProgramName: "bash",
			IsShell:     true,
			Commands:    []string{"echo 1", "sleep 1", "echo 2"},
		},
		testCreateLogger(t),
	)
	require.NoError(t, err)

	var results []output
	exitCode, err := executeCmd(
		context.Background(),
		cmd,
		func(data output) error {
			results = append(results, data.Clone())
			return nil
		},
		time.Millisecond*250,
	)
	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.EqualValues(
		t,
		[]output{
			{Stdout: []byte("1\n")},
			{Stdout: []byte("2\n")},
		},
		results,
	)
}

func testStartRunnerServiceServer(t *testing.T) (
	interface{ Dial() (net.Conn, error) },
	func(),
) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	lis := bufconn.Listen(2048)
	server := grpc.NewServer()
	runnerv1.RegisterRunnerServiceServer(server, newRunnerService(logger))
	go server.Serve(lis)
	return lis, server.Stop
}

func testCreateRunnerServiceClient(
	t *testing.T,
	lis interface{ Dial() (net.Conn, error) },
) (*grpc.ClientConn, runnerv1.RunnerServiceClient) {
	conn, err := grpc.Dial(
		"",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, runnerv1.NewRunnerServiceClient(conn)
}

func Test_runnerService_Execute(t *testing.T) {
	lis, stop := testStartRunnerServiceServer(t)
	defer stop()
	_, client := testCreateRunnerServiceClient(t, lis)

	stream, err := client.Execute(context.Background())
	require.NoError(t, err)

	var resps []*runnerv1.ExecuteResponse
	errC := make(chan error)

	go func() {
		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					err = nil
				}
				errC <- err
				return
			}
			resps = append(resps, resp)
		}
	}()

	err = stream.Send(&runnerv1.ExecuteRequest{
		ProgramName: "bash",
		Commands:    []string{"echo 1", "sleep 1", "echo 2"},
	})
	assert.NoError(t, err)
	assert.NoError(t, stream.CloseSend())
	assert.NoError(t, <-errC)
	assert.Len(t, resps, 3)
	assert.Equal(t, "1\n", string(resps[0].StdoutData))
	assert.Equal(t, "2\n", string(resps[1].StdoutData))
	assert.EqualValues(t, 0, resps[2].ExitCode.Value)
}
