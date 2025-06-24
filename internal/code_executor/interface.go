package code_executor

import (
	"context"
	"go-code-runner/internal/models"
)

type Service interface {
	Execute(ctx context.Context, code string, language string) (*ExecutionResult, error)
	ExecuteWithTestCases(ctx context.Context, code string, language string, testCases []*models.TestCase) (*models.ExecutionResults, error)
	ExecuteForProblem(ctx context.Context, code string, language string, problemID int) (*models.ExecutionResults, error)
}
