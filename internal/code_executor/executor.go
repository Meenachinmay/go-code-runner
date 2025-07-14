package code_executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go-code-runner/internal/models"
	testcaserepo "go-code-runner/internal/repository/test_cases"
)

// Configuration for the executor service
type Config struct {
	WorkerCount      int
	MaxQueueSize     int
	ExecutionTimeout time.Duration
	ResultTTL        time.Duration
}

// Execution result types
type ExecutionResult struct {
	Output string
	Error  string
}

type ExecutionJob struct {
	ID         string
	Code       string
	Language   string
	ProblemID  int
	TestCases  []*models.TestCase
	SubmitTime time.Time
}

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

type JobResult struct {
	JobID            string
	Status           JobStatus
	ExecutionResult  *ExecutionResult
	ExecutionResults *models.ExecutionResults
	Error            string
	CompletedAt      time.Time
}

// Service implementation
type service struct {
	config      Config
	logger      *log.Logger
	imageCache  map[string]bool
	repository  testcaserepo.TestCaseRepository
	redisClient *redis.Client

	buildCacheDir string
	modCacheDir   string
	hostTempDir   string

	// Worker pool management
	jobQueue   chan *ExecutionJob
	workerWg   sync.WaitGroup
	shutdownCh chan struct{}

	// Metrics
	activeExecutions int64
}

func NewService(cfg Config, logger *log.Logger, repo testcaserepo.TestCaseRepository, redisClient *redis.Client) Service {
	logger.Printf("Initializing code executor service with config: WorkerCount=%d, MaxQueueSize=%d, ExecutionTimeout=%v, ResultTTL=%v", 
		cfg.WorkerCount, cfg.MaxQueueSize, cfg.ExecutionTimeout, cfg.ResultTTL)

	buildCacheDir := "/tmp/runbox/go-build-cache"
	modCacheDir := "/tmp/runbox/go-mod-cache"

	logger.Printf("Creating cache directories: buildCache=%s, modCache=%s", buildCacheDir, modCacheDir)
	os.MkdirAll(buildCacheDir, 0755)
	os.MkdirAll(modCacheDir, 0755)

	hostTempDir := os.Getenv("HOST_TEMP_DIR")
	if hostTempDir == "" {
		logger.Fatal("HOST_TEMP_DIR environment variable must be set")
	}
	logger.Printf("Using host temp directory: %s", hostTempDir)

	s := &service{
		config:           cfg,
		logger:           logger,
		imageCache:       make(map[string]bool),
		repository:       repo,
		redisClient:      redisClient,
		buildCacheDir:    buildCacheDir,
		modCacheDir:      modCacheDir,
		hostTempDir:      hostTempDir,
		jobQueue:         make(chan *ExecutionJob, cfg.MaxQueueSize),
		shutdownCh:       make(chan struct{}),
		activeExecutions: 0,
	}

	logger.Printf("Redis connection established successfully")

	// Pre-pull Docker image
	s.ensureDockerImageAvailable("golang:1.22-alpine")

	// Start workers
	logger.Printf("Starting %d worker goroutines", cfg.WorkerCount)
	s.startWorkers()

	// Start Redis queue processor
	logger.Printf("Starting Redis queue processor goroutine")
	go s.processRedisQueue()

	// Start metrics reporter
	logger.Printf("Starting metrics reporter goroutine")
	go s.reportMetrics()

	logger.Printf("Code executor service initialized successfully")
	return s
}

// Ensure Docker image is available (YOUR ORIGINAL METHOD)
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

