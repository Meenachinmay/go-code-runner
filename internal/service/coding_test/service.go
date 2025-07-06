package coding_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"go-code-runner/internal/models"
	codingtestrepository "go-code-runner/internal/repository/coding_test"
	companyrepository "go-code-runner/internal/repository/company"
	problemrepository "go-code-runner/internal/repository/problems"
	"time"
)

type service struct {
	repo              codingtestrepository.CodingTestRepository
	problemRepository problemrepository.ProblemRepository
	companyRepository companyrepository.Repository
	baseURL           string
}

func New(repo codingtestrepository.CodingTestRepository, problemRepository problemrepository.ProblemRepository, companyRepository companyrepository.Repository, baseURL string) Service {
	return &service{
		repo:              repo,
		problemRepository: problemRepository,
		companyRepository: companyRepository,
		baseURL:           baseURL,
	}
}

func (s *service) GenerateTest(ctx context.Context, companyID, problemID int, expiresInHours int) (*models.CodingTest, string, error) {
	problem, err := s.problemRepository.GetProblemByID(ctx, problemID)
	if err != nil {
		return nil, "", fmt.Errorf("problem not found: %w", err)
	}

	testID := uuid.New().String()

	test := &models.CodingTest{
		ID:                  testID,
		CompanyID:           companyID,
		ProblemID:           problem.ID,
		Status:              models.TestStatusPending,
		ExpiresAt:           time.Now().Add(time.Duration(expiresInHours) * time.Hour),
		TestDurationMinutes: 60,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	if err := s.repo.CreateTest(ctx, test); err != nil {
		return nil, "", fmt.Errorf("failed to create test: %w", err)
	}

	company, err := s.companyRepository.GetByID(ctx, companyID)
	if err != nil {
		return nil, "", fmt.Errorf("company not found: %w", err)
	}

	link := fmt.Sprintf("%s/test/%s?client_id=%s", s.baseURL, testID, *company.ClientID)

	return test, link, nil
}

func (s *service) VerifyTest(ctx context.Context, testID string) (*models.CodingTest, error) {
	test, err := s.repo.GetTestByID(ctx, testID)
	if err != nil {
		return nil, fmt.Errorf("test not found: %w", err)
	}

	if time.Now().After(test.ExpiresAt) {
		return nil, errors.New("test link has expired")
	}

	if test.Status == models.TestStatusCompleted {
		return nil, errors.New("test has already been completed")
	}

	if test.Status == models.TestStatusExpired {
		return nil, errors.New("test has expired")
	}

	if test.Status == models.TestStatusStarted && test.StartedAt != nil {
		expiryTime := test.StartedAt.Add(time.Duration(test.TestDurationMinutes) * time.Minute)
		if time.Now().After(expiryTime) {
			test.Status = models.TestStatusExpired
			_ = s.repo.Update(ctx, test)
			return nil, errors.New("test duration has expired")
		}
	}

	return test, nil
}

func (s *service) StartTest(ctx context.Context, testID, candidateName, candidateEmail string) error {
	test, err := s.VerifyTest(ctx, testID)
	if err != nil {
		return err
	}

	if test.Status != models.TestStatusPending {
		return errors.New("test has already been started")
	}

	now := time.Now()
	test.Status = models.TestStatusStarted
	test.StartedAt = &now
	test.CandidateName = &candidateName
	test.CandidateEmail = &candidateEmail

	return s.repo.Update(ctx, test)
}

func (s *service) SubmitTest(ctx context.Context, testID, code string, passedPercentage int) error {
	test, err := s.repo.GetTestByID(ctx, testID)
	if err != nil {
		return fmt.Errorf("test not found: %w", err)
	}

	if test.Status != models.TestStatusStarted {
		return errors.New("test is not in progress")
	}

	if test.StartedAt != nil {
		expiryTime := test.StartedAt.Add(time.Duration(test.TestDurationMinutes) * time.Minute)
		if time.Now().After(expiryTime) {
			test.Status = models.TestStatusExpired
			_ = s.repo.Update(ctx, test)
			return errors.New("test duration has expired")
		}
	}

	now := time.Now()
	test.Status = models.TestStatusCompleted
	test.CompletedAt = &now
	test.SubmissionCode = &code
	test.PassedPercentage = &passedPercentage

	return s.repo.Update(ctx, test)
}

func (s *service) GetCompanyTests(ctx context.Context, companyID int) ([]*models.CodingTest, error) {
	return s.repo.GetByCompanyID(ctx, companyID)
}
