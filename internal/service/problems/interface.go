package problems

import (
	"context"
	"go-code-runner/internal/models"
)

type Service interface {
	GetProblemByID(ctx context.Context, id int) (*models.Problem, error)
	
	ListProblems(ctx context.Context) ([]*models.Problem, error)
	
	GetTestCasesByProblemID(ctx context.Context, problemID int) ([]*models.TestCase, error)
}