// Core execution method (YOUR ORIGINAL METHOD - UNCHANGED)
func (s *service) executeCode(ctx context.Context, code string, language string, input string) (*ExecutionResult, error) {
	// Track active executions for metrics
	atomic.AddInt64(&s.activeExecutions, 1)
	defer atomic.AddInt64(&s.activeExecutions, -1)

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

	execCtx, cancel := context.WithTimeout(ctx, s.config.ExecutionTimeout)
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
		return nil, fmt.Errorf("execution timed out after %v", s.config.ExecutionTimeout)
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

// Internal method for executing with test cases (extracted from your original ExecuteWithTestCases)
func (s *service) executeWithTestCasesInternal(ctx context.Context, code string, language string, testCases []*models.TestCase) (*models.ExecutionResults, error) {
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

	return &models.ExecutionResults{
		Success:     success,
		TestResults: testResults,
	}, nil
}

// ============= PUBLIC INTERFACE METHODS (YOUR ORIGINAL SIGNATURES) =============

func (s *service) Execute(ctx context.Context, code string, language string) (*ExecutionResult, error) {
	overallStart := time.Now()
	s.logger.Printf("-------------------------------------------------")
	s.logger.Println("Received new execution request.")

	// Submit job to queue
	job := &ExecutionJob{
		Code:     code,
		Language: language,
	}

	jobID, err := s.SubmitJob(ctx, job)
	if err != nil {
		return nil, err
	}

	// Wait for result (synchronous for backward compatibility)
	result, err := s.waitForJobResult(ctx, jobID)
	if err != nil {
		return nil, err
	}

	s.logger.Printf("Total request processing time: %v", time.Since(overallStart))
	s.logger.Printf("-------------------------------------------------")

	return result.ExecutionResult, nil
}

func (s *service) ExecuteWithTestCases(ctx context.Context, code string, language string, testCases []*models.TestCase) (*models.ExecutionResults, error) {
	overallStart := time.Now()
	s.logger.Printf("-------------------------------------------------")
	s.logger.Println("Received execution request with test cases.")

	// Submit job to queue
	job := &ExecutionJob{
		Code:      code,
		Language:  language,
		TestCases: testCases,
	}

	jobID, err := s.SubmitJob(ctx, job)
	if err != nil {
		return nil, err
	}

	// Wait for result
	result, err := s.waitForJobResult(ctx, jobID)
	if err != nil {
		return nil, err
	}

	s.logger.Printf("Total request processing time: %v", time.Since(overallStart))
	s.logger.Printf("-------------------------------------------------")

	return result.ExecutionResults, nil
}

func (s *service) ExecuteForProblem(ctx context.Context, code string, language string, problemID int) (*models.ExecutionResults, error) {
	s.logger.Printf("Executing code for problem %d", problemID)

	// Submit job to queue
	job := &ExecutionJob{
		Code:      code,
		Language:  language,
		ProblemID: problemID,
	}

	jobID, err := s.SubmitJob(ctx, job)
	if err != nil {
		return nil, err
	}

	// Wait for result
	result, err := s.waitForJobResult(ctx, jobID)
	if err != nil {
		return nil, err
	}

	if result.Error != "" {
		return nil, fmt.Errorf(result.Error)
	}

	return result.ExecutionResults, nil
}

// ============= NEW ASYNC METHODS FOR SCALABILITY =============

func (s *service) SubmitJob(ctx context.Context, job *ExecutionJob) (string, error) {
	startTime := time.Now()
	job.ID = uuid.New().String()
	job.SubmitTime = startTime

	s.logger.Printf("[JobID: %s] Submitting new job: Language=%s, ProblemID=%d, TestCases=%d", 
		job.ID, job.Language, job.ProblemID, len(job.TestCases))

	// Serialize job
	jobData, err := json.Marshal(job)
	if err != nil {
		s.logger.Printf("[JobID: %s] ERROR: Failed to marshal job: %v", job.ID, err)
		return "", fmt.Errorf("failed to marshal job: %w", err)
	}
	s.logger.Printf("[JobID: %s] Job serialized successfully, size=%d bytes", job.ID, len(jobData))

	// Push to Redis queue
	s.logger.Printf("[JobID: %s] Pushing job to Redis execution_queue", job.ID)
	if err := s.redisClient.RPush(ctx, "execution_queue", jobData).Err(); err != nil {
		s.logger.Printf("[JobID: %s] ERROR: Failed to push job to Redis queue: %v", job.ID, err)
		return "", fmt.Errorf("failed to queue job: %w", err)
	}
	s.logger.Printf("[JobID: %s] Job pushed to Redis queue successfully", job.ID)

	// Set initial status
	s.logger.Printf("[JobID: %s] Setting initial job status to %s", job.ID, JobStatusPending)
	s.updateJobStatus(ctx, job.ID, JobStatusPending)

	s.logger.Printf("[JobID: %s] Job submitted successfully (took %v)", job.ID, time.Since(startTime))
	return job.ID, nil
}

func (s *service) GetJobResult(ctx context.Context, jobID string) (*JobResult, error) {
	startTime := time.Now()
	s.logger.Printf("[JobID: %s] Retrieving job result from Redis", jobID)

	resultKey := fmt.Sprintf("job_result:%s", jobID)
	s.logger.Printf("[JobID: %s] Checking Redis key: %s", jobID, resultKey)
	data, err := s.redisClient.Get(ctx, resultKey).Result()

	if err == redis.Nil {
		// Check if job is still pending or running
		statusKey := fmt.Sprintf("job_status:%s", jobID)
		s.logger.Printf("[JobID: %s] Job result not found, checking status key: %s", jobID, statusKey)
		status, statusErr := s.redisClient.Get(ctx, statusKey).Result()

		if statusErr != nil && statusErr != redis.Nil {
			s.logger.Printf("[JobID: %s] ERROR: Failed to get job status: %v", jobID, statusErr)
		}

		if status != "" {
			s.logger.Printf("[JobID: %s] Job is still in progress with status: %s", jobID, status)
			return &JobResult{
				JobID:  jobID,
				Status: JobStatus(status),
			}, nil
		}

		s.logger.Printf("[JobID: %s] ERROR: Job not found in Redis", jobID)
		return nil, fmt.Errorf("job not found")
	} else if err != nil {
		s.logger.Printf("[JobID: %s] ERROR: Failed to get job result from Redis: %v", jobID, err)
		return nil, err
	}

	s.logger.Printf("[JobID: %s] Job result found in Redis, size=%d bytes", jobID, len(data))

	var result JobResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		s.logger.Printf("[JobID: %s] ERROR: Failed to unmarshal job result: %v", jobID, err)
		return nil, err
	}

	s.logger.Printf("[JobID: %s] Job result retrieved successfully with status=%s (took %v)", 
		jobID, result.Status, time.Since(startTime))
	return &result, nil
}

