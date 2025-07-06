package code_executor

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go-code-runner/internal/models"
	testcaserepo "go-code-runner/internal/repository/test_cases"
)

type ExecutionResult struct {
	Output string
	Error  string
}

type service struct {
	executionTimeout time.Duration
	logger           *log.Logger
	imageCache       map[string]bool
	repository       testcaserepo.TestCaseRepository

	buildCacheDir string
	modCacheDir   string
	hostTempDir   string
}

func NewService(timeout time.Duration, logger *log.Logger, repo testcaserepo.TestCaseRepository) Service {
	buildCacheDir := "/tmp/runbox/go-build-cache"
	modCacheDir := "/tmp/runbox/go-mod-cache"

	os.MkdirAll(buildCacheDir, 0755)
	os.MkdirAll(modCacheDir, 0755)

	hostTempDir := os.Getenv("HOST_TEMP_DIR") // TODO: Throw error if ENV is not set for HOST_TEMP_DIR
	if hostTempDir == "" {
		hostTempDir = "/tmp/runbox"
	}

	return &service{
		executionTimeout: timeout,
		logger:           logger,
		imageCache:       make(map[string]bool),
		repository:       repo,
		buildCacheDir:    buildCacheDir,
		modCacheDir:      modCacheDir,
		hostTempDir:      hostTempDir,
	}
}

func (s *service) ensureDockerImageAvailable(imageName string) {
	if _, exists := s.imageCache[imageName]; exists {
		return
	}

	s.logger.Printf("Checking if Docker image %s is available locally...", imageName)

	checkCmd := exec.Command("docker", "image", "inspect", imageName)
	if err := checkCmd.Run(); err != nil {
		s.logger.Printf("Docker image %s not found locally, pulling...", imageName)
		pullCmd := exec.Command("docker", "pull", imageName)
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {
			s.logger.Printf("Failed to pull Docker image %s: %v", imageName, err)
		} else {
			s.logger.Printf("Docker image %s pulled successfully", imageName)
		}
	} else {
		s.logger.Printf("Docker image %s is already available locally", imageName)
	}

	s.imageCache[imageName] = true
}

