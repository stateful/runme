package testutils

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewTestGRPCClientConn(
	t *testing.T,
	lis interface{ Dial() (net.Conn, error) },
) *grpc.ClientConn {
	if t != nil {
		t.Helper()
	}
	conn, err := grpc.NewClient(
		"passthrough://bufconn",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn
}

func NewTestGRPCClient[T any](
	t *testing.T,
	lis interface{ Dial() (net.Conn, error) },
	fn func(grpc.ClientConnInterface) T,
) (*grpc.ClientConn, T) {
	if t != nil {
		t.Helper()
	}
	conn := NewTestGRPCClientConn(t, lis)
	return conn, fn(conn)
}
