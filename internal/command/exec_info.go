package command

import "context"

type runnerContextKey struct{}

var ExecutionInfoKey = &runnerContextKey{}

type ExecutionInfo struct {
	RunID     string
	KnownName string
	KnownID   string
}

func ContextWithExecutionInfo(ctx context.Context, execInfo *ExecutionInfo) context.Context {
	return context.WithValue(ctx, ExecutionInfoKey, execInfo)
}
