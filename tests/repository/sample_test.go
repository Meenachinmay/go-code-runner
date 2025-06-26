package repository

import (
	"context"
	"go-code-runner/internal/models"
	"go-code-runner/internal/repository"
	"go-code-runner/tests/helpers"
	"testing"
	"time"
)

func TestProblemCRUD(t *testing.T) {
	db, cleanup := helpers.NewTestDB(t)
	defer cleanup()

	repo := repository.New(db)

	now := time.Now().UTC().Truncate(time.Microsecond)
	problem := models.Problem{
		Title:       "Sample Test Problem",
		Description: "This is a sample test problem",
		Difficulty:  "Easy",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	id, err := repo.CreateProblem(context.Background(), problem)
	if err != nil {
		t.Fatalf("failed to create problem: %v", err)
	}

	retrievedProblem, err := repo.GetProblemByID(context.Background(), id)
	if err != nil {
		t.Fatalf("failed to get problem: %v", err)
	}

	if retrievedProblem.Title != problem.Title {
		t.Errorf("expected title %q, got %q", problem.Title, retrievedProblem.Title)
	}
}
