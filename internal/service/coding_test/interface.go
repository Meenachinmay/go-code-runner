package coding_test

import (
	"context"
	"go-code-runner/internal/models"
)

type Service interface {
	GenerateTest(ctx context.Context, companyID, problemID int, expiresInHours int) (*models.CodingTest, string, error)
	VerifyTest(ctx context.Context, testID string) (*models.CodingTest, error)
	StartTest(ctx context.Context, testID, candidateName, candidateEmail string) error
	SubmitTest(ctx context.Context, testID, code string, passedPercentage int) error
	GetCompanyTests(ctx context.Context, companyID int) ([]*models.CodingTest, error)
}