// ============= WORKER POOL IMPLEMENTATION =============

func (s *service) startWorkers() {
	s.logger.Printf("Starting %d workers in the worker pool", s.config.WorkerCount)
	startTime := time.Now()

	for i := 0; i < s.config.WorkerCount; i++ {
		s.workerWg.Add(1)
		s.logger.Printf("Launching worker goroutine %d", i)
		go s.worker(i)
	}

	s.logger.Printf("All %d workers started successfully (took %v)", s.config.WorkerCount, time.Since(startTime))
}

func (s *service) worker(id int) {
	defer s.workerWg.Done()
	startTime := time.Now()

	s.logger.Printf("[Worker-%d] Worker started and waiting for jobs", id)
	jobsProcessed := 0

	for {
		select {
		case job := <-s.jobQueue:
			jobsProcessed++
			s.logger.Printf("[Worker-%d] Received job %s (total jobs processed: %d)", id, job.ID, jobsProcessed)
			jobStartTime := time.Now()
			s.processJob(job)
			s.logger.Printf("[Worker-%d] Completed job %s (took %v, total jobs processed: %d)", 
				id, job.ID, time.Since(jobStartTime), jobsProcessed)

		case <-s.shutdownCh:
			s.logger.Printf("[Worker-%d] Shutting down after processing %d jobs (active for %v)", 
				id, jobsProcessed, time.Since(startTime))
			return
		}
	}
}

