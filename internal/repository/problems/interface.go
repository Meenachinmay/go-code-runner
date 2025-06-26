package problems

import (
	"context"
	"go-code-runner/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ProblemRepository defines the interface for problem-related database operations
type ProblemRepository interface {
	CreateProblem(ctx context.Context, p models.Problem) (int, error)
	GetProblemByID(ctx context.Context, id int) (*models.Problem, error)
	ListProblems(ctx context.Context) ([]*models.Problem, error)
}

// problemRepository implements the ProblemRepository interface
type problemRepository struct {
	db *pgxpool.Pool
}

// NewProblemRepository creates a new problem repository
func NewProblemRepository(db *pgxpool.Pool) ProblemRepository {
	return &problemRepository{
		db: db,
	}
}