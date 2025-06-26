package repository

import (
	"go-code-runner/internal/repository/problems"
	"go-code-runner/internal/repository/test_cases"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository combines all repository interfaces
type Repository interface {
	problems.ProblemRepository
	test_cases.TestCaseRepository
}

// repository struct implements the Repository interface
type repository struct {
	problems.ProblemRepository
	test_cases.TestCaseRepository
}

// New creates a new repository instance
func New(db *pgxpool.Pool) Repository {
	return &repository{
		ProblemRepository:  problems.NewProblemRepository(db),
		TestCaseRepository: test_cases.NewTestCaseRepository(db),
	}
}