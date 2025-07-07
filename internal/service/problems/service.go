package problems

import (
	"context"
	"go-code-runner/internal/models"
	"go-code-runner/internal/repository"
)

type service struct {
	repo repository.Repository
}

func New(repo repository.Repository) Service {
	return &service{
		repo: repo,
	}
}

func (s *service) GetProblemByID(ctx context.Context, id int) (*models.Problem, error) {
	return s.repo.GetProblemByID(ctx, id)
}

func (s *service) ListProblems(ctx context.Context) ([]*models.Problem, error) {
	return s.repo.ListProblems(ctx)
}

func (s *service) GetTestCasesByProblemID(ctx context.Context, problemID int) ([]*models.TestCase, error) {
	return s.repo.GetTestCasesByProblemID(ctx, problemID)
}