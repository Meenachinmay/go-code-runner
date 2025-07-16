package code_executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go-code-runner/internal/models"
	testcaserepo "go-code-runner/internal/repository/test_cases"
)

const (
	LogLevelError = iota
	LogLevelInfo
	LogLevelDebug
)

type Config struct {
	WorkerCount         int
	MaxQueueSize        int
	ExecutionTimeout    time.Duration
	ResultTTL           time.Duration
	EnableContainerPool bool
	ContainerPoolSize   int
}

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

type Container struct {
	ID        string
	Available bool
	LastUsed  time.Time
}

type ContainerPool struct {
	mu           sync.Mutex
	containers   []*Container
	available    chan string
	maxSize      int
	logger       *log.Logger
	failureCount int
	lastFailure  time.Time
	circuitOpen  bool
}

type service struct {
	config      Config
	logger      *log.Logger
	logLevel    int
	imageCache  map[string]bool
	repository  testcaserepo.TestCaseRepository
	redisClient *redis.Client

	buildCacheDir string
	modCacheDir   string
	hostTempDir   string

	jobQueue      chan *ExecutionJob
	workerWg      sync.WaitGroup
	shutdownCh    chan struct{}
	containerPool *ContainerPool

	activeExecutions int64
}

func (s *service) logDebug(format string, v ...interface{}) {
	if s.logLevel >= LogLevelDebug {
		s.logger.Printf(format, v...)
	}
}

func (s *service) logInfo(format string, v ...interface{}) {
	if s.logLevel >= LogLevelInfo {
		s.logger.Printf(format, v...)
	}
}

func (s *service) logError(format string, v ...interface{}) {
	s.logger.Printf("ERROR: "+format, v...)
}

func NewService(cfg Config, logger *log.Logger, repo testcaserepo.TestCaseRepository, redisClient *redis.Client) Service {
	logger.Printf("Initializing code executor service with config: WorkerCount=%d, MaxQueueSize=%d, ExecutionTimeout=%v, ResultTTL=%v",
		cfg.WorkerCount, cfg.MaxQueueSize, cfg.ExecutionTimeout, cfg.ResultTTL)

	buildCacheDir := "/tmp/runbox/go-build-cache"
	modCacheDir := "/tmp/runbox/go-mod-cache"

	os.MkdirAll(buildCacheDir, 0755)
	os.MkdirAll(modCacheDir, 0755)

	hostTempDir := os.Getenv("HOST_TEMP_DIR")
	if hostTempDir == "" {
		logger.Fatal("HOST_TEMP_DIR environment variable must be set")
	}

	logLevel := LogLevelInfo
	if os.Getenv("DEBUG") == "true" {
		logLevel = LogLevelDebug
	} else if os.Getenv("ENV") == "production" {
		logLevel = LogLevelError
	}

	s := &service{
		config:           cfg,
		logger:           logger,
		logLevel:         logLevel,
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

	s.ensureDockerImageAvailable("golang:1.22-alpine")

	if err := s.initializeContainerPool(cfg.ContainerPoolSize); err != nil {
		logger.Printf("WARNING: Failed to initialize container pool: %v", err)
	}

	s.startWorkers()

	numProcessors := 20
	if cfg.WorkerCount > 100 {
		numProcessors = cfg.WorkerCount / 5
	}
	logger.Printf("Starting %d Redis queue processor goroutines", numProcessors)
	for i := 0; i < numProcessors; i++ {
		go s.processRedisQueue()
	}

	go s.reportMetrics()

	logger.Printf("Code executor service initialized successfully")
	return s
}

func NewContainerPool(size int, logger *log.Logger) *ContainerPool {
	pool := &ContainerPool{
		containers:   make([]*Container, 0, size),
		available:    make(chan string, size),
		maxSize:      size,
		logger:       logger,
		failureCount: 0,
		lastFailure:  time.Time{},
		circuitOpen:  false,
	}
	return pool
}

// RecordFailure increments the failure count and opens the circuit if threshold is reached
func (p *ContainerPool) RecordFailure() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.failureCount++
	p.lastFailure = time.Now()

	if p.failureCount >= 5 {
		p.circuitOpen = true
		p.logger.Printf("Circuit breaker opened due to %d consecutive failures", p.failureCount)
	}
}

