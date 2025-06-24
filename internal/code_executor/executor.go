package code_executor

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type ExecutionResult struct {
	Output string
	Error  string
}

type service struct {
	executionTimeout time.Duration
	logger           *log.Logger
	imageCache       map[string]bool

	buildCacheDir string
	modCacheDir   string
}

func NewService(timeout time.Duration, logger *log.Logger) Service {
	buildCacheDir, _ := os.MkdirTemp("", "go-build-cache-*")
	modCache, _ := os.MkdirTemp("", "go-mod-cache-*")
	return &service{
		executionTimeout: timeout,
		logger:           logger,
		imageCache:       make(map[string]bool),
		buildCacheDir:    buildCacheDir,
		modCacheDir:      modCache,
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

func (s *service) Execute(ctx context.Context, code string, language string) (*ExecutionResult, error) {
	overallStart := time.Now()
	s.logger.Printf("-------------------------------------------------")
	s.logger.Println("Received new execution request.")

	s.ensureDockerImageAvailable("golang:1.22-alpine")

	runID := uuid.New().String()
	s.logger.Printf("[%s] Creating temp directory...", runID)
	dirStart := time.Now()

	tempDir, err := os.MkdirTemp("", "runbox-"+runID+"-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)
	s.logger.Printf("[%s] Temp directory created at %s. (took %v)", runID, tempDir, time.Since(dirStart))

	s.logger.Printf("[%s] Writing code to file...", runID)
	writeStart := time.Now()

	codeFileName := "main.go"
	codePath := filepath.Join(tempDir, codeFileName)
	if err := os.WriteFile(codePath, []byte(code), 0644); err != nil {
		return nil, fmt.Errorf("failed to write code to file: %w", err)
	}
	s.logger.Printf("[%s] Code written to %s. (took %v)", runID, codePath, time.Since(writeStart))

	execCtx, cancel := context.WithTimeout(ctx, s.executionTimeout)
	defer cancel()

	volumeMount := fmt.Sprintf("%s:/app:ro", tempDir)
	cacheMount := fmt.Sprintf("%s:/root/.cache/go-build:rw", s.buildCacheDir)
	modMount := fmt.Sprintf("%s:/go/pkg/mod:rw", s.modCacheDir)

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
		"sh", "-c", fmt.Sprintf("cd /app && GOFLAGS=-mod=readonly go run %s", codeFileName),
	}

	cmd := exec.CommandContext(execCtx, "docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	s.logger.Printf("[%s] Executing docker command: docker %v", runID, args)
	dockerStart := time.Now()

	err = cmd.Run()

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

	s.logger.Printf("[%s] Total request processing time: %v", runID, time.Since(overallStart))
	s.logger.Printf("-------------------------------------------------")

	return result, nil
}
