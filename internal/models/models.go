package models

import (
	"time"
)

// Problem represents a coding problem
type Problem struct {
	ID          int       `json:"id" db:"id"`
	Title       string    `json:"title" db:"title"`
	Description string    `json:"description" db:"description"`
	Difficulty  string    `json:"difficulty" db:"difficulty"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// TestCase represents a test case for a problem
type TestCase struct {
	ID             int       `json:"id" db:"id"`
	ProblemID      int       `json:"problem_id" db:"problem_id"`
	Input          string    `json:"input" db:"input"`
	ExpectedOutput string    `json:"expected_output" db:"expected_output"`
	IsHidden       bool      `json:"is_hidden" db:"is_hidden"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// TestResult represents the result of running a test case
type TestResult struct {
	TestCaseID     int    `json:"test_case_id"`
	Input          string `json:"input,omitempty"`
	ExpectedOutput string `json:"expected_output,omitempty"`
	ActualOutput   string `json:"actual_output"`
	Error          string `json:"error,omitempty"`
	Passed         bool   `json:"passed"`
}

// ExecutionResults represents the results of running code against multiple test cases
type ExecutionResults struct {
	Success    bool         `json:"success"`
	TestResults []TestResult `json:"test_results"`
}