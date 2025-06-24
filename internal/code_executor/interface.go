package code_executor

import "context"

type Service interface {
	Execute(ctx context.Context, code string, language string) (*ExecutionResult, error)
}
