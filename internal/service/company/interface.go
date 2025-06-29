package company

import (
	"context"
	"go-code-runner/internal/models"
)

type Service interface {
	Register(ctx context.Context, name, email, password string) (*models.Company, error)
	Login(ctx context.Context, email, password string) (*models.Company, string, error)
	GenerateAPIKey(ctx context.Context, companyID int) (string, error)
	GenerateClientID(ctx context.Context, companyID int) (string, error)
}