// RecordSuccess resets the failure count and closes the circuit
func (p *ContainerPool) RecordSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.failureCount > 0 {
		p.failureCount = 0
		p.circuitOpen = false
		p.logger.Printf("Circuit breaker reset after successful operation")
	}
}

// IsCircuitOpen checks if the circuit is open, with auto-reset after cooldown
func (p *ContainerPool) IsCircuitOpen() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.circuitOpen && time.Since(p.lastFailure) > 30*time.Second {
		p.circuitOpen = false
		p.failureCount = 0
		p.logger.Printf("Circuit breaker auto-reset after cooldown period")
		return false
	}

	return p.circuitOpen
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

func (s *service) isContainerHealthy(containerID string) bool {
	s.logDebug("Checking health of container %s", containerID[:12])

	checkCmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerID)
	output, err := checkCmd.Output()

	if err != nil || strings.TrimSpace(string(output)) != "true" {
		s.logDebug("Container %s is not running", containerID[:12])
		return false
	}

	pingCmd := exec.Command("docker", "exec", containerID, "echo", "ping")
	if err := pingCmd.Run(); err != nil {
		s.logDebug("Container %s is not responsive: %v", containerID[:12], err)
		return false
	}

	s.logDebug("Container %s is healthy", containerID[:12])
	return true
}

func (s *service) executeCode(ctx context.Context, code string, language string, input string) (*ExecutionResult, error) {
	atomic.AddInt64(&s.activeExecutions, 1)
	defer atomic.AddInt64(&s.activeExecutions, -1)

	runID := uuid.New().String()
	s.logDebug("[%s] Starting code execution...", runID)

	if s.containerPool != nil && len(s.containerPool.containers) > 10 && !s.containerPool.IsCircuitOpen() {
		s.logDebug("[%s] Container pool is available and circuit is closed", runID)

		for attempts := 0; attempts < 2; attempts++ {
			containerID, err := s.getContainer(ctx)
			if err == nil {
				s.logDebug("[%s] Using pooled container: %s (attempt %d)", runID, containerID[:12], attempts+1)

				if !s.isContainerHealthy(containerID) {
					s.logDebug("[%s] Container %s is unhealthy, skipping", runID, containerID[:12])

					s.containerPool.RecordFailure()
					s.returnContainer(containerID)
					continue
				}

				result, err := s.executeInPooledContainer(ctx, containerID, code, language, input, runID)

				s.returnContainer(containerID)

				if err == nil {

					s.containerPool.RecordSuccess()
					return result, nil
				}

				s.containerPool.RecordFailure()
				s.logDebug("[%s] Execution failed in pooled container: %v", runID, err)
				continue
			}
			s.logDebug("[%s] Failed to get pooled container (attempt %d): %v", runID, attempts+1, err)
		}

		s.logInfo("[%s] Falling back to traditional execution after pool failures", runID)
	} else if s.containerPool != nil && s.containerPool.IsCircuitOpen() {
		s.logInfo("[%s] Circuit breaker is open, bypassing container pool", runID)
	}

	containerID, err := s.createOnDemandContainer()
	if err != nil {
		return nil, fmt.Errorf("failed to create on-demand container: %w", err)
	}

	s.logDebug("[%s] Using on-demand container: %s", runID, containerID[:12])

	result, err := s.executeInPooledContainer(ctx, containerID, code, language, input, runID)

	go func() {
		exec.Command("docker", "rm", "-f", containerID).Run()
	}()

	if err != nil {
		return nil, fmt.Errorf("execution failed in on-demand container: %w", err)
	}

	return result, nil
}

