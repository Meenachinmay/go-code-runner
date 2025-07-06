package company

import (
	"context"
	"errors"
	"go-code-runner/internal/models"
	svc "go-code-runner/internal/service/company"
	"testing"
	"time"
)

// mockRepository is a mock implementation of the company.Repository interface
type mockRepository struct {
	companies map[int]*models.Company
	emails    map[string]int // email -> company ID mapping
	nextID    int
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		companies: make(map[int]*models.Company),
		emails:    make(map[string]int),
		nextID:    1,
	}
}

func (m *mockRepository) Create(ctx context.Context, c *models.Company) (*models.Company, error) {
	// Check if email already exists
	if _, exists := m.emails[c.Email]; exists {
		return nil, errors.New("email already exists")
	}

	// Set ID and timestamps
	c.ID = m.nextID
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	m.nextID++

	// Store the company
	m.companies[c.ID] = c
	m.emails[c.Email] = c.ID

	return c, nil
}

func (m *mockRepository) GetByEmail(ctx context.Context, email string) (*models.Company, error) {
	id, exists := m.emails[email]
	if !exists {
		return nil, errors.New("company not found")
	}
	return m.companies[id], nil
}

func (m *mockRepository) GetByID(ctx context.Context, id int) (*models.Company, error) {
	c, exists := m.companies[id]
	if !exists {
		return nil, errors.New("company not found")
	}
	return c, nil
}

func (m *mockRepository) UpdateAPIKey(ctx context.Context, id int, apiKey string) error {
	c, exists := m.companies[id]
	if !exists {
		return errors.New("company not found")
	}
	c.APIKey = &apiKey
	c.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepository) UpdateClientID(ctx context.Context, id int, clientID string) error {
	c, exists := m.companies[id]
	if !exists {
		return errors.New("company not found")
	}
	c.ClientID = &clientID
	c.UpdatedAt = time.Now()
	return nil
}

func (m *mockRepository) GetCompanyByAPIKey(ctx context.Context, apiKey string) (*models.Company, error) {
	// Iterate through companies to find one with matching API key
	for _, company := range m.companies {
		if company.APIKey != nil && *company.APIKey == apiKey {
			return company, nil
		}
	}
	return nil, errors.New("company not found")
}

func TestRegister(t *testing.T) {
	repo := newMockRepository()
	service := svc.New(repo)

	t.Run("SuccessfulRegistration", func(t *testing.T) {
		name := "Test Company"
		email := "test@example.com"
		password := "password123"

		company, err := service.Register(context.Background(), name, email, password)
		if err != nil {
			t.Fatalf("failed to register company: %v", err)
		}

		if company.ID <= 0 {
			t.Errorf("expected positive ID, got %d", company.ID)
		}
		if company.Name != name {
			t.Errorf("expected name %q, got %q", name, company.Name)
		}
		if company.Email != email {
			t.Errorf("expected email %q, got %q", email, company.Email)
		}
		if company.PasswordHash == password {
			t.Error("password was not hashed")
		}
	})

	t.Run("DuplicateEmail", func(t *testing.T) {
		name := "Another Company"
		email := "test@example.com" // Same email as previous test
		password := "password456"

		_, err := service.Register(context.Background(), name, email, password)
		if err == nil {
			t.Error("expected error when registering with duplicate email, got nil")
		}
	})
}

func TestLogin(t *testing.T) {
	repo := newMockRepository()
	service := svc.New(repo)

	// Register a company for login tests
	name := "Login Test Company"
	email := "login@example.com"
	password := "loginpassword"

	_, err := service.Register(context.Background(), name, email, password)
	if err != nil {
		t.Fatalf("failed to register company for login test: %v", err)
	}

	t.Run("SuccessfulLogin", func(t *testing.T) {
		company, token, err := service.Login(context.Background(), email, password)
		if err != nil {
			t.Fatalf("failed to login: %v", err)
		}

		if company == nil {
			t.Fatal("expected company to be returned, got nil")
		}
		if company.Email != email {
			t.Errorf("expected email %q, got %q", email, company.Email)
		}
		if token == "" {
			t.Error("expected token to be returned, got empty string")
		}
	})

	t.Run("InvalidEmail", func(t *testing.T) {
		_, _, err := service.Login(context.Background(), "nonexistent@example.com", password)
		if err == nil {
			t.Error("expected error when logging in with invalid email, got nil")
		}
	})

	t.Run("InvalidPassword", func(t *testing.T) {
		_, _, err := service.Login(context.Background(), email, "wrongpassword")
		if err == nil {
			t.Error("expected error when logging in with invalid password, got nil")
		}
	})
}

func TestGenerateAPIKey(t *testing.T) {
	repo := newMockRepository()
	service := svc.New(repo)

	// Register a company for API key tests
	name := "API Key Test Company"
	email := "apikey@example.com"
	password := "password"

	company, err := service.Register(context.Background(), name, email, password)
	if err != nil {
		t.Fatalf("failed to register company for API key test: %v", err)
	}

	t.Run("SuccessfulAPIKeyGeneration", func(t *testing.T) {
		apiKey, err := service.GenerateAPIKey(context.Background(), company.ID)
		if err != nil {
			t.Fatalf("failed to generate API key: %v", err)
		}

		if apiKey == "" {
			t.Error("expected API key to be returned, got empty string")
		}

		// Verify the API key was stored in the repository
		updatedCompany, err := repo.GetByID(context.Background(), company.ID)
		if err != nil {
			t.Fatalf("failed to get company after API key generation: %v", err)
		}

		if updatedCompany.APIKey == nil {
			t.Fatal("expected APIKey to be set, got nil")
		}
		if *updatedCompany.APIKey != apiKey {
			t.Errorf("expected APIKey %q, got %q", apiKey, *updatedCompany.APIKey)
		}
	})

	t.Run("NonExistentCompany", func(t *testing.T) {
		_, err := service.GenerateAPIKey(context.Background(), -1)
		if err == nil {
			t.Error("expected error when generating API key for non-existent company, got nil")
		}
	})
}

func TestGenerateClientID(t *testing.T) {
	repo := newMockRepository()
	service := svc.New(repo)

	// Register a company for client ID tests
	name := "Client ID Test Company"
	email := "clientid@example.com"
	password := "password"

	company, err := service.Register(context.Background(), name, email, password)
	if err != nil {
		t.Fatalf("failed to register company for client ID test: %v", err)
	}

	t.Run("SuccessfulClientIDGeneration", func(t *testing.T) {
		clientID, err := service.GenerateClientID(context.Background(), company.ID)
		if err != nil {
			t.Fatalf("failed to generate client ID: %v", err)
		}

		if clientID == "" {
			t.Error("expected client ID to be returned, got empty string")
		}

		// Verify the client ID was stored in the repository
		updatedCompany, err := repo.GetByID(context.Background(), company.ID)
		if err != nil {
			t.Fatalf("failed to get company after client ID generation: %v", err)
		}

		if updatedCompany.ClientID == nil {
			t.Fatal("expected ClientID to be set, got nil")
		}
		if *updatedCompany.ClientID != clientID {
			t.Errorf("expected ClientID %q, got %q", clientID, *updatedCompany.ClientID)
		}
	})

	t.Run("NonExistentCompany", func(t *testing.T) {
		_, err := service.GenerateClientID(context.Background(), -1)
		if err == nil {
			t.Error("expected error when generating client ID for non-existent company, got nil")
		}
	})
}
