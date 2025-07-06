package coding_test

import (
	"context"
	"errors"
	"go-code-runner/internal/models"
	svc "go-code-runner/internal/service/coding_test"
	"testing"
	"time"
)

// Mock coding test repository
type mockCodingTestRepository struct {
	tests map[string]*models.CodingTest
}

func newMockCodingTestRepository() *mockCodingTestRepository {
	return &mockCodingTestRepository{
		tests: make(map[string]*models.CodingTest),
	}
}

func (m *mockCodingTestRepository) CreateTest(ctx context.Context, test *models.CodingTest) error {
	if _, exists := m.tests[test.ID]; exists {
		return errors.New("test already exists")
	}
	m.tests[test.ID] = test
	return nil
}

func (m *mockCodingTestRepository) GetTestByID(ctx context.Context, id string) (*models.CodingTest, error) {
	test, exists := m.tests[id]
	if !exists {
		return nil, errors.New("test not found")
	}
	return test, nil
}

func (m *mockCodingTestRepository) Update(ctx context.Context, test *models.CodingTest) error {
	if _, exists := m.tests[test.ID]; !exists {
		return errors.New("test not found")
	}
	test.UpdatedAt = time.Now()
	m.tests[test.ID] = test
	return nil
}

func (m *mockCodingTestRepository) ExpireOldTests(ctx context.Context) error {
	now := time.Now()
	for id, test := range m.tests {
		if test.Status == models.TestStatusStarted && test.StartedAt != nil {
			expiryTime := test.StartedAt.Add(time.Duration(test.TestDurationMinutes) * time.Minute)
			if now.After(expiryTime) {
				test.Status = models.TestStatusExpired
				test.UpdatedAt = now
				m.tests[id] = test
			}
		}
	}
	return nil
}

func (m *mockCodingTestRepository) GetByCompanyID(ctx context.Context, companyID int) ([]*models.CodingTest, error) {
	var result []*models.CodingTest
	for _, test := range m.tests {
		if test.CompanyID == companyID {
			result = append(result, test)
		}
	}
	return result, nil
}

// Mock problem repository
type mockProblemRepository struct {
	problems map[int]*models.Problem
}

