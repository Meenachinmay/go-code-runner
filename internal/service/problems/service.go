package problems

import (
	"context"
	"go-code-runner/internal/models"
	"go-code-runner/internal/repository"
)

// service implements the Service interface
type service struct {
	repo repository.Repository
}

// New creates a new problem service
func New(repo repository.Repository) Service {
	return &service{
		repo: repo,
	}
}

// GetProblemByID retrieves a problem by its ID
func (s *service) GetProblemByID(ctx context.Context, id int) (*models.Problem, error) {
	return s.repo.GetProblemByID(ctx, id)
}

// ListProblems retrieves all problems
func (s *service) ListProblems(ctx context.Context) ([]*models.Problem, error) {
	return s.repo.ListProblems(ctx)
}

// GetTestCasesByProblemID retrieves test cases for a specific problem
func (s *service) GetTestCasesByProblemID(ctx context.Context, problemID int) ([]*models.TestCase, error) {
	return s.repo.GetTestCasesByProblemID(ctx, problemID)
}