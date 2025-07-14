package problems

import (
	"context"
	"go-code-runner/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProblemRepository interface {
	CreateProblem(ctx context.Context, p models.Problem) (int, error)
	GetProblemByID(ctx context.Context, id int) (*models.Problem, error)
	ListProblems(ctx context.Context) ([]*models.Problem, error)
}

type problemRepository struct {
	db *pgxpool.Pool
}

func NewProblemRepository(db *pgxpool.Pool) ProblemRepository {
	return &problemRepository{
		db: db,
	}
}
