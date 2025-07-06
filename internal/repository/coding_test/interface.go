package coding_test

import (
	"context"
	"go-code-runner/internal/models"
)

type CodingTestRepository interface {
	CreateTest(ctx context.Context, test *models.CodingTest) error
	GetTestByID(ctx context.Context, id string) (*models.CodingTest, error)
	Update(ctx context.Context, test *models.CodingTest) error
	ExpireOldTests(ctx context.Context) error
	GetByCompanyID(ctx context.Context, companyID int) ([]*models.CodingTest, error)
}
