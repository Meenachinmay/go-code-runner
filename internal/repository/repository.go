package repository

import (
	"context"
	"go-code-runner/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreateProblem(ctx context.Context, p models.Problem) (int, error)
	GetProblemByID(ctx context.Context, id int) (*models.Problem, error)
	ListProblems(ctx context.Context) ([]*models.Problem, error)

	GetTestCasesByProblemID(ctx context.Context, problemID int) ([]*models.TestCase, error)
	CreateTestCase(ctx context.Context, tc models.TestCase) (int, error) // <-- NEW
}

type repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) Repository {
	return &repository{
		db: db,
	}
}

func (r *repository) GetProblemByID(ctx context.Context, id int) (*models.Problem, error) {
	query := `
		SELECT id, title, description, difficulty, created_at, updated_at
		FROM problems
		WHERE id = $1
	`

	var problem models.Problem
	err := r.db.QueryRow(ctx, query, id).Scan(
		&problem.ID,
		&problem.Title,
		&problem.Description,
		&problem.Difficulty,
		&problem.CreatedAt,
		&problem.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &problem, nil
}

// ListProblems retrieves all problems
func (r *repository) ListProblems(ctx context.Context) ([]*models.Problem, error) {
	query := `
		SELECT id, title, description, difficulty, created_at, updated_at
		FROM problems
		ORDER BY id
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var problems []*models.Problem
	for rows.Next() {
		var problem models.Problem
		err := rows.Scan(
			&problem.ID,
			&problem.Title,
			&problem.Description,
			&problem.Difficulty,
			&problem.CreatedAt,
			&problem.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		problems = append(problems, &problem)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return problems, nil
}

func (r *repository) GetTestCasesByProblemID(ctx context.Context, problemID int) ([]*models.TestCase, error) {
	query := `
		SELECT id, problem_id, input, expected_output, is_hidden, created_at, updated_at
		FROM test_cases
		WHERE problem_id = $1
		ORDER BY id
	`

	rows, err := r.db.Query(ctx, query, problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var testCases []*models.TestCase
	for rows.Next() {
		var testCase models.TestCase
		err := rows.Scan(
			&testCase.ID,
			&testCase.ProblemID,
			&testCase.Input,
			&testCase.ExpectedOutput,
			&testCase.IsHidden,
			&testCase.CreatedAt,
			&testCase.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		testCases = append(testCases, &testCase)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return testCases, nil
}

func (r *repository) CreateTestCase(ctx context.Context, tc models.TestCase) (int, error) {
	const q = `
		INSERT INTO test_cases
		    (problem_id, input, expected_output, is_hidden, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id;
	`

	var id int
	err := r.db.QueryRow(
		ctx,
		q,
		tc.ProblemID,
		tc.Input,
		tc.ExpectedOutput,
		tc.IsHidden,
		tc.CreatedAt,
		tc.UpdatedAt,
	).Scan(&id)

	return id, err
}

func (r *repository) CreateProblem(ctx context.Context, p models.Problem) (int, error) {
	const q = `
		INSERT INTO problems
(title, description, difficulty, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;
    `
	var id int
	err := r.db.QueryRow(
		ctx,
		q,
		p.Title,
		p.Description,
		p.Difficulty,
		p.CreatedAt,
		p.UpdatedAt,
	).Scan(&id)

	return id, err
}