func (s *service) executeInPooledContainer(ctx context.Context, containerID string, code string, language string, input string, runID string) (*ExecutionResult, error) {
	s.logDebug("[%s] Executing in pooled container %s", runID, containerID[:12])

	execCtx, cancel := context.WithTimeout(ctx, s.config.ExecutionTimeout)
	defer cancel()

	execDir := fmt.Sprintf("/tmp/exec_%s", runID)
	mkdirCmd := exec.CommandContext(execCtx, "docker", "exec", containerID, "mkdir", "-p", execDir)
	if err := mkdirCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create exec dir: %w", err)
	}

	codeFile := fmt.Sprintf("%s/main.go", execDir)
	writeCodeCmd := exec.CommandContext(execCtx, "docker", "exec", "-i", containerID, "tee", codeFile)
	writeCodeCmd.Stdin = strings.NewReader(code)

	if err := writeCodeCmd.Run(); err != nil {
		exec.Command("docker", "exec", containerID, "rm", "-rf", execDir).Run()
		return nil, fmt.Errorf("failed to write code: %w", err)
	}

	execCmd := fmt.Sprintf(`cd %s && export GOCACHE=/root/.cache/go-build GOMODCACHE=/go/pkg/mod GOFLAGS=-mod=readonly`, execDir)

	if input != "" {

		inputFile := fmt.Sprintf("%s/input.txt", execDir)
		writeInputCmd := exec.CommandContext(execCtx, "docker", "exec", "-i", containerID, "tee", inputFile)
		writeInputCmd.Stdin = strings.NewReader(input)
		if err := writeInputCmd.Run(); err != nil {
			exec.Command("docker", "exec", containerID, "rm", "-rf", execDir).Run()
			return nil, fmt.Errorf("failed to write input: %w", err)
		}
		execCmd += " && cat input.txt | go run main.go"
	} else {
		execCmd += " && go run main.go"
	}

	dockerExec := exec.CommandContext(execCtx, "docker", "exec", containerID, "sh", "-c", execCmd)

	var stdout, stderr bytes.Buffer
	dockerExec.Stdout = &stdout
	dockerExec.Stderr = &stderr

	err := dockerExec.Run()

	exec.Command("docker", "exec", containerID, "rm", "-rf", execDir).Run()

	if execCtx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("execution timed out after %v", s.config.ExecutionTimeout)
	}

	result := &ExecutionResult{
		Output: stdout.String(),
		Error:  stderr.String(),
	}

	if err != nil && result.Error == "" {
		result.Error = err.Error()
	}

	return result, nil
}

func (s *service) createWarmContainer(index int) (string, error) {
	containerName := fmt.Sprintf("executor-pool-%d-%s", index, uuid.New().String()[:8])

	hostBuildCacheDir := strings.Replace(s.buildCacheDir, "/tmp/runbox", s.hostTempDir, 1)
	hostModCacheDir := strings.Replace(s.modCacheDir, "/tmp/runbox", s.hostTempDir, 1)

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--network", "none",
		"--memory", "256m",
		"--cpus", "0.5",
		"-v", fmt.Sprintf("%s:/root/.cache/go-build:rw", hostBuildCacheDir),
		"-v", fmt.Sprintf("%s:/go/pkg/mod:rw", hostModCacheDir),
		"golang:1.22-alpine",
		"sh", "-c", "while true; do sleep 3600; done",
	}

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create warm container: %w, output: %s", err, output)
	}

	containerID := strings.TrimSpace(string(output))

	time.Sleep(100 * time.Millisecond)

	checkCmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerID)
	checkOutput, err := checkCmd.Output()
	if err != nil || strings.TrimSpace(string(checkOutput)) != "true" {

		exec.Command("docker", "rm", "-f", containerID).Run()
		return "", fmt.Errorf("container failed to start properly")
	}

	s.logDebug("Created and verified warm container %s", containerID[:12])

	go s.prewarmContainer(containerID)

	return containerID, nil
}

func (s *service) prewarmContainer(containerID string) {

	prewarmCode := `package main
	import (
		_ "fmt"
		_ "strings"
		_ "sort"
		_ "io"
		_ "os"
		_ "time"
	)
	func main() {}`

	cmd := exec.Command("docker", "exec", "-i", containerID, "sh", "-c",
		fmt.Sprintf("echo '%s' | go build -o /dev/null -", prewarmCode))

	if err := cmd.Run(); err != nil {
		s.logDebug("Failed to prewarm container %s: %v", containerID[:12], err)
	} else {
		s.logDebug("Successfully prewarmed container %s", containerID[:12])
	}
}

