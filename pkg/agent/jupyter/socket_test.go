package jupyter

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGoZeroMQFactoryValidatesSocketConfiguration(t *testing.T) {
	factory := NewGoZeroMQFactory(Limits{})
	ctx := context.Background()

	if _, err := factory.NewSocket(ctx, SocketTypeDealer, nil); err == nil {
		t.Fatal("NewSocket() accepted an empty DEALER identity")
	}
	if _, err := factory.NewSocket(ctx, SocketType("push"), []byte("client")); err == nil {
		t.Fatal("NewSocket() accepted an unsupported socket type")
	}

	dealer, err := factory.NewSocket(ctx, SocketTypeDealer, []byte("client"))
	if err != nil {
		t.Fatalf("NewSocket() error = %v", err)
	}
	defer dealer.Close()
	if err := dealer.Dial("tcp://192.0.2.10:1234"); err == nil {
		t.Fatal("Dial() accepted a non-loopback endpoint")
	}
	if err := dealer.Dial("ipc:///tmp/kernel.sock"); err == nil {
		t.Fatal("Dial() accepted a non-TCP endpoint")
	}
}

func TestGoZeroMQSocketEnforcesMultipartLimits(t *testing.T) {
	factory := NewGoZeroMQFactory(Limits{
		MaxFrameCount:   2,
		MaxFrameBytes:   4,
		MaxMessageBytes: 8,
	})
	socket, err := factory.NewSocket(context.Background(), SocketTypeReq, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()

	if err := socket.SendMultipart([][]byte{[]byte("12345")}); err == nil {
		t.Fatal("SendMultipart() accepted an oversized frame")
	}
	if err := socket.SendMultipart([][]byte{[]byte("1"), []byte("2"), []byte("3")}); err == nil {
		t.Fatal("SendMultipart() accepted too many frames")
	}
}

func TestGoZeroMQSocketReceiveHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	factory := NewGoZeroMQFactory(Limits{})
	socket, err := factory.NewSocket(ctx, SocketTypeSub, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	cancel()

	done := make(chan error, 1)
	go func() {
		_, err := socket.ReceiveMultipart()
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("ReceiveMultipart() error = %v, want context cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReceiveMultipart() did not unblock after context cancellation")
	}
}
