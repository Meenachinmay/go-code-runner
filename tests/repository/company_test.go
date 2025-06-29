package repository

import (
	"context"
	"fmt"
	"go-code-runner/internal/models"
	"go-code-runner/internal/repository/company"
	"go-code-runner/tests/helpers"
	"testing"
	"time"
)

func TestCompanyRepository(t *testing.T) {
	db, cleanup := helpers.NewTestDB(t)
	defer cleanup()

	repo := company.New(db)

	uniqueEmail := func(prefix string) string {
		return fmt.Sprintf("%s-%d@example.com", prefix, time.Now().UnixNano())
	}

	t.Run("Create", func(t *testing.T) {
		testCompany := &models.Company{
			Name:         "Test Company",
			Email:        uniqueEmail("test"),
			PasswordHash: "hashed_password",
		}

		createdCompany, err := repo.Create(context.Background(), testCompany)
		if err != nil {
			t.Fatalf("failed to create company: %v", err)
		}

		if createdCompany.ID <= 0 {
			t.Fatalf("expected positive ID, got %d", createdCompany.ID)
		}

		if createdCompany.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set, got zero time")
		}
		if createdCompany.UpdatedAt.IsZero() {
			t.Error("expected UpdatedAt to be set, got zero time")
		}

		apiKey := fmt.Sprintf("test-api-key-%d", time.Now().UnixNano())
		err = repo.UpdateAPIKey(context.Background(), createdCompany.ID, apiKey)
		if err != nil {
			t.Fatalf("failed to update API key: %v", err)
		}

		clientID := fmt.Sprintf("test-client-id-%d", time.Now().UnixNano())
		err = repo.UpdateClientID(context.Background(), createdCompany.ID, clientID)
		if err != nil {
			t.Fatalf("failed to update client ID: %v", err)
		}

		retrievedCompany, err := repo.GetByID(context.Background(), createdCompany.ID)
		if err != nil {
			t.Fatalf("failed to get created company: %v", err)
		}

		if retrievedCompany.ID != createdCompany.ID {
			t.Errorf("expected ID %d, got %d", createdCompany.ID, retrievedCompany.ID)
		}
		if retrievedCompany.Name != testCompany.Name {
			t.Errorf("expected Name %q, got %q", testCompany.Name, retrievedCompany.Name)
		}
		if retrievedCompany.Email != testCompany.Email {
			t.Errorf("expected Email %q, got %q", testCompany.Email, retrievedCompany.Email)
		}
		if retrievedCompany.PasswordHash != testCompany.PasswordHash {
			t.Errorf("expected PasswordHash %q, got %q", testCompany.PasswordHash, retrievedCompany.PasswordHash)
		}

		if retrievedCompany.APIKey == nil {
			t.Fatal("expected APIKey to be set, got nil")
		}
		if *retrievedCompany.APIKey != apiKey {
			t.Errorf("expected APIKey %q, got %q", apiKey, *retrievedCompany.APIKey)
		}

		if retrievedCompany.ClientID == nil {
			t.Fatal("expected ClientID to be set, got nil")
		}
		if *retrievedCompany.ClientID != clientID {
			t.Errorf("expected ClientID %q, got %q", clientID, *retrievedCompany.ClientID)
		}
	})

	t.Run("GetByEmail", func(t *testing.T) {
		email := uniqueEmail("get-by-email")
		testCompany := &models.Company{
			Name:         "Get By Email Company",
			Email:        email,
			PasswordHash: "hashed_password",
		}

		createdCompany, err := repo.Create(context.Background(), testCompany)
		if err != nil {
			t.Fatalf("failed to create company for GetByEmail test: %v", err)
		}

		apiKey := fmt.Sprintf("test-api-key-email-%d", time.Now().UnixNano())
		err = repo.UpdateAPIKey(context.Background(), createdCompany.ID, apiKey)
		if err != nil {
			t.Fatalf("failed to update API key: %v", err)
		}

		clientID := fmt.Sprintf("test-client-id-email-%d", time.Now().UnixNano())
		err = repo.UpdateClientID(context.Background(), createdCompany.ID, clientID)
		if err != nil {
			t.Fatalf("failed to update client ID: %v", err)
		}

		retrievedCompany, err := repo.GetByEmail(context.Background(), email)
		if err != nil {
			t.Fatalf("failed to get company by email: %v", err)
		}

		if retrievedCompany.ID != createdCompany.ID {
			t.Errorf("expected ID %d, got %d", createdCompany.ID, retrievedCompany.ID)
		}
		if retrievedCompany.Name != testCompany.Name {
			t.Errorf("expected Name %q, got %q", testCompany.Name, retrievedCompany.Name)
		}
		if retrievedCompany.Email != testCompany.Email {
			t.Errorf("expected Email %q, got %q", testCompany.Email, retrievedCompany.Email)
		}
		if retrievedCompany.PasswordHash != testCompany.PasswordHash {
			t.Errorf("expected PasswordHash %q, got %q", testCompany.PasswordHash, retrievedCompany.PasswordHash)
		}

		if retrievedCompany.APIKey == nil {
			t.Fatal("expected APIKey to be set, got nil")
		}
		if *retrievedCompany.APIKey != apiKey {
			t.Errorf("expected APIKey %q, got %q", apiKey, *retrievedCompany.APIKey)
		}

		if retrievedCompany.ClientID == nil {
			t.Fatal("expected ClientID to be set, got nil")
		}
		if *retrievedCompany.ClientID != clientID {
			t.Errorf("expected ClientID %q, got %q", clientID, *retrievedCompany.ClientID)
		}

		_, err = repo.GetByEmail(context.Background(), "nonexistent@example.com")
		if err == nil {
			t.Error("expected error when getting company with non-existent email, got nil")
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		testCompany := &models.Company{
			Name:         "Get By ID Company",
			Email:        uniqueEmail("get-by-id"),
			PasswordHash: "hashed_password",
		}

		createdCompany, err := repo.Create(context.Background(), testCompany)
		if err != nil {
			t.Fatalf("failed to create company for GetByID test: %v", err)
		}

		apiKey := fmt.Sprintf("test-api-key-id-%d", time.Now().UnixNano())
		err = repo.UpdateAPIKey(context.Background(), createdCompany.ID, apiKey)
		if err != nil {
			t.Fatalf("failed to update API key: %v", err)
		}

		clientID := fmt.Sprintf("test-client-id-id-%d", time.Now().UnixNano())
		err = repo.UpdateClientID(context.Background(), createdCompany.ID, clientID)
		if err != nil {
			t.Fatalf("failed to update client ID: %v", err)
		}

		retrievedCompany, err := repo.GetByID(context.Background(), createdCompany.ID)
		if err != nil {
			t.Fatalf("failed to get company by ID: %v", err)
		}

		if retrievedCompany.ID != createdCompany.ID {
			t.Errorf("expected ID %d, got %d", createdCompany.ID, retrievedCompany.ID)
		}
		if retrievedCompany.Name != testCompany.Name {
			t.Errorf("expected Name %q, got %q", testCompany.Name, retrievedCompany.Name)
		}
		if retrievedCompany.Email != testCompany.Email {
			t.Errorf("expected Email %q, got %q", testCompany.Email, retrievedCompany.Email)
		}
		if retrievedCompany.PasswordHash != testCompany.PasswordHash {
			t.Errorf("expected PasswordHash %q, got %q", testCompany.PasswordHash, retrievedCompany.PasswordHash)
		}

		if retrievedCompany.APIKey == nil {
			t.Fatal("expected APIKey to be set, got nil")
		}
		if *retrievedCompany.APIKey != apiKey {
			t.Errorf("expected APIKey %q, got %q", apiKey, *retrievedCompany.APIKey)
		}

		if retrievedCompany.ClientID == nil {
			t.Fatal("expected ClientID to be set, got nil")
		}
		if *retrievedCompany.ClientID != clientID {
			t.Errorf("expected ClientID %q, got %q", clientID, *retrievedCompany.ClientID)
		}

		// Test getting a company with a non-existent ID
		_, err = repo.GetByID(context.Background(), -1)
		if err == nil {
			t.Error("expected error when getting company with non-existent ID, got nil")
		}
	})

	t.Run("UpdateAPIKey", func(t *testing.T) {
		// Create a test company
		testCompany := &models.Company{
			Name:         "API Key Company",
			Email:        uniqueEmail("api-key"),
			PasswordHash: "hashed_password",
		}

		// Create the company in the database
		createdCompany, err := repo.Create(context.Background(), testCompany)
		if err != nil {
			t.Fatalf("failed to create company for UpdateAPIKey test: %v", err)
		}

		// Update the API key with a unique value
		apiKey := fmt.Sprintf("test-api-key-%d", time.Now().UnixNano())
		err = repo.UpdateAPIKey(context.Background(), createdCompany.ID, apiKey)
		if err != nil {
			t.Fatalf("failed to update API key: %v", err)
		}

		// Retrieve the company to verify the API key was updated
		updatedCompany, err := repo.GetByID(context.Background(), createdCompany.ID)
		if err != nil {
			t.Fatalf("failed to get company after updating API key: %v", err)
		}

		// Verify the API key was updated
		if updatedCompany.APIKey == nil {
			t.Fatal("expected APIKey to be set, got nil")
		}
		if *updatedCompany.APIKey != apiKey {
			t.Errorf("expected APIKey %q, got %q", apiKey, *updatedCompany.APIKey)
		}

		// Verify updated_at was updated
		if !updatedCompany.UpdatedAt.After(createdCompany.UpdatedAt) {
			t.Error("expected UpdatedAt to be updated, but it wasn't")
		}

		// Test updating a non-existent company
		err = repo.UpdateAPIKey(context.Background(), -1, apiKey)
		if err == nil {
			t.Error("expected error when updating API key for non-existent company, got nil")
		}
	})

	t.Run("UpdateClientID", func(t *testing.T) {
		testCompany := &models.Company{
			Name:         "Client ID Company",
			Email:        uniqueEmail("client-id"),
			PasswordHash: "hashed_password",
		}

		createdCompany, err := repo.Create(context.Background(), testCompany)
		if err != nil {
			t.Fatalf("failed to create company for UpdateClientID test: %v", err)
		}

		clientID := fmt.Sprintf("test-client-id-%d", time.Now().UnixNano())
		err = repo.UpdateClientID(context.Background(), createdCompany.ID, clientID)
		if err != nil {
			t.Fatalf("failed to update client ID: %v", err)
		}

		updatedCompany, err := repo.GetByID(context.Background(), createdCompany.ID)
		if err != nil {
			t.Fatalf("failed to get company after updating client ID: %v", err)
		}

		if updatedCompany.ClientID == nil {
			t.Fatal("expected ClientID to be set, got nil")
		}
		if *updatedCompany.ClientID != clientID {
			t.Errorf("expected ClientID %q, got %q", clientID, *updatedCompany.ClientID)
		}

		if !updatedCompany.UpdatedAt.After(createdCompany.UpdatedAt) {
			t.Error("expected UpdatedAt to be updated, but it wasn't")
		}

		// Test updating a non-existent company
		err = repo.UpdateClientID(context.Background(), -1, clientID)
		if err == nil {
			t.Error("expected error when updating client ID for non-existent company, got nil")
		}
	})
}
