package runner

import "context"

type contextKey struct{ string }

var executionInfoKey = &contextKey{"ExecutionInfo"}

type ExecutionInfo struct {
	ExecContext string
	KnownID     string
	KnownName   string
	RunID       string
}

func WithExecutionInfo(ctx context.Context, execInfo *ExecutionInfo) context.Context {
	return context.WithValue(ctx, executionInfoKey, execInfo)
}

func ExecutionInfoFromContext(ctx context.Context) (*ExecutionInfo, bool) {
	execInfo, ok := ctx.Value(executionInfoKey).(*ExecutionInfo)
	return execInfo, ok
}
