//go:build !windows

package kernel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/bufbuild/connect-go"
	kernelv1 "github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1"
	"github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1/kernelv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func Test_kernelServiceHandler_IO(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle(
		kernelv1connect.NewKernelServiceHandler(
			NewKernelServiceHandler(zap.NewNop()),
		),
	)
	server := httptest.NewUnstartedServer(mux)
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client := kernelv1connect.NewKernelServiceClient(
		server.Client(),
		server.URL,
	)

	bashBin, prompt := testGetBash(t)
	sessionResp, err := client.PostSession(
		ctx,
		connect.NewRequest(
			&kernelv1.PostSessionRequest{
				Command: bashBin,
				Prompt:  string(prompt),
			},
		),
	)
	require.NoError(t, err)

	stream := client.IO(ctx)
	t.Cleanup(func() {
		go stream.CloseResponse()
		go stream.CloseRequest()
	})

	go func() {
		err := stream.Send(&kernelv1.IORequest{
			SessionId: sessionResp.Msg.Session.Id,
			Data:      []byte("echo 'Hello'\n"),
		})
		assert.NoError(t, err)
	}()

	re := regexp.MustCompile(`(?m:^Hello\s$)`)
	for {
		resp, err := stream.Receive()
		if err != nil {
			assert.Fail(t, "no match")
			break
		}
		if re.Match(resp.Data) {
			break
		}
	}
}
