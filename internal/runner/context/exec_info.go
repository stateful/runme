package runner

import "context"

type runnerContextKey struct{}

var ExecutionInfoKey = &runnerContextKey{}

type ExecutionInfo struct {
	ExecContext string
	KnownID     string
	KnownName   string
	RunID       string
}

func ContextWithExecutionInfo(ctx context.Context, execInfo *ExecutionInfo) context.Context {
	return context.WithValue(ctx, ExecutionInfoKey, execInfo)
}
