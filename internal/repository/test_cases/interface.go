package test_cases

import (
	"context"
	"go-code-runner/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TestCaseRepository interface {
	GetTestCasesByProblemID(ctx context.Context, problemID int) ([]*models.TestCase, error)
	CreateTestCase(ctx context.Context, tc models.TestCase) (int, error)
}

type testCaseRepository struct {
	db *pgxpool.Pool
}

func NewTestCaseRepository(db *pgxpool.Pool) TestCaseRepository {
	return &testCaseRepository{
		db: db,
	}
}