func (s *service) getContainer(ctx context.Context) (string, error) {

	if s.containerPool.IsCircuitOpen() {
		s.logDebug("Circuit breaker is open, bypassing container pool")
		return s.createOnDemandContainer()
	}

	select {
	case containerID := <-s.containerPool.available:
		checkCmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerID)
		output, err := checkCmd.Output()

		isRunning := err == nil && strings.TrimSpace(string(output)) == "true"

		if !isRunning {
			s.logDebug("Container %s is not running, creating replacement", containerID[:12])

			s.containerPool.RecordFailure()

			go func() {
				exec.Command("docker", "rm", "-f", containerID).Run()
			}()

			newID, err := s.createWarmContainer(len(s.containerPool.containers))
			if err != nil {

				s.containerPool.RecordFailure()
				return "", fmt.Errorf("failed to create replacement container: %w", err)
			}

			return newID, nil
		}

		s.containerPool.RecordSuccess()
		return containerID, nil

	case <-ctx.Done():
		return "", ctx.Err()

	default:
		s.logDebug("No containers available in pool, creating on-demand container")
		return s.createOnDemandContainer()
	}
}

func (s *service) createOnDemandContainer() (string, error) {
	containerID, err := s.createWarmContainer(9999)
	if err != nil {
		return "", err
	}

	return containerID, nil
}

func (s *service) returnContainer(containerID string) {
	checkCmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerID)
	output, err := checkCmd.Output()

	if err != nil || strings.TrimSpace(string(output)) != "true" {
		s.logDebug("Not returning dead container %s to pool", containerID[:12])

		s.containerPool.RecordFailure()

		go func() {
			exec.Command("docker", "rm", "-f", containerID).Run()
		}()
		return
	}

	select {
	case s.containerPool.available <- containerID:

		s.containerPool.RecordSuccess()

	default:
		s.logDebug("Container pool full, removing container %s", containerID[:12])
		go func() {
			exec.Command("docker", "rm", "-f", containerID).Run()
		}()
	}
}

func (s *service) initializeContainerPool(size int) error {
	s.logger.Printf("Initializing container pool with %d containers", size)

	os.MkdirAll(s.buildCacheDir, 0755)
	os.MkdirAll(s.modCacheDir, 0755)

	s.containerPool = NewContainerPool(size, s.logger)

	concurrentCreations := 10
	if size < concurrentCreations {
		concurrentCreations = size
	}

	semaphore := make(chan struct{}, concurrentCreations)
	var wg sync.WaitGroup
	successCount := int32(0)

	for i := 0; i < size; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			containerID, err := s.createWarmContainer(index)
			if err != nil {
				s.logger.Printf("Failed to create container %d: %v", index, err)
				return
			}

			container := &Container{
				ID:        containerID,
				Available: true,
				LastUsed:  time.Now(),
			}

			s.containerPool.mu.Lock()
			s.containerPool.containers = append(s.containerPool.containers, container)
			s.containerPool.mu.Unlock()

			select {
			case s.containerPool.available <- containerID:
				atomic.AddInt32(&successCount, 1)
			default:

				s.logger.Printf("WARNING: Could not add container %s to available pool", containerID[:12])
			}
		}(i)
	}

	wg.Wait()

	finalCount := atomic.LoadInt32(&successCount)
	if finalCount == 0 {
		return fmt.Errorf("failed to create any containers for pool")
	}

	s.logger.Printf("Container pool initialized with %d/%d containers", finalCount, size)

	go s.maintainContainerPool()

	return nil
}

