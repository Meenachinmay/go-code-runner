package repository

import (
	"context"
	"go-code-runner/internal/models"
	"go-code-runner/internal/repository"
	"go-code-runner/tests/helpers"
	"testing"
	"time"
)

func TestProblemRepository(t *testing.T) {
	db, cleanup := helpers.NewTestDB(t)
	defer cleanup()

	repo := repository.New(db)

	t.Run("CreateProblem", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Microsecond)
		problem := models.Problem{
			Title:       "Test Problem",
			Description: "This is a test problem",
			Difficulty:  "Easy",
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		id, err := repo.CreateProblem(context.Background(), problem)
		if err != nil {
			t.Fatalf("failed to create problem: %v", err)
		}

		if id <= 0 {
			t.Fatalf("expected positive ID, got %d", id)
		}

		createdProblem, err := repo.GetProblemByID(context.Background(), id)
		if err != nil {
			t.Fatalf("failed to get created problem: %v", err)
		}

		if createdProblem.Title != problem.Title {
			t.Errorf("expected title %q, got %q", problem.Title, createdProblem.Title)
		}
		if createdProblem.Description != problem.Description {
			t.Errorf("expected description %q, got %q", problem.Description, createdProblem.Description)
		}
		if createdProblem.Difficulty != problem.Difficulty {
			t.Errorf("expected difficulty %q, got %q", problem.Difficulty, createdProblem.Difficulty)
		}
	})

	t.Run("GetProblemByID", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Microsecond)
		problem := models.Problem{
			Title:       "Problem to Retrieve",
			Description: "This problem will be retrieved by ID",
			Difficulty:  "Medium",
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		id, err := repo.CreateProblem(context.Background(), problem)
		if err != nil {
			t.Fatalf("failed to create problem for retrieval test: %v", err)
		}

		retrievedProblem, err := repo.GetProblemByID(context.Background(), id)
		if err != nil {
			t.Fatalf("failed to get problem by ID: %v", err)
		}

		if retrievedProblem.ID != id {
			t.Errorf("expected ID %d, got %d", id, retrievedProblem.ID)
		}
		if retrievedProblem.Title != problem.Title {
			t.Errorf("expected title %q, got %q", problem.Title, retrievedProblem.Title)
		}
		if retrievedProblem.Description != problem.Description {
			t.Errorf("expected description %q, got %q", problem.Description, retrievedProblem.Description)
		}
		if retrievedProblem.Difficulty != problem.Difficulty {
			t.Errorf("expected difficulty %q, got %q", problem.Difficulty, retrievedProblem.Difficulty)
		}

		_, err = repo.GetProblemByID(context.Background(), -1)
		if err == nil {
			t.Error("expected error when retrieving non-existent problem, got nil")
		}
	})

	t.Run("ListProblems", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Microsecond)
		problems := []models.Problem{
			{
				Title:       "List Test Problem 1",
				Description: "First problem for list test",
				Difficulty:  "Easy",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				Title:       "List Test Problem 2",
				Description: "Second problem for list test",
				Difficulty:  "Hard",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		}

		for _, p := range problems {
			_, err := repo.CreateProblem(context.Background(), p)
			if err != nil {
				t.Fatalf("failed to create problem for list test: %v", err)
			}
		}

		listedProblems, err := repo.ListProblems(context.Background())
		if err != nil {
			t.Fatalf("failed to list problems: %v", err)
		}

		if len(listedProblems) < len(problems) {
			t.Errorf("expected at least %d problems, got %d", len(problems), len(listedProblems))
		}

		foundCount := 0
		for _, lp := range listedProblems {
			for _, p := range problems {
				if lp.Title == p.Title && lp.Description == p.Description {
					foundCount++
					break
				}
			}
		}

		if foundCount < len(problems) {
			t.Errorf("not all created problems were found in the list, found %d of %d", foundCount, len(problems))
		}
	})
}