func newMockProblemRepository() *mockProblemRepository {
	return &mockProblemRepository{
		problems: map[int]*models.Problem{
			1: {
				ID:          1,
				Title:       "Test Problem",
				Description: "Test Description",
				Difficulty:  "Easy",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
	}
}

func (m *mockProblemRepository) GetProblemByID(ctx context.Context, id int) (*models.Problem, error) {
	problem, exists := m.problems[id]
	if !exists {
		return nil, errors.New("problem not found")
	}
	return problem, nil
}

func (m *mockProblemRepository) CreateProblem(ctx context.Context, p models.Problem) (int, error) {
	id := len(m.problems) + 1
	p.ID = id
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	m.problems[id] = &p
	return id, nil
}

func (m *mockProblemRepository) ListProblems(ctx context.Context) ([]*models.Problem, error) {
	var problems []*models.Problem
	for _, p := range m.problems {
		problems = append(problems, p)
	}
	return problems, nil
}

// Mock company repository
type mockCompanyRepository struct {
	companies map[int]*models.Company
	apiKeys   map[string]int // Map API keys to company IDs
}

func newMockCompanyRepository() *mockCompanyRepository {
	clientID := "test-client-id"
	apiKey := "test-api-key"
	return &mockCompanyRepository{
		companies: map[int]*models.Company{
			1: {
				ID:           1,
				Name:         "Test Company",
				Email:        "test@example.com",
				PasswordHash: "hashed_password",
				ClientID:     &clientID,
				APIKey:       &apiKey,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
		},
		apiKeys: map[string]int{
			"test-api-key": 1,
		},
	}
}

func (m *mockCompanyRepository) GetByID(ctx context.Context, id int) (*models.Company, error) {
	company, exists := m.companies[id]
	if !exists {
		return nil, errors.New("company not found")
	}
	return company, nil
}

func (m *mockCompanyRepository) GetCompanyByAPIKey(ctx context.Context, apiKey string) (*models.Company, error) {
	companyID, exists := m.apiKeys[apiKey]
	if !exists {
		return nil, errors.New("invalid API key")
	}
	return m.GetByID(ctx, companyID)
}

// Implement only the methods needed for the tests
func (m *mockCompanyRepository) Create(ctx context.Context, c *models.Company) (*models.Company, error) {
	return nil, nil
}

func (m *mockCompanyRepository) GetByEmail(ctx context.Context, email string) (*models.Company, error) {
	return nil, nil
}

func (m *mockCompanyRepository) UpdateAPIKey(ctx context.Context, id int, apiKey string) error {
	return nil
}

func (m *mockCompanyRepository) UpdateClientID(ctx context.Context, id int, clientID string) error {
	return nil
}

// Tests
func TestGenerateTest(t *testing.T) {
	codingTestRepo := newMockCodingTestRepository()
	problemRepo := newMockProblemRepository()
	companyRepo := newMockCompanyRepository()
	baseURL := "http://example.com"

	service := svc.New(codingTestRepo, problemRepo, companyRepo, baseURL)

	t.Run("SuccessfulGeneration", func(t *testing.T) {
		companyID := 1
		problemID := 1
		expiresInHours := 24

		test, link, err := service.GenerateTest(context.Background(), companyID, problemID, expiresInHours)
		if err != nil {
			t.Fatalf("failed to generate test: %v", err)
		}

		if test == nil {
			t.Fatal("expected test to be returned, got nil")
		}
		if test.CompanyID != companyID {
			t.Errorf("expected CompanyID %d, got %d", companyID, test.CompanyID)
		}
		if test.ProblemID != problemID {
			t.Errorf("expected ProblemID %d, got %d", problemID, test.ProblemID)
		}
		if test.Status != models.TestStatusPending {
			t.Errorf("expected Status %s, got %s", models.TestStatusPending, test.Status)
		}
		if link == "" {
			t.Error("expected link to be returned, got empty string")
		}
	})

	t.Run("ProblemNotFound", func(t *testing.T) {
		companyID := 1
		problemID := 999 // Non-existent problem
		expiresInHours := 24

		_, _, err := service.GenerateTest(context.Background(), companyID, problemID, expiresInHours)
		if err == nil {
			t.Error("expected error when problem not found, got nil")
		}
	})

	t.Run("CompanyNotFound", func(t *testing.T) {
		companyID := 999 // Non-existent company
		problemID := 1
		expiresInHours := 24

		_, _, err := service.GenerateTest(context.Background(), companyID, problemID, expiresInHours)
		if err == nil {
			t.Error("expected error when company not found, got nil")
		}
	})
}

func TestVerifyTest(t *testing.T) {
	codingTestRepo := newMockCodingTestRepository()
	problemRepo := newMockProblemRepository()
	companyRepo := newMockCompanyRepository()
	baseURL := "http://example.com"

	service := svc.New(codingTestRepo, problemRepo, companyRepo, baseURL)

	// Create a test for verification
	testID := "test-verify"
	test := &models.CodingTest{
		ID:                 testID,
		CompanyID:          1,
		ProblemID:          1,
		Status:             models.TestStatusPending,
		ExpiresAt:          time.Now().Add(24 * time.Hour),
		TestDurationMinutes: 60,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	err := codingTestRepo.CreateTest(context.Background(), test)
	if err != nil {
		t.Fatalf("failed to create test for verification: %v", err)
	}

	t.Run("ValidTest", func(t *testing.T) {
		verifiedTest, err := service.VerifyTest(context.Background(), testID)
		if err != nil {
			t.Fatalf("failed to verify test: %v", err)
		}

		if verifiedTest == nil {
			t.Fatal("expected test to be returned, got nil")
		}
		if verifiedTest.ID != testID {
			t.Errorf("expected ID %s, got %s", testID, verifiedTest.ID)
		}
	})

	t.Run("TestNotFound", func(t *testing.T) {
		_, err := service.VerifyTest(context.Background(), "non-existent-test")
		if err == nil {
			t.Error("expected error when test not found, got nil")
		}
	})

	t.Run("ExpiredTest", func(t *testing.T) {
		expiredTestID := "test-expired"
		expiredTest := &models.CodingTest{
			ID:                 expiredTestID,
			CompanyID:          1,
			ProblemID:          1,
			Status:             models.TestStatusPending,
			ExpiresAt:          time.Now().Add(-24 * time.Hour), // Expired
			TestDurationMinutes: 60,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
		err := codingTestRepo.CreateTest(context.Background(), expiredTest)
		if err != nil {
			t.Fatalf("failed to create expired test: %v", err)
		}

		_, err = service.VerifyTest(context.Background(), expiredTestID)
		if err == nil {
			t.Error("expected error when test expired, got nil")
		}
	})
}

func TestStartTest(t *testing.T) {
	codingTestRepo := newMockCodingTestRepository()
	problemRepo := newMockProblemRepository()
	companyRepo := newMockCompanyRepository()
	baseURL := "http://example.com"

	service := svc.New(codingTestRepo, problemRepo, companyRepo, baseURL)

	// Create a test for starting
	testID := "test-start"
	test := &models.CodingTest{
		ID:                 testID,
		CompanyID:          1,
		ProblemID:          1,
		Status:             models.TestStatusPending,
		ExpiresAt:          time.Now().Add(24 * time.Hour),
		TestDurationMinutes: 60,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	err := codingTestRepo.CreateTest(context.Background(), test)
	if err != nil {
		t.Fatalf("failed to create test for starting: %v", err)
	}

	t.Run("SuccessfulStart", func(t *testing.T) {
		candidateName := "Test Candidate"
		candidateEmail := "candidate@example.com"

		err := service.StartTest(context.Background(), testID, candidateName, candidateEmail)
		if err != nil {
			t.Fatalf("failed to start test: %v", err)
		}

		// Verify the test was updated
		startedTest, err := codingTestRepo.GetTestByID(context.Background(), testID)
		if err != nil {
			t.Fatalf("failed to get started test: %v", err)
		}

		if startedTest.Status != models.TestStatusStarted {
			t.Errorf("expected Status %s, got %s", models.TestStatusStarted, startedTest.Status)
		}
		if startedTest.StartedAt == nil {
			t.Error("expected StartedAt to be set, got nil")
		}
		if *startedTest.CandidateName != candidateName {
			t.Errorf("expected CandidateName %s, got %s", candidateName, *startedTest.CandidateName)
		}
		if *startedTest.CandidateEmail != candidateEmail {
			t.Errorf("expected CandidateEmail %s, got %s", candidateEmail, *startedTest.CandidateEmail)
		}
	})

	t.Run("AlreadyStarted", func(t *testing.T) {
		// Create a test that's already started
		startedTestID := "test-already-started"
		now := time.Now()
		candidateName := "Already Started"
		candidateEmail := "already@example.com"
		startedTest := &models.CodingTest{
			ID:                 startedTestID,
			CompanyID:          1,
			ProblemID:          1,
			Status:             models.TestStatusStarted,
			StartedAt:          &now,
			CandidateName:      &candidateName,
			CandidateEmail:     &candidateEmail,
			ExpiresAt:          time.Now().Add(24 * time.Hour),
			TestDurationMinutes: 60,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
		err := codingTestRepo.CreateTest(context.Background(), startedTest)
		if err != nil {
			t.Fatalf("failed to create already started test: %v", err)
		}

		err = service.StartTest(context.Background(), startedTestID, "New Candidate", "new@example.com")
		if err == nil {
			t.Error("expected error when test already started, got nil")
		}
	})
}

func TestSubmitTest(t *testing.T) {
	codingTestRepo := newMockCodingTestRepository()
	problemRepo := newMockProblemRepository()
	companyRepo := newMockCompanyRepository()
	baseURL := "http://example.com"

	service := svc.New(codingTestRepo, problemRepo, companyRepo, baseURL)

	// Create a started test for submission
	testID := "test-submit"
	now := time.Now()
	candidateName := "Submit Candidate"
	candidateEmail := "submit@example.com"
	test := &models.CodingTest{
		ID:                 testID,
		CompanyID:          1,
		ProblemID:          1,
		Status:             models.TestStatusStarted,
		StartedAt:          &now,
		CandidateName:      &candidateName,
		CandidateEmail:     &candidateEmail,
		ExpiresAt:          time.Now().Add(24 * time.Hour),
		TestDurationMinutes: 60,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	err := codingTestRepo.CreateTest(context.Background(), test)
	if err != nil {
		t.Fatalf("failed to create test for submission: %v", err)
	}

	t.Run("SuccessfulSubmission", func(t *testing.T) {
		code := "console.log('Hello, World!');"
		passedPercentage := 80

		err := service.SubmitTest(context.Background(), testID, code, passedPercentage)
		if err != nil {
			t.Fatalf("failed to submit test: %v", err)
		}

		// Verify the test was updated
		submittedTest, err := codingTestRepo.GetTestByID(context.Background(), testID)
		if err != nil {
			t.Fatalf("failed to get submitted test: %v", err)
		}

		if submittedTest.Status != models.TestStatusCompleted {
			t.Errorf("expected Status %s, got %s", models.TestStatusCompleted, submittedTest.Status)
		}
		if submittedTest.CompletedAt == nil {
			t.Error("expected CompletedAt to be set, got nil")
		}
		if *submittedTest.SubmissionCode != code {
			t.Errorf("expected SubmissionCode %s, got %s", code, *submittedTest.SubmissionCode)
		}
		if *submittedTest.PassedPercentage != passedPercentage {
			t.Errorf("expected PassedPercentage %d, got %d", passedPercentage, *submittedTest.PassedPercentage)
		}
	})

	t.Run("TestNotInProgress", func(t *testing.T) {
		// Create a test that's not in progress
		pendingTestID := "test-not-in-progress"
		pendingTest := &models.CodingTest{
			ID:                 pendingTestID,
			CompanyID:          1,
			ProblemID:          1,
			Status:             models.TestStatusPending,
			ExpiresAt:          time.Now().Add(24 * time.Hour),
			TestDurationMinutes: 60,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
		err := codingTestRepo.CreateTest(context.Background(), pendingTest)
		if err != nil {
			t.Fatalf("failed to create pending test: %v", err)
		}

		err = service.SubmitTest(context.Background(), pendingTestID, "code", 50)
		if err == nil {
			t.Error("expected error when test not in progress, got nil")
		}
	})

	t.Run("TestExpired", func(t *testing.T) {
		// Create a test that's expired
		expiredTestID := "test-expired-submit"
		startedTime := time.Now().Add(-2 * time.Hour) // Started 2 hours ago
		candidateName := "Expired Candidate"
		candidateEmail := "expired@example.com"
		expiredTest := &models.CodingTest{
			ID:                 expiredTestID,
			CompanyID:          1,
			ProblemID:          1,
			Status:             models.TestStatusStarted,
			StartedAt:          &startedTime,
			CandidateName:      &candidateName,
			CandidateEmail:     &candidateEmail,
			ExpiresAt:          time.Now().Add(24 * time.Hour),
			TestDurationMinutes: 60, // 1 hour duration, so it's expired
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
		err := codingTestRepo.CreateTest(context.Background(), expiredTest)
		if err != nil {
			t.Fatalf("failed to create expired test: %v", err)
		}

		err = service.SubmitTest(context.Background(), expiredTestID, "code", 50)
		if err == nil {
			t.Error("expected error when test expired, got nil")
		}
	})
}

func TestGetCompanyTests(t *testing.T) {
	codingTestRepo := newMockCodingTestRepository()
	problemRepo := newMockProblemRepository()
	companyRepo := newMockCompanyRepository()
	baseURL := "http://example.com"

	service := svc.New(codingTestRepo, problemRepo, companyRepo, baseURL)

	// Create tests for a company
	companyID := 1
	for i := 0; i < 3; i++ {
		testID := "test-company-" + string(rune('a'+i))
		test := &models.CodingTest{
			ID:                 testID,
			CompanyID:          companyID,
			ProblemID:          1,
			Status:             models.TestStatusPending,
			ExpiresAt:          time.Now().Add(24 * time.Hour),
			TestDurationMinutes: 60,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
		err := codingTestRepo.CreateTest(context.Background(), test)
		if err != nil {
			t.Fatalf("failed to create test for company %d: %v", companyID, err)
		}
	}

	t.Run("GetCompanyTests", func(t *testing.T) {
		tests, err := service.GetCompanyTests(context.Background(), companyID)
		if err != nil {
			t.Fatalf("failed to get company tests: %v", err)
		}

		if len(tests) != 3 {
			t.Errorf("expected 3 tests, got %d", len(tests))
		}

		// Verify all tests belong to the company
		for _, test := range tests {
			if test.CompanyID != companyID {
				t.Errorf("expected CompanyID %d, got %d", companyID, test.CompanyID)
			}
		}
	})

	t.Run("NoTests", func(t *testing.T) {
		emptyCompanyID := 999
		tests, err := service.GetCompanyTests(context.Background(), emptyCompanyID)
		if err != nil {
			t.Fatalf("failed to get company tests: %v", err)
		}

		if len(tests) != 0 {
			t.Errorf("expected 0 tests, got %d", len(tests))
		}
	})
}
