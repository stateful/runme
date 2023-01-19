//go:build !windows

package kernel

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/bufbuild/connect-go"
	kernelv1 "github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1"
	"github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1/kernelv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func Test_kernelServiceHandler_IO(t *testing.T) {
	// TODO: revise it later. It fails to receive
	// messages sent before closing the send side.
	t.SkipNow()

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

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := stream.Send(&kernelv1.IORequest{
			SessionId: sessionResp.Msg.Session.Id,
			Data:      []byte("echo 'Hello'\n"),
		})
		assert.NoError(t, err)
		assert.NoError(t, stream.CloseRequest())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		re := regexp.MustCompile(`(?m:^Hello\s$)`)
		matched := false
		for {
			resp, err := stream.Receive()
			if err != nil && errors.Unwrap(err).Error() == "EOF" {
				break
			}
			require.NoError(t, err)
			if re.Match(resp.Data) {
				matched = true
			}
		}
		assert.True(t, matched)
		assert.NoError(t, stream.CloseResponse())
	}()

	wg.Wait()
}