func (s *service) executeCode(ctx context.Context, code string, language string, input string) (*ExecutionResult, error) {
	runID := uuid.New().String()
	s.logger.Printf("[%s] Creating temp directory...", runID)
	dirStart := time.Now()

	apiContainerBaseDir := "/tmp/runbox"
	if err := os.MkdirAll(apiContainerBaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base temp dir: %w", err)
	}

	apiContainerTempDir := filepath.Join(apiContainerBaseDir, "runbox-"+runID)
	if err := os.MkdirAll(apiContainerTempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(apiContainerTempDir)
	s.logger.Printf("[%s] Temp directory created at %s. (took %v)", runID, apiContainerTempDir, time.Since(dirStart))

	s.logger.Printf("[%s] Writing code to file...", runID)
	writeStart := time.Now()

	codeFileName := "main.go"
	codePath := filepath.Join(apiContainerTempDir, codeFileName)
	if err := os.WriteFile(codePath, []byte(code), 0644); err != nil {
		return nil, fmt.Errorf("failed to write code to file: %w", err)
	}
	s.logger.Printf("[%s] Code written to %s. (took %v)", runID, codePath, time.Since(writeStart))

	inputFile := ""
	if input != "" {
		inputFile = filepath.Join(apiContainerTempDir, "input.txt")
		if err := os.WriteFile(inputFile, []byte(input), 0644); err != nil {
			return nil, fmt.Errorf("failed to write input to file: %w", err)
		}
		s.logger.Printf("[%s] Input written to %s", runID, inputFile)
	}

	execCtx, cancel := context.WithTimeout(ctx, s.executionTimeout)
	defer cancel()

	hostPath := strings.Replace(apiContainerTempDir, apiContainerBaseDir, s.hostTempDir, 1)
	volumeMount := fmt.Sprintf("%s:/app", hostPath)

	hostBuildCacheDir := strings.Replace(s.buildCacheDir, "/tmp/runbox", s.hostTempDir, 1)
	hostModCacheDir := strings.Replace(s.modCacheDir, "/tmp/runbox", s.hostTempDir, 1)

	cacheMount := fmt.Sprintf("%s:/root/.cache/go-build:rw", hostBuildCacheDir)
	modMount := fmt.Sprintf("%s:/go/pkg/mod:rw", hostModCacheDir)

	s.logger.Printf("[%s] Container temp dir: %s", runID, apiContainerTempDir)
	s.logger.Printf("[%s] Host mount path: %s", runID, hostPath)

	runCmd := fmt.Sprintf("cd /app && GOFLAGS=-mod=readonly go run %s", codeFileName)

	if inputFile != "" {
		runCmd = fmt.Sprintf("cd /app && cat input.txt | GOFLAGS=-mod=readonly go run %s", codeFileName)
	}

	args := []string{
		"run", "--rm",
		"--network", "none",
		"--memory", "256m",
		"--cpus", "0.5",
		"-v", volumeMount,
		"-v", cacheMount,
		"-v", modMount,
		"-w", "/app",
		"golang:1.22-alpine",
		"sh", "-c", runCmd,
	}

	cmd := exec.CommandContext(execCtx, "docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	s.logger.Printf("[%s] Executing docker command: docker %v", runID, args)
	dockerStart := time.Now()

	err := cmd.Run()

	dockerDuration := time.Since(dockerStart)
	s.logger.Printf("[%s] Docker command finished. (took %v)", runID, dockerDuration)

	if execCtx.Err() == context.DeadlineExceeded {
		s.logger.Printf("[%s] CONTEXT DEADLINE EXCEEDED. Total execution time: %v", runID, dockerDuration)
		return nil, fmt.Errorf("execution timed out after %v", s.executionTimeout)
	}

	result := &ExecutionResult{
		Output: stdout.String(),
		Error:  stderr.String(),
	}

	if err != nil {
		if result.Error == "" {
			result.Error = err.Error()
		}
		s.logger.Printf("[%s] Command failed with error: %s", runID, result.Error)
	} else {
		s.logger.Printf("[%s] Command executed successfully.", runID)
	}

	return result, nil
}

func (s *service) Execute(ctx context.Context, code string, language string) (*ExecutionResult, error) {
	overallStart := time.Now()
	s.logger.Printf("-------------------------------------------------")
	s.logger.Println("Received new execution request.")

	s.ensureDockerImageAvailable("golang:1.22-alpine")

	result, err := s.executeCode(ctx, code, language, "")

	s.logger.Printf("Total request processing time: %v", time.Since(overallStart))
	s.logger.Printf("-------------------------------------------------")

	return result, err
}

func (s *service) ExecuteWithTestCases(ctx context.Context, code string, language string, testCases []*models.TestCase) (*models.ExecutionResults, error) {
	overallStart := time.Now()
	s.logger.Printf("-------------------------------------------------")
	s.logger.Println("Received execution request with test cases.")

	s.ensureDockerImageAvailable("golang:1.22-alpine")

	var testResults []models.TestResult
	success := true

	for _, testCase := range testCases {
		s.logger.Printf("Running test case %d", testCase.ID)

		result, err := s.executeCode(ctx, code, language, testCase.Input)
		if err != nil {
			return nil, err
		}

		actualOutput := strings.TrimSpace(result.Output)
		expectedOutput := strings.TrimSpace(testCase.ExpectedOutput)

		passed := actualOutput == expectedOutput
		if !passed {
			success = false
		}

		testResult := models.TestResult{
			TestCaseID:     testCase.ID,
			Input:          testCase.Input,
			ExpectedOutput: testCase.ExpectedOutput,
			ActualOutput:   actualOutput,
			Error:          result.Error,
			Passed:         passed,
		}

		if testCase.IsHidden {
			testResult.Input = ""
			testResult.ExpectedOutput = ""
		}

		testResults = append(testResults, testResult)
	}

	s.logger.Printf("Total request processing time: %v", time.Since(overallStart))
	s.logger.Printf("-------------------------------------------------")

	return &models.ExecutionResults{
		Success:     success,
		TestResults: testResults,
	}, nil
}

func (s *service) ExecuteForProblem(ctx context.Context, code string, language string, problemID int) (*models.ExecutionResults, error) {
	s.logger.Printf("Executing code for problem %d", problemID)

	testCases, err := s.repository.GetTestCasesByProblemID(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get test cases for problem %d: %w", problemID, err)
	}

	if len(testCases) == 0 {
		return nil, fmt.Errorf("no test cases found for problem %d", problemID)
	}

	return s.ExecuteWithTestCases(ctx, code, language, testCases)
}
