package test_cases

import (
	"context"
	"go-code-runner/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestCaseRepository defines the interface for test case-related database operations
type TestCaseRepository interface {
	GetTestCasesByProblemID(ctx context.Context, problemID int) ([]*models.TestCase, error)
	CreateTestCase(ctx context.Context, tc models.TestCase) (int, error)
}

// testCaseRepository implements the TestCaseRepository interface
type testCaseRepository struct {
	db *pgxpool.Pool
}

// NewTestCaseRepository creates a new test case repository
func NewTestCaseRepository(db *pgxpool.Pool) TestCaseRepository {
	return &testCaseRepository{
		db: db,
	}
}