package problems

import (
	"context"
	"go-code-runner/internal/models"
)

// Service defines the interface for problem-related operations
type Service interface {
	// GetProblemByID retrieves a problem by its ID
	GetProblemByID(ctx context.Context, id int) (*models.Problem, error)
	
	// ListProblems retrieves all problems
	ListProblems(ctx context.Context) ([]*models.Problem, error)
	
	// GetTestCasesByProblemID retrieves test cases for a specific problem
	GetTestCasesByProblemID(ctx context.Context, problemID int) ([]*models.TestCase, error)
}