func (s *service) maintainContainerPool() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdownCh:
			return
		case <-ticker.C:
			if s.containerPool.IsCircuitOpen() {
				s.logInfo("Circuit breaker is open, monitoring for auto-reset")
				continue
			}

			s.containerPool.mu.Lock()
			totalContainers := len(s.containerPool.containers)
			s.containerPool.mu.Unlock()

			availableCount := len(s.containerPool.available)

			s.logDebug("Container pool health: %d/%d available (%.1f%%)",
				availableCount, totalContainers,
				float64(availableCount)/float64(totalContainers)*100)

			if availableCount < totalContainers/4 {
				s.logDebug("Container pool low (%d/%d), creating more containers",
					availableCount, totalContainers)

				healthyCount := 0
				for i := 0; i < 5; i++ {
					go func() {
						if containerID, err := s.createWarmContainer(int(time.Now().Unix())); err == nil {
							if s.isContainerHealthy(containerID) {
								healthyCount++
								s.containerPool.mu.Lock()
								s.containerPool.containers = append(s.containerPool.containers, &Container{
									ID:        containerID,
									Available: true,
									LastUsed:  time.Now(),
								})
								s.containerPool.mu.Unlock()

								select {
								case s.containerPool.available <- containerID:
									s.logDebug("Added new container to pool")
								default:
									exec.Command("docker", "rm", "-f", containerID).Run()
								}
							} else {
								s.logDebug("Newly created container is unhealthy, removing")
								s.containerPool.RecordFailure()
								exec.Command("docker", "rm", "-f", containerID).Run()
							}
						} else {
							s.logDebug("Failed to create new container: %v", err)
							s.containerPool.RecordFailure()
						}
					}()
				}
			}
		}
	}
}

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

func (s *service) Execute(ctx context.Context, code string, language string) (*ExecutionResult, error) {
	overallStart := time.Now()
	s.logger.Printf("-------------------------------------------------")
	s.logger.Println("Received new execution request.")

	job := &ExecutionJob{
		Code:     code,
		Language: language,
	}

	jobID, err := s.SubmitJob(ctx, job)
	if err != nil {
		return nil, err
	}

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

	job := &ExecutionJob{
		Code:      code,
		Language:  language,
		TestCases: testCases,
	}

	jobID, err := s.SubmitJob(ctx, job)
	if err != nil {
		return nil, err
	}

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

	job := &ExecutionJob{
		Code:      code,
		Language:  language,
		ProblemID: problemID,
	}

	jobID, err := s.SubmitJob(ctx, job)
	if err != nil {
		return nil, err
	}

	result, err := s.waitForJobResult(ctx, jobID)
	if err != nil {
		return nil, err
	}

	if result.Error != "" {
		return nil, fmt.Errorf(result.Error)
	}

	return result.ExecutionResults, nil
}

func (s *service) SubmitJob(ctx context.Context, job *ExecutionJob) (string, error) {
	startTime := time.Now()
	job.ID = uuid.New().String()
	job.SubmitTime = startTime

	s.logger.Printf("[JobID: %s] Submitting new job: Language=%s, ProblemID=%d, TestCases=%d",
		job.ID, job.Language, job.ProblemID, len(job.TestCases))

	jobData, err := json.Marshal(job)
	if err != nil {
		s.logger.Printf("[JobID: %s] ERROR: Failed to marshal job: %v", job.ID, err)
		return "", fmt.Errorf("failed to marshal job: %w", err)
	}
	s.logger.Printf("[JobID: %s] Job serialized successfully, size=%d bytes", job.ID, len(jobData))

	s.logger.Printf("[JobID: %s] Pushing job to Redis execution_queue", job.ID)
	if err := s.redisClient.RPush(ctx, "execution_queue", jobData).Err(); err != nil {
		s.logger.Printf("[JobID: %s] ERROR: Failed to push job to Redis queue: %v", job.ID, err)
		return "", fmt.Errorf("failed to queue job: %w", err)
	}
	s.logger.Printf("[JobID: %s] Job pushed to Redis queue successfully", job.ID)

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

	s.logDebug("[Worker-%d] Started", id)
	jobsProcessed := 0

	for {
		select {
		case job := <-s.jobQueue:
			jobsProcessed++
			s.logDebug("[Worker-%d] Processing job %s", id, job.ID)
			jobStartTime := time.Now()
			s.processJob(job)

			duration := time.Since(jobStartTime)
			if duration > 5*time.Second {
				s.logInfo("[Worker-%d] Slow job %s took %v", id, job.ID, duration)
			}

		case <-s.shutdownCh:
			s.logInfo("[Worker-%d] Shutting down after %d jobs", id, jobsProcessed)
			return
		}
	}
}

