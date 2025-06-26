package test_cases

import (
	"context"
	"go-code-runner/internal/models"
)

// GetTestCasesByProblemID retrieves all test cases for a specific problem
func (r *testCaseRepository) GetTestCasesByProblemID(ctx context.Context, problemID int) ([]*models.TestCase, error) {
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

// CreateTestCase creates a new test case
func (r *testCaseRepository) CreateTestCase(ctx context.Context, tc models.TestCase) (int, error) {
	q := `
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