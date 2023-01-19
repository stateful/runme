//go:build !windows

package kernel

import (
	"context"
	"io"
	"net"
	"regexp"
	"testing"
	"time"

	kernelv1 "github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func testStartKernelServiceServer(t *testing.T) (
	interface{ Dial() (net.Conn, error) },
	func(),
) {
	lis := bufconn.Listen(2048)
	server := grpc.NewServer()
	kernelv1.RegisterKernelServiceServer(server, NewKernelServiceServer(zap.NewNop()))
	go server.Serve(lis)
	return lis, server.GracefulStop
}

func testCreateKernelServiceClient(
	t *testing.T,
	lis interface{ Dial() (net.Conn, error) },
) (*grpc.ClientConn, kernelv1.KernelServiceClient) {
	conn, err := grpc.Dial(
		"",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, kernelv1.NewKernelServiceClient(conn)
}

func testCreateSessionUsingKernelService(t *testing.T, client kernelv1.KernelServiceClient) (sessionID string) {
	bashBin, prompt := testGetBash(t)
	resp, err := client.PostSession(
		context.Background(),
		&kernelv1.PostSessionRequest{
			Command: bashBin,
			Prompt:  string(prompt),
		},
	)
	require.NoError(t, err)
	return resp.Session.Id
}

func Test_kernelServiceServer_PostSession(t *testing.T) {
	lis, stop := testStartKernelServiceServer(t)
	defer stop()
	_, client := testCreateKernelServiceClient(t, lis)
	testCreateSessionUsingKernelService(t, client)
}

func Test_kernelServiceServer_DeleteSession(t *testing.T) {
	lis, stop := testStartKernelServiceServer(t)
	defer stop()
	_, client := testCreateKernelServiceClient(t, lis)

	_, err := client.DeleteSession(
		context.Background(),
		&kernelv1.DeleteSessionRequest{},
	)
	require.Error(t, err)

	sessionID := testCreateSessionUsingKernelService(t, client)

	_, err = client.DeleteSession(
		context.Background(),
		&kernelv1.DeleteSessionRequest{SessionId: sessionID},
	)
	require.Error(t, err)
}

func Test_kernelServiceServer_ListSessions(t *testing.T) {
	lis, stop := testStartKernelServiceServer(t)
	defer stop()
	_, client := testCreateKernelServiceClient(t, lis)

	resp, err := client.ListSessions(
		context.Background(),
		&kernelv1.ListSessionsRequest{},
	)
	require.NoError(t, err)
	require.Len(t, resp.Sessions, 0)

	sessionID := testCreateSessionUsingKernelService(t, client)

	resp, err = client.ListSessions(
		context.Background(),
		&kernelv1.ListSessionsRequest{},
	)
	require.NoError(t, err)
	require.Len(t, resp.Sessions, 1)
	require.Equal(t, sessionID, resp.Sessions[0].Id)
}

func Test_kernelServiceServer_Execute(t *testing.T) {
	lis, stop := testStartKernelServiceServer(t)
	defer stop()
	_, client := testCreateKernelServiceClient(t, lis)

	sessionID := testCreateSessionUsingKernelService(t, client)

	resp, err := client.Execute(
		context.Background(),
		&kernelv1.ExecuteRequest{
			SessionId: sessionID,
			Command:   "echo Hello",
		},
	)
	require.NoError(t, err)
	require.Equal(t, "Hello", string(resp.Data))
	require.EqualValues(t, 0, resp.ExitCode.Value)
}

func Test_kernelServiceServer_ExecuteStream(t *testing.T) {
	lis, stop := testStartKernelServiceServer(t)
	defer stop()
	_, client := testCreateKernelServiceClient(t, lis)

	sessionID := testCreateSessionUsingKernelService(t, client)

	stream, err := client.ExecuteStream(
		context.Background(),
		&kernelv1.ExecuteRequest{
			SessionId: sessionID,
			Command:   "echo Hello",
		},
	)
	require.NoError(t, err)

	var resp []*kernelv1.ExecuteResponse
	rErr := make(chan error, 1)
	go func() {
		defer close(rErr)
		for {
			item, err := stream.Recv()
			if err != nil {
				rErr <- err
				return
			}
			resp = append(resp, item)
		}
	}()
	err = <-rErr
	assert.ErrorContains(t, err, io.EOF.Error())
	assert.NotEmpty(t, resp)
	assert.EqualValues(t, 0, resp[len(resp)-1].ExitCode.Value)
}

func Test_kernelServiceServer_InputOutput(t *testing.T) {
	lis, _ := testStartKernelServiceServer(t)
	// defer stop()
	_, client := testCreateKernelServiceClient(t, lis)

	sessionID := testCreateSessionUsingKernelService(t, client)

	ctx, cancel := context.WithCancel(context.Background())
	stream, err := client.Output(
		ctx,
		&kernelv1.OutputRequest{
			SessionId: sessionID,
		},
	)
	require.NoError(t, err)

	var resp []*kernelv1.OutputResponse
	rErr := make(chan error, 1)
	go func() {
		defer close(rErr)
		for {
			item, err := stream.Recv()
			if err != nil {
				t.Logf("received item with error %s", err)
				rErr <- err
				return
			}
			t.Logf("received item %s", item.Data)
			resp = append(resp, item)
		}
	}()

	time.Sleep(time.Second)
	_, err = client.Input(
		context.Background(),
		&kernelv1.InputRequest{SessionId: sessionID, Data: []byte("echo Hello\n")},
	)
	require.NoError(t, err)
	time.Sleep(time.Second)
	_, err = client.Input(
		context.Background(),
		&kernelv1.InputRequest{SessionId: sessionID, Data: []byte("echo World\n")},
	)
	require.NoError(t, err)
	time.Sleep(time.Second)

	cancel()
	err = <-rErr
	require.ErrorIs(t, err, status.FromContextError(context.Canceled).Err())
}

func Test_kernelServiceServer_IO(t *testing.T) {
	lis, stop := testStartKernelServiceServer(t)
	defer stop()
	conn, client := testCreateKernelServiceClient(t, lis)

	sessionID := testCreateSessionUsingKernelService(t, client)

	ctx, cancel := context.WithCancel(context.Background())
	stream, err := client.IO(ctx)
	require.NoError(t, err)

	err = stream.Send(&kernelv1.IORequest{
		SessionId: sessionID,
		Data:      []byte("echo 'Hello'\n"),
	})
	require.NoError(t, err)

	re := regexp.MustCompile(`(?m:^Hello\s$)`)
	for {
		resp, err := stream.Recv()
		if err != nil {
			require.Fail(t, "no match")
		}
		if re.Match(resp.Data) {
			break
		}
	}

	cancel()
	err = stream.CloseSend()
	require.NoError(t, err)
	err = conn.Close()
	require.NoError(t, err)
}