func (s *service) processJob(job *ExecutionJob) {
	ctx := context.Background()
	startTime := time.Now()

	s.updateJobStatus(ctx, job.ID, JobStatusRunning)

	var result *JobResult

	if job.ProblemID > 0 {
		testCases, err := s.repository.GetTestCasesByProblemID(ctx, job.ProblemID)

		if err != nil {
			s.logError("[JobID: %s] Failed to get test cases: %v", job.ID, err)
			result = &JobResult{
				JobID:       job.ID,
				Status:      JobStatusFailed,
				Error:       fmt.Sprintf("failed to get test cases: %v", err),
				CompletedAt: time.Now(),
			}
		} else if len(testCases) == 0 {
			result = &JobResult{
				JobID:       job.ID,
				Status:      JobStatusFailed,
				Error:       "no test cases found",
				CompletedAt: time.Now(),
			}
		} else {
			execResults, err := s.executeWithTestCasesInternal(ctx, job.Code, job.Language, testCases)
			completedAt := time.Now()

			if err != nil {
				s.logError("[JobID: %s] Execution failed: %v", job.ID, err)
				result = &JobResult{
					JobID:            job.ID,
					Status:           JobStatusFailed,
					ExecutionResults: execResults,
					Error:            err.Error(),
					CompletedAt:      completedAt,
				}
			} else {
				result = &JobResult{
					JobID:            job.ID,
					Status:           JobStatusCompleted,
					ExecutionResults: execResults,
					CompletedAt:      completedAt,
				}
			}
		}
	} else if len(job.TestCases) > 0 {
		execResults, err := s.executeWithTestCasesInternal(ctx, job.Code, job.Language, job.TestCases)
		completedAt := time.Now()

		if err != nil {
			result = &JobResult{
				JobID:            job.ID,
				Status:           JobStatusFailed,
				ExecutionResults: execResults,
				Error:            err.Error(),
				CompletedAt:      completedAt,
			}
		} else {
			result = &JobResult{
				JobID:            job.ID,
				Status:           JobStatusCompleted,
				ExecutionResults: execResults,
				CompletedAt:      completedAt,
			}
		}
	} else {
		execResult, err := s.executeCode(ctx, job.Code, job.Language, "")
		completedAt := time.Now()

		if err != nil {
			result = &JobResult{
				JobID:           job.ID,
				Status:          JobStatusFailed,
				ExecutionResult: execResult,
				Error:           err.Error(),
				CompletedAt:     completedAt,
			}
		} else {
			result = &JobResult{
				JobID:           job.ID,
				Status:          JobStatusCompleted,
				ExecutionResult: execResult,
				CompletedAt:     completedAt,
			}
		}
	}

	s.storeJobResult(ctx, result)

	processingTime := time.Since(startTime)
	if processingTime > 5*time.Second {
		s.logInfo("[JobID: %s] Slow job completed in %v", job.ID, processingTime)
	}
}

func (s *service) processRedisQueue() {
	s.logInfo("Redis queue processor started")
	jobsProcessed := 0
	startTime := time.Now()

	batchSize := 10
	batchBuffer := make([]string, 0, batchSize)

	for {
		select {
		case <-s.shutdownCh:
			s.logInfo("Redis queue processor shutting down after processing %d jobs (active for %v)",
				jobsProcessed, time.Since(startTime))
			return
		default:

			ctx := context.Background()

			batchBuffer = batchBuffer[:0]

			for i := 0; i < batchSize; i++ {
				result, err := s.redisClient.LPop(ctx, "execution_queue").Result()
				if err == redis.Nil {
					break
				} else if err != nil {
					s.logError("Failed to pop from Redis: %v", err)
					break
				}
				batchBuffer = append(batchBuffer, result)
			}

			for _, jobData := range batchBuffer {
				var job ExecutionJob
				if err := json.Unmarshal([]byte(jobData), &job); err != nil {
					s.logError("Failed to unmarshal job: %v", err)
					continue
				}

				jobsProcessed++
				s.logDebug("[JobID: %s] Processing job from Redis queue", job.ID)

				select {
				case s.jobQueue <- &job:
					s.logDebug("[JobID: %s] Job queued to worker pool", job.ID)
				default:

					s.logDebug("[JobID: %s] Internal queue full, pushing back to Redis", job.ID)
					go func(data string) {
						if err := s.redisClient.RPush(context.Background(), "execution_queue", data).Err(); err != nil {
							s.logError("Failed to push job back to Redis: %v", err)
						}
					}(jobData)
				}
			}

			if len(batchBuffer) == 0 {
				time.Sleep(10 * time.Millisecond)
			}

			if jobsProcessed > 0 && jobsProcessed%1000 == 0 {
				s.logInfo("Redis queue processor stats: processed %d jobs in %v",
					jobsProcessed, time.Since(startTime))
			}
		}
	}
}

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

				if pollCount%10 == 0 {
					s.logger.Printf("[JobID: %s] Still waiting for job result (polled %d times over %v)",
						jobID, pollCount, time.Since(startTime))
				}
				continue
			}

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

				if pollCount%10 == 0 {
					s.logger.Printf("[JobID: %s] Job still in progress with status %s (waited %v so far)",
						jobID, result.Status, time.Since(startTime))
				}
				continue
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

	statusKey := fmt.Sprintf("job_status:%s", result.JobID)
	s.logger.Printf("[JobID: %s] Removing job status key from Redis (key: %s)", result.JobID, statusKey)

	err = s.redisClient.Del(ctx, statusKey).Err()
	if err != nil {
		s.logger.Printf("[JobID: %s] ERROR: Failed to remove job status key from Redis: %v", result.JobID, err)
	}

	s.logger.Printf("[JobID: %s] Job result stored successfully in Redis (took %v)",
		result.JobID, time.Since(startTime))
}