func (s *service) processJob(job *ExecutionJob) {
	ctx := context.Background()
	startTime := time.Now()
	s.logger.Printf("[JobID: %s] Starting job processing", job.ID)

	waitTime := startTime.Sub(job.SubmitTime)
	s.logger.Printf("[JobID: %s] Job waited in queue for %v before processing", job.ID, waitTime)

	// Update job status to running
	s.logger.Printf("[JobID: %s] Updating job status to %s", job.ID, JobStatusRunning)
	s.updateJobStatus(ctx, job.ID, JobStatusRunning)

	var result *JobResult

	// Route to appropriate execution method based on job type
	if job.ProblemID > 0 {
		s.logger.Printf("[JobID: %s] Processing job for problem ID %d", job.ID, job.ProblemID)

		// Get test cases for problem
		s.logger.Printf("[JobID: %s] Fetching test cases for problem ID %d", job.ID, job.ProblemID)
		testCases, err := s.repository.GetTestCasesByProblemID(ctx, job.ProblemID)

		if err != nil {
			s.logger.Printf("[JobID: %s] ERROR: Failed to get test cases for problem %d: %v", job.ID, job.ProblemID, err)
			result = &JobResult{
				JobID:       job.ID,
				Status:      JobStatusFailed,
				Error:       fmt.Sprintf("failed to get test cases for problem %d: %v", job.ProblemID, err),
				CompletedAt: time.Now(),
			}
		} else if len(testCases) == 0 {
			s.logger.Printf("[JobID: %s] ERROR: No test cases found for problem %d", job.ID, job.ProblemID)
			result = &JobResult{
				JobID:       job.ID,
				Status:      JobStatusFailed,
				Error:       fmt.Sprintf("no test cases found for problem %d", job.ProblemID),
				CompletedAt: time.Now(),
			}
		} else {
			s.logger.Printf("[JobID: %s] Found %d test cases for problem %d, executing code", job.ID, len(testCases), job.ProblemID)
			execResults, err := s.executeWithTestCasesInternal(ctx, job.Code, job.Language, testCases)

			completedAt := time.Now()
			if err != nil {
				s.logger.Printf("[JobID: %s] ERROR: Code execution failed: %v", job.ID, err)
				result = &JobResult{
					JobID:            job.ID,
					Status:           JobStatusFailed,
					ExecutionResults: execResults,
					Error:            err.Error(),
					CompletedAt:      completedAt,
				}
			} else {
				s.logger.Printf("[JobID: %s] Code execution completed successfully with %d test results", 
					job.ID, len(execResults.TestResults))
				result = &JobResult{
					JobID:            job.ID,
					Status:           JobStatusCompleted,
					ExecutionResults: execResults,
					CompletedAt:      completedAt,
				}
			}
		}
	} else if len(job.TestCases) > 0 {
		// Execute with provided test cases
		s.logger.Printf("[JobID: %s] Processing job with %d provided test cases", job.ID, len(job.TestCases))
		execResults, err := s.executeWithTestCasesInternal(ctx, job.Code, job.Language, job.TestCases)

		completedAt := time.Now()
		if err != nil {
			s.logger.Printf("[JobID: %s] ERROR: Code execution with test cases failed: %v", job.ID, err)
			result = &JobResult{
				JobID:            job.ID,
				Status:           JobStatusFailed,
				ExecutionResults: execResults,
				Error:            err.Error(),
				CompletedAt:      completedAt,
			}
		} else {
			s.logger.Printf("[JobID: %s] Code execution with test cases completed successfully with %d test results", 
				job.ID, len(execResults.TestResults))
			result = &JobResult{
				JobID:            job.ID,
				Status:           JobStatusCompleted,
				ExecutionResults: execResults,
				CompletedAt:      completedAt,
			}
		}
	} else {
		// Simple execution without test cases
		s.logger.Printf("[JobID: %s] Processing simple execution job without test cases", job.ID)
		execResult, err := s.executeCode(ctx, job.Code, job.Language, "")

		completedAt := time.Now()
		if err != nil {
			s.logger.Printf("[JobID: %s] ERROR: Simple code execution failed: %v", job.ID, err)
			result = &JobResult{
				JobID:           job.ID,
				Status:          JobStatusFailed,
				ExecutionResult: execResult,
				Error:           err.Error(),
				CompletedAt:     completedAt,
			}
		} else {
			s.logger.Printf("[JobID: %s] Simple code execution completed successfully", job.ID)
			result = &JobResult{
				JobID:           job.ID,
				Status:          JobStatusCompleted,
				ExecutionResult: execResult,
				CompletedAt:     completedAt,
			}
		}
	}

	// Store result in Redis
	s.logger.Printf("[JobID: %s] Storing job result in Redis with status %s", job.ID, result.Status)
	s.storeJobResult(ctx, result)

	processingTime := time.Since(startTime)
	totalTime := result.CompletedAt.Sub(job.SubmitTime)
	s.logger.Printf("[JobID: %s] Job processing completed: status=%s, processing_time=%v, total_time=%v", 
		job.ID, result.Status, processingTime, totalTime)
}

// ============= REDIS QUEUE PROCESSING =============

