package repository

import (
	"context"
	"go-code-runner/internal/models"
	"go-code-runner/internal/repository"
	"go-code-runner/tests/helpers"
	"testing"
	"time"
)

func TestTestCaseRepository(t *testing.T) {
	db, cleanup := helpers.NewTestDB(t)
	defer cleanup()

	repo := repository.New(db)

	now := time.Now().UTC().Truncate(time.Microsecond)
	problem := models.Problem{
		Title:       "Test Problem for Test Cases",
		Description: "This problem is used for testing test cases",
		Difficulty:  "Medium",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	problemID, err := repo.CreateProblem(context.Background(), problem)
	if err != nil {
		t.Fatalf("failed to create problem for test cases: %v", err)
	}

	t.Run("CreateTestCase", func(t *testing.T) {
		testCase := models.TestCase{
			ProblemID:      problemID,
			Input:          "test input",
			ExpectedOutput: "expected output",
			IsHidden:       false,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		id, err := repo.CreateTestCase(context.Background(), testCase)
		if err != nil {
			t.Fatalf("failed to create test case: %v", err)
		}

		if id <= 0 {
			t.Fatalf("expected positive ID, got %d", id)
		}

		testCases, err := repo.GetTestCasesByProblemID(context.Background(), problemID)
		if err != nil {
			t.Fatalf("failed to get test cases: %v", err)
		}

		if len(testCases) == 0 {
			t.Fatalf("no test cases found for problem ID %d", problemID)
		}

		found := false
		for _, tc := range testCases {
			if tc.ID == id {
				found = true
				if tc.ProblemID != testCase.ProblemID {
					t.Errorf("expected problem ID %d, got %d", testCase.ProblemID, tc.ProblemID)
				}
				if tc.Input != testCase.Input {
					t.Errorf("expected input %q, got %q", testCase.Input, tc.Input)
				}
				if tc.ExpectedOutput != testCase.ExpectedOutput {
					t.Errorf("expected output %q, got %q", testCase.ExpectedOutput, tc.ExpectedOutput)
				}
				if tc.IsHidden != testCase.IsHidden {
					t.Errorf("expected IsHidden %v, got %v", testCase.IsHidden, tc.IsHidden)
				}
				break
			}
		}

		if !found {
			t.Errorf("created test case with ID %d not found in retrieved test cases", id)
		}
	})

	t.Run("GetTestCasesByProblemID", func(t *testing.T) {
		testCases := []models.TestCase{
			{
				ProblemID:      problemID,
				Input:          "input 1",
				ExpectedOutput: "output 1",
				IsHidden:       false,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			{
				ProblemID:      problemID,
				Input:          "input 2",
				ExpectedOutput: "output 2",
				IsHidden:       true,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		}

		var testCaseIDs []int
		for _, tc := range testCases {
			id, err := repo.CreateTestCase(context.Background(), tc)
			if err != nil {
				t.Fatalf("failed to create test case for GetTestCasesByProblemID test: %v", err)
			}
			testCaseIDs = append(testCaseIDs, id)
		}

		retrievedTestCases, err := repo.GetTestCasesByProblemID(context.Background(), problemID)
		if err != nil {
			t.Fatalf("failed to get test cases by problem ID: %v", err)
		}

		if len(retrievedTestCases) < len(testCases) {
			t.Errorf("expected at least %d test cases, got %d", len(testCases), len(retrievedTestCases))
		}

		foundCount := 0
		for _, id := range testCaseIDs {
			for _, tc := range retrievedTestCases {
				if tc.ID == id {
					foundCount++
					break
				}
			}
		}

		if foundCount < len(testCaseIDs) {
			t.Errorf("not all created test cases were found, found %d of %d", foundCount, len(testCaseIDs))
		}

		nonExistentProblemID := -1
		testCasesForNonExistentProblem, err := repo.GetTestCasesByProblemID(context.Background(), nonExistentProblemID)
		if err != nil {
			t.Fatalf("unexpected error when retrieving test cases for non-existent problem: %v", err)
		}

		if len(testCasesForNonExistentProblem) > 0 {
			t.Errorf("expected no test cases for non-existent problem, got %d", len(testCasesForNonExistentProblem))
		}
	})
}
