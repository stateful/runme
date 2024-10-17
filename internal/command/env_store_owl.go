package command

import (
	"sync"

	"github.com/stateful/runme/v3/internal/owl"
	"go.uber.org/zap"
)

type envStoreOwl struct {
	logger   *zap.Logger
	owlStore *owl.Store

	mu sync.RWMutex
	// subscribers []owlEnvStorerSubscriber
}
