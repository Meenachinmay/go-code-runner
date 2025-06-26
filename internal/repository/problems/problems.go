package problems

import (
	"context"
	"go-code-runner/internal/models"
)

// GetProblemByID retrieves a problem by its ID
func (r *problemRepository) GetProblemByID(ctx context.Context, id int) (*models.Problem, error) {
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
func (r *problemRepository) ListProblems(ctx context.Context) ([]*models.Problem, error) {
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

// CreateProblem creates a new problem
func (r *problemRepository) CreateProblem(ctx context.Context, p models.Problem) (int, error) {
	q := `
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
