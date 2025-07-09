package repository

import (
	"context"
	"fmt"
	"go-code-runner/internal/models"
	"go-code-runner/internal/repository/coding_test"
	"go-code-runner/internal/repository/company"
	"go-code-runner/internal/repository/problems"
	"go-code-runner/tests/helpers"
	"testing"
	"time"
)

func TestCodingTestRepository(t *testing.T) {
	db, cleanup := helpers.NewTestDB(t)
	defer cleanup()

	// Create a company first to satisfy the foreign key constraint
	companyRepo := company.New(db)
	// Use a unique email to avoid conflicts
	uniqueEmail := fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())
	testCompany := &models.Company{
		Name:         "Test Company",
		Email:        uniqueEmail,
		PasswordHash: "password_hash",
	}
	createdCompany, err := companyRepo.Create(context.Background(), testCompany)
	if err != nil {
		t.Fatalf("failed to create test company: %v", err)
	}

	// Set API key and client ID with unique values
	uniqueAPIKey := fmt.Sprintf("api-key-%d", time.Now().UnixNano())
	err = companyRepo.UpdateAPIKey(context.Background(), createdCompany.ID, uniqueAPIKey)
	if err != nil {
		t.Fatalf("failed to update API key: %v", err)
	}
	uniqueClientID := fmt.Sprintf("client-id-%d", time.Now().UnixNano())
	err = companyRepo.UpdateClientID(context.Background(), createdCompany.ID, uniqueClientID)
	if err != nil {
		t.Fatalf("failed to update client ID: %v", err)
	}

	// Create a problem first to satisfy the foreign key constraint
	problemRepo := problems.NewProblemRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	testProblem := models.Problem{
		Title:       "Test Problem",
		Description: "This is a test problem",
		Difficulty:  "Easy",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	problemID, err := problemRepo.CreateProblem(context.Background(), testProblem)
	if err != nil {
		t.Fatalf("failed to create test problem: %v", err)
	}

	repo := coding_test.New(db)

	createTestCodingTest := func(t *testing.T) *models.CodingTest {
		testID := "test-" + time.Now().Format("20060102150405.000000")
		test := &models.CodingTest{
			ID:                 testID,
			CompanyID:          createdCompany.ID,
			ProblemID:          problemID,
			Status:             models.TestStatusPending,
			ExpiresAt:          time.Now().Add(24 * time.Hour),
			TestDurationMinutes: 60,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}

		err := repo.CreateTest(context.Background(), test)
		if err != nil {
			t.Fatalf("failed to create test coding test: %v", err)
		}

		return test
	}

	t.Run("CreateTest", func(t *testing.T) {
		testID := "test-create-" + time.Now().Format("20060102150405")
		test := &models.CodingTest{
			ID:                 testID,
			CompanyID:          createdCompany.ID,
			ProblemID:          problemID,
			Status:             models.TestStatusPending,
			ExpiresAt:          time.Now().Add(24 * time.Hour),
			TestDurationMinutes: 60,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}

		err := repo.CreateTest(context.Background(), test)
		if err != nil {
			t.Fatalf("failed to create coding test: %v", err)
		}

		retrievedTest, err := repo.GetTestByID(context.Background(), testID)
		if err != nil {
			t.Fatalf("failed to retrieve created test: %v", err)
		}

		if retrievedTest.ID != testID {
			t.Errorf("expected ID %s, got %s", testID, retrievedTest.ID)
		}
		if retrievedTest.CompanyID != createdCompany.ID {
			t.Errorf("expected CompanyID %d, got %d", createdCompany.ID, retrievedTest.CompanyID)
		}
		if retrievedTest.ProblemID != problemID {
			t.Errorf("expected ProblemID %d, got %d", problemID, retrievedTest.ProblemID)
		}
		if retrievedTest.Status != models.TestStatusPending {
			t.Errorf("expected Status %s, got %s", models.TestStatusPending, retrievedTest.Status)
		}
	})

	t.Run("GetTestByID", func(t *testing.T) {
		test := createTestCodingTest(t)

		retrievedTest, err := repo.GetTestByID(context.Background(), test.ID)
		if err != nil {
			t.Fatalf("failed to get test by ID: %v", err)
		}

		if retrievedTest.ID != test.ID {
			t.Errorf("expected ID %s, got %s", test.ID, retrievedTest.ID)
		}
		if retrievedTest.CompanyID != test.CompanyID {
			t.Errorf("expected CompanyID %d, got %d", test.CompanyID, retrievedTest.CompanyID)
		}
		if retrievedTest.ProblemID != test.ProblemID {
			t.Errorf("expected ProblemID %d, got %d", test.ProblemID, retrievedTest.ProblemID)
		}
		if retrievedTest.Status != test.Status {
			t.Errorf("expected Status %s, got %s", test.Status, retrievedTest.Status)
		}
	})

	t.Run("Update", func(t *testing.T) {
		test := createTestCodingTest(t)

		candidateName := "Test Candidate"
		candidateEmail := "test@example.com"
		submissionCode := "console.log('Hello, World!');"
		passedPercentage := 75

		test.CandidateName = &candidateName
		test.CandidateEmail = &candidateEmail
		test.Status = models.TestStatusCompleted
		now := time.Now()
		test.StartedAt = &now
		test.CompletedAt = &now
		test.SubmissionCode = &submissionCode
		test.PassedPercentage = &passedPercentage

		err := repo.Update(context.Background(), test)
		if err != nil {
			t.Fatalf("failed to update test: %v", err)
		}

		retrievedTest, err := repo.GetTestByID(context.Background(), test.ID)
		if err != nil {
			t.Fatalf("failed to get updated test: %v", err)
		}

		if retrievedTest.Status != models.TestStatusCompleted {
			t.Errorf("expected Status %s, got %s", models.TestStatusCompleted, retrievedTest.Status)
		}
		if *retrievedTest.CandidateName != candidateName {
			t.Errorf("expected CandidateName %s, got %s", candidateName, *retrievedTest.CandidateName)
		}
		if *retrievedTest.CandidateEmail != candidateEmail {
			t.Errorf("expected CandidateEmail %s, got %s", candidateEmail, *retrievedTest.CandidateEmail)
		}
		if *retrievedTest.SubmissionCode != submissionCode {
			t.Errorf("expected SubmissionCode %s, got %s", submissionCode, *retrievedTest.SubmissionCode)
		}
		if *retrievedTest.PassedPercentage != passedPercentage {
			t.Errorf("expected PassedPercentage %d, got %d", passedPercentage, *retrievedTest.PassedPercentage)
		}
	})

	t.Run("ExpireOldTests", func(t *testing.T) {
		testID := "test-expire-" + time.Now().Format("20060102150405.000000")
		startedTime := time.Now().Add(-2 * time.Hour)
		test := &models.CodingTest{
			ID:                 testID,
			CompanyID:          createdCompany.ID,
			ProblemID:          problemID,
			Status:             models.TestStatusStarted,
			StartedAt:          &startedTime,
			ExpiresAt:          time.Now().Add(24 * time.Hour),
			TestDurationMinutes: 60,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}

		err := repo.CreateTest(context.Background(), test)
		if err != nil {
			t.Fatalf("failed to create test for expiration: %v", err)
		}

		query := `UPDATE coding_tests SET status = $1, updated_at = $2 WHERE id = $3`
		_, err = db.Exec(context.Background(), query, models.TestStatusExpired, time.Now(), testID)
		if err != nil {
			t.Fatalf("failed to directly update test status: %v", err)
		}

		retrievedTest, err := repo.GetTestByID(context.Background(), testID)
		if err != nil {
			t.Fatalf("failed to get test after expiration: %v", err)
		}

		if retrievedTest.Status != models.TestStatusExpired {
			t.Errorf("expected Status %s, got %s", models.TestStatusExpired, retrievedTest.Status)
		}
	})

	t.Run("GetByCompanyID", func(t *testing.T) {
		companyID := createdCompany.ID
		for i := 0; i < 3; i++ {
			testID := "test-company-" + time.Now().Format("20060102150405.000000") + "-" + string(rune('a'+i))
			test := &models.CodingTest{
				ID:                 testID,
				CompanyID:          companyID,
				ProblemID:          problemID,
				Status:             models.TestStatusPending,
				ExpiresAt:          time.Now().Add(24 * time.Hour),
				TestDurationMinutes: 60,
				CreatedAt:          time.Now(),
				UpdatedAt:          time.Now(),
			}

			err := repo.CreateTest(context.Background(), test)
			if err != nil {
				t.Fatalf("failed to create test for company %d: %v", companyID, err)
			}
		}

		tests, err := repo.GetByCompanyID(context.Background(), companyID)
		if err != nil {
			t.Fatalf("failed to get tests for company %d: %v", companyID, err)
		}

		if len(tests) < 3 {
			t.Errorf("expected at least 3 tests for company %d, got %d", companyID, len(tests))
		}

		for _, test := range tests {
			if test.CompanyID != companyID {
				t.Errorf("expected CompanyID %d, got %d", companyID, test.CompanyID)
			}
		}
	})
}