func (s *service) processRedisQueue() {
	s.logger.Printf("Redis queue processor started, monitoring 'execution_queue'")
	jobsProcessed := 0
	startTime := time.Now()

	for {
		select {
		case <-s.shutdownCh:
			s.logger.Printf("Redis queue processor shutting down after processing %d jobs (active for %v)", 
				jobsProcessed, time.Since(startTime))
			return
		default:
			// Pop job from Redis queue (blocking pop with 1 second timeout)
			s.logger.Printf("Waiting for jobs in Redis 'execution_queue' (BLPOP with 1s timeout)")
			result, err := s.redisClient.BLPop(context.Background(), time.Second, "execution_queue").Result()

			if err == redis.Nil {
				// This is normal for BLPOP timeout, don't log to avoid noise
				continue // Timeout, try again
			} else if err != nil {
				s.logger.Printf("ERROR: Failed to pop job from Redis queue: %v", err)
				s.logger.Printf("Backing off for 1 second before retrying")
				time.Sleep(time.Second)
				continue
			}

			if len(result) < 2 {
				s.logger.Printf("WARNING: Unexpected result from Redis BLPOP, expected at least 2 items, got %d", len(result))
				continue
			}

			s.logger.Printf("Job popped from Redis queue, data size: %d bytes", len(result[1]))
			jobsProcessed++

			var job ExecutionJob
			if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
				s.logger.Printf("ERROR: Failed to unmarshal job data: %v", err)
				s.logger.Printf("Job data: %s", result[1])
				continue
			}

			s.logger.Printf("[JobID: %s] Job unmarshaled successfully from Redis queue", job.ID)

			// Send to worker pool
			s.logger.Printf("[JobID: %s] Attempting to send job to internal worker pool queue", job.ID)
			select {
			case s.jobQueue <- &job:
				s.logger.Printf("[JobID: %s] Job successfully queued to worker pool", job.ID)
			default:
				s.logger.Printf("[JobID: %s] WARNING: Internal worker pool queue is full, pushing job back to Redis", job.ID)
				pushErr := s.redisClient.LPush(context.Background(), "execution_queue", result[1]).Err()
				if pushErr != nil {
					s.logger.Printf("[JobID: %s] ERROR: Failed to push job back to Redis queue: %v", job.ID, pushErr)
				} else {
					s.logger.Printf("[JobID: %s] Job pushed back to Redis queue successfully", job.ID)
				}
				time.Sleep(100 * time.Millisecond)
			}

			// Log queue stats periodically
			if jobsProcessed%100 == 0 {
				s.logger.Printf("Redis queue processor stats: processed %d jobs in %v", 
					jobsProcessed, time.Since(startTime))
			}
		}
	}
}

// ============= HELPER METHODS =============

