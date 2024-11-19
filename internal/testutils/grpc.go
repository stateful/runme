package testutils

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewGRPCClientWithT[T any](
	t *testing.T,
	lis interface{ Dial() (net.Conn, error) },
	fn func(grpc.ClientConnInterface) T,
) (*grpc.ClientConn, T) {
	t.Helper()
	conn, client, err := newGRPCClient(lis, fn)
	require.NoError(t, err)
	return conn, client
}

func NewGRPCClient[T any](
	lis interface{ Dial() (net.Conn, error) },
	fn func(grpc.ClientConnInterface) T,
) (*grpc.ClientConn, T) {
	conn, client, err := newGRPCClient(lis, fn)
	if err != nil {
		panic(err)
	}
	return conn, client
}

func newGRPCClient[T any](
	lis interface{ Dial() (net.Conn, error) },
	fn func(grpc.ClientConnInterface) T,
) (*grpc.ClientConn, T, error) {
	conn, err := grpc.NewClient(
		"passthrough://bufconn",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		var result T
		return nil, result, err
	}
	return conn, fn(conn), nil
}
