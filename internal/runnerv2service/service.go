package runnerv2service

import (
	"go.uber.org/zap"

	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
	"github.com/stateful/runme/internal/runner"
)

type runnerService struct {
	runnerv2alpha1.UnimplementedRunnerServiceServer

	sessions *runner.SessionList

	logger *zap.Logger
}

func NewRunnerService(logger *zap.Logger) (runnerv2alpha1.RunnerServiceServer, error) {
	return newRunnerService(logger)
}

func newRunnerService(logger *zap.Logger) (*runnerService, error) {
	sessions, err := runner.NewSessionList()
	if err != nil {
		return nil, err
	}

	return &runnerService{
		sessions: sessions,
		logger:   logger,
	}, nil
}