func (s *service) waitForJobResult(ctx context.Context, jobID string) (*JobResult, error) {
	startTime := time.Now()
	s.logger.Printf("[JobID: %s] Waiting for job result with timeout %v", jobID, s.config.ExecutionTimeout+5*time.Second)

	timeout := time.After(s.config.ExecutionTimeout + 5*time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	pollCount := 0
	lastLoggedStatus := ""

	for {
		select {
		case <-ctx.Done():
			s.logger.Printf("[JobID: %s] Context cancelled while waiting for job result: %v", jobID, ctx.Err())
			return nil, ctx.Err()

		case <-timeout:
			s.logger.Printf("[JobID: %s] ERROR: Execution timed out after %v while waiting for job result", 
				jobID, s.config.ExecutionTimeout+5*time.Second)
			return nil, fmt.Errorf("execution timeout waiting for job %s", jobID)

		case <-ticker.C:
			pollCount++
			result, err := s.GetJobResult(ctx, jobID)

			if err != nil {
				// Only log every 10 polls to reduce noise
				if pollCount%10 == 0 {
					s.logger.Printf("[JobID: %s] Still waiting for job result (polled %d times over %v)", 
						jobID, pollCount, time.Since(startTime))
				}
				continue // Job not ready yet
			}

			// Log status changes
			if lastLoggedStatus != string(result.Status) {
				s.logger.Printf("[JobID: %s] Job status changed to %s", jobID, result.Status)
				lastLoggedStatus = string(result.Status)
			}

			switch result.Status {
			case JobStatusCompleted:
				s.logger.Printf("[JobID: %s] Job completed successfully after %v (%d polls)", 
					jobID, time.Since(startTime), pollCount)
				return result, nil

			case JobStatusFailed:
				s.logger.Printf("[JobID: %s] Job failed after %v (%d polls): %s", 
					jobID, time.Since(startTime), pollCount, result.Error)
				return result, fmt.Errorf("job failed: %s", result.Error)

			case JobStatusPending, JobStatusRunning:
				// Only log every 10 polls to reduce noise
				if pollCount%10 == 0 {
					s.logger.Printf("[JobID: %s] Job still in progress with status %s (waited %v so far)", 
						jobID, result.Status, time.Since(startTime))
				}
				continue // Still processing
			}
		}
	}
}

func (s *service) updateJobStatus(ctx context.Context, jobID string, status JobStatus) {
	key := fmt.Sprintf("job_status:%s", jobID)
	s.logger.Printf("[JobID: %s] Updating job status to %s in Redis (key: %s)", jobID, status, key)

	err := s.redisClient.Set(ctx, key, string(status), s.config.ResultTTL).Err()
	if err != nil {
		s.logger.Printf("[JobID: %s] ERROR: Failed to update job status in Redis: %v", jobID, err)
	} else {
		s.logger.Printf("[JobID: %s] Job status updated successfully in Redis with TTL %v", jobID, s.config.ResultTTL)
	}
}

func (s *service) storeJobResult(ctx context.Context, result *JobResult) {
	startTime := time.Now()
	s.logger.Printf("[JobID: %s] Storing job result in Redis with status %s", result.JobID, result.Status)

	data, err := json.Marshal(result)
	if err != nil {
		s.logger.Printf("[JobID: %s] ERROR: Failed to marshal job result: %v", result.JobID, err)
		return
	}

	key := fmt.Sprintf("job_result:%s", result.JobID)
	s.logger.Printf("[JobID: %s] Setting job result in Redis (key: %s, size: %d bytes)", 
		result.JobID, key, len(data))

	err = s.redisClient.Set(ctx, key, data, s.config.ResultTTL).Err()
	if err != nil {
		s.logger.Printf("[JobID: %s] ERROR: Failed to store job result in Redis: %v", result.JobID, err)
		return
	}

	// Remove status key
	statusKey := fmt.Sprintf("job_status:%s", result.JobID)
	s.logger.Printf("[JobID: %s] Removing job status key from Redis (key: %s)", result.JobID, statusKey)

	err = s.redisClient.Del(ctx, statusKey).Err()
	if err != nil {
		s.logger.Printf("[JobID: %s] ERROR: Failed to remove job status key from Redis: %v", result.JobID, err)
	}

	s.logger.Printf("[JobID: %s] Job result stored successfully in Redis (took %v)", 
		result.JobID, time.Since(startTime))
}

// ============= METRICS AND MONITORING =============

func (s *service) reportMetrics() {
	s.logger.Printf("Metrics reporter started, reporting interval: 10 seconds")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	reportCount := 0

	for {
		select {
		case <-ticker.C:
			reportCount++
			active := atomic.LoadInt64(&s.activeExecutions)
			queued := len(s.jobQueue)

			// Get Redis queue length
			queueLen, err := s.redisClient.LLen(context.Background(), "execution_queue").Result()
			if err != nil {
				s.logger.Printf("ERROR: Failed to get Redis queue length: %v", err)
				queueLen = -1 // Indicate error
			}

			s.logger.Printf("METRICS [%d]: Active executions: %d, Internal queue: %d jobs, Redis queue: %d jobs, Uptime: %v", 
				reportCount, active, queued, queueLen, time.Since(startTime))

			// Log worker pool utilization
			if s.config.WorkerCount > 0 {
				utilization := float64(active) / float64(s.config.WorkerCount) * 100
				s.logger.Printf("METRICS [%d]: Worker pool utilization: %.2f%% (%d/%d workers active)", 
					reportCount, utilization, active, s.config.WorkerCount)
			}

		case <-s.shutdownCh:
			s.logger.Printf("Metrics reporter shutting down after %d reports (active for %v)", 
				reportCount, time.Since(startTime))
			return
		}
	}
}

// ============= SHUTDOWN =============

func (s *service) Shutdown() {
	s.logger.Println("Shutting down executor service...")
	shutdownStart := time.Now()

	s.logger.Println("Signaling all goroutines to stop")
	close(s.shutdownCh)

	s.logger.Println("Waiting for all workers to complete their current jobs")
	waitStart := time.Now()
	s.workerWg.Wait()

	s.logger.Printf("All workers stopped gracefully (took %v)", time.Since(waitStart))
	s.logger.Printf("Executor service shutdown complete (took %v)", time.Since(shutdownStart))
}
