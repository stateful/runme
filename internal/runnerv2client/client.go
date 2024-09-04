package runnerv2client

import (
	"github.com/pkg/errors"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
	"google.golang.org/grpc"
)

type Client struct {
	runnerv2.RunnerServiceClient
}

func New(target string, opts ...grpc.DialOption) (*Client, error) {
	client, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	serviceClient := &Client{RunnerServiceClient: runnerv2.NewRunnerServiceClient(client)}

	return serviceClient, nil
}