func (s *service) reportMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			active := atomic.LoadInt64(&s.activeExecutions)
			queued := len(s.jobQueue)

			queueLen, _ := s.redisClient.LLen(context.Background(), "execution_queue").Result()

			s.logInfo("METRICS: Active: %d, Internal queue: %d, Redis queue: %d",
				active, queued, queueLen)

		case <-s.shutdownCh:
			return
		}
	}
}

func (s *service) Shutdown() {
	s.logger.Println("Shutting down executor service...")
	shutdownStart := time.Now()

	s.logger.Println("Signaling all goroutines to stop")
	close(s.shutdownCh)

	s.logger.Println("Waiting for all workers to complete their current jobs")
	waitStart := time.Now()
	s.workerWg.Wait()

	var trackedContainers []string
	if s.containerPool != nil {
		s.logger.Println("Cleaning up tracked containers in pool...")

		s.containerPool.mu.Lock()
		for _, container := range s.containerPool.containers {
			trackedContainers = append(trackedContainers, container.ID)
		}
		s.containerPool.mu.Unlock()

		s.logger.Printf("Found %d tracked containers to clean up", len(trackedContainers))
	}

	s.logger.Println("Finding all executor-pool containers...")
	findCmd := exec.Command("docker", "ps", "-q", "--filter", "name=executor-pool")
	output, err := findCmd.Output()

	allContainers := []string{}
	if err != nil {
		s.logger.Printf("Error finding executor-pool containers: %v", err)
	} else {
		containerIDs := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, id := range containerIDs {
			if id != "" {
				allContainers = append(allContainers, id)
			}
		}
		s.logger.Printf("Found %d executor-pool containers running", len(allContainers))
	}

	containersToRemove := make(map[string]bool)
	for _, id := range trackedContainers {
		containersToRemove[id] = true
	}
	for _, id := range allContainers {
		containersToRemove[id] = true
	}

	containersList := []string{}
	for id := range containersToRemove {
		containersList = append(containersList, id)
	}

	s.logger.Printf("Total of %d unique containers to remove", len(containersList))
	cleanupStart := time.Now()

	var wg sync.WaitGroup
	for _, containerID := range containersList {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			s.logger.Printf("Removing container %s", id[:12])
			cmd := exec.Command("docker", "rm", "-f", id)
			if output, err := cmd.CombinedOutput(); err != nil {
				s.logger.Printf("Error removing container %s: %v, output: %s", id[:12], err, output)
			} else {
				s.logger.Printf("Container %s removed successfully", id[:12])
			}
		}(containerID)
	}

	wg.Wait()
	s.logger.Printf("Container cleanup complete (took %v)", time.Since(cleanupStart))

	s.logger.Printf("All workers stopped gracefully (took %v)", time.Since(waitStart))
	s.logger.Printf("Executor service shutdown complete (took %v)", time.Since(shutdownStart))
}
