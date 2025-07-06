package company

import (
	"context"
	"go-code-runner/internal/models"
)

type Repository interface {
	Create(ctx context.Context, c *models.Company) (*models.Company, error)
	GetByEmail(ctx context.Context, email string) (*models.Company, error)
	GetByID(ctx context.Context, id int) (*models.Company, error)
	GetCompanyByAPIKey(ctx context.Context, apiKey string) (*models.Company, error)
	UpdateAPIKey(ctx context.Context, id int, apiKey string) error
	UpdateClientID(ctx context.Context, id int, clientID string) error
}
