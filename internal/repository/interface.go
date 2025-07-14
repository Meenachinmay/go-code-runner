package repository

import (
	"go-code-runner/internal/repository/coding_test"
	"go-code-runner/internal/repository/company"
	"go-code-runner/internal/repository/problems"
	"go-code-runner/internal/repository/test_cases"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	problems.ProblemRepository
	test_cases.TestCaseRepository
	company.Repository
	coding_test.CodingTestRepository
}

type repository struct {
	problems.ProblemRepository
	test_cases.TestCaseRepository
	company.Repository
	coding_test.CodingTestRepository
}

func New(db *pgxpool.Pool) Repository {
	return &repository{
		ProblemRepository:    problems.NewProblemRepository(db),
		TestCaseRepository:   test_cases.NewTestCaseRepository(db),
		Repository:           company.New(db),
		CodingTestRepository: coding_test.New(db),
	}
}
