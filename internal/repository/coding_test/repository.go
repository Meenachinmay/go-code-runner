package coding_test

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"go-code-runner/internal/models"
	"time"
)

type repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) CodingTestRepository {
	return &repository{db: db}
}

func (r repository) CreateTest(ctx context.Context, test *models.CodingTest) error {
	query := `
			INSERT INTO coding_tests (id, company_id, problem_id, status, expires_at, test_duration_minutes, created_at, updated_at
			) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
)`

	_, err := r.db.Exec(
		ctx,
		query,
		test.ID,
		test.CompanyID,
		test.ProblemID,
		test.Status,
		test.ExpiresAt,
		test.TestDurationMinutes,
		test.CreatedAt,
		test.UpdatedAt, )
	return err

}

func (r repository) GetTestByID(ctx context.Context, id string) (*models.CodingTest, error) {
	query := `
	SELECT id, company_id, problem_id, candidate_name, candidate_email, status, started_at, completed_at, expires_at, test_duration_minutes, 
submission_code, passed_percentage, created_at, updated_at
FROM coding_tests
WHERE id = $1`

	var test models.CodingTest
	err := r.db.QueryRow(ctx, query, id).Scan(
		&test.ID,
		&test.CompanyID,
		&test.ProblemID,
		&test.CandidateName,
		&test.CandidateEmail,
		&test.Status,
		&test.StartedAt,
		&test.CompletedAt,
		&test.ExpiresAt,
		&test.TestDurationMinutes,
		&test.SubmissionCode,
		&test.PassedPercentage,
		&test.CreatedAt,
		&test.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &test, nil
}

func (r *repository) Update(ctx context.Context, test *models.CodingTest) error {
	query := `
        UPDATE coding_tests
        SET 
            candidate_name = $2,
            candidate_email = $3,
            status = $4,
            started_at = $5,
            completed_at = $6,
            submission_code = $7,
            passed_percentage = $8,
            updated_at = $9
        WHERE id = $1`

	_, err := r.db.Exec(ctx, query,
		test.ID,
		test.CandidateName,
		test.CandidateEmail,
		test.Status,
		test.StartedAt,
		test.CompletedAt,
		test.SubmissionCode,
		test.PassedPercentage,
		time.Now(),
	)

	return err
}

func (r *repository) ExpireOldTests(ctx context.Context) error {
	query := `
        UPDATE coding_tests
        SET status = $1, updated_at = $2
        WHERE status = $3 
        AND started_at IS NOT NULL 
        AND EXTRACT(EPOCH FROM (NOW() - started_at))/60 > test_duration_minutes`

	_, err := r.db.Exec(ctx, query,
		models.TestStatusExpired,
		time.Now(),
		models.TestStatusStarted,
	)

	return err
}

func (r *repository) GetByCompanyID(ctx context.Context, companyID int) ([]*models.CodingTest, error) {
	query := `
        SELECT 
            id, company_id, problem_id, candidate_name, candidate_email,
            status, started_at, completed_at, expires_at, test_duration_minutes,
            submission_code, passed_percentage, created_at, updated_at
        FROM coding_tests
        WHERE company_id = $1
        ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tests []*models.CodingTest
	for rows.Next() {
		var test models.CodingTest
		err := rows.Scan(
			&test.ID,
			&test.CompanyID,
			&test.ProblemID,
			&test.CandidateName,
			&test.CandidateEmail,
			&test.Status,
			&test.StartedAt,
			&test.CompletedAt,
			&test.ExpiresAt,
			&test.TestDurationMinutes,
			&test.SubmissionCode,
			&test.PassedPercentage,
			&test.CreatedAt,
			&test.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		tests = append(tests, &test)
	}

	return tests, nil
}
