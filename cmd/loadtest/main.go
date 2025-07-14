package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Test configuration
type Config struct {
	BaseURL          string
	TotalRequests    int
	ConcurrentUsers  int
	RampUpTime       time.Duration
	TestDuration     time.Duration
	RequestTimeout   time.Duration
	JobPollInterval  time.Duration
	JobPollTimeout   time.Duration
}

// Request/Response types matching your API
type ExecuteRequest struct {
	Language  string `json:"language"`
	Code      string `json:"code"`
	ProblemID int    `json:"problem_id,omitempty"`
}

type ExecuteResponse struct {
	Success bool   `json:"success"`
	JobID   string `json:"job_id"`
	Message string `json:"message,omitempty"`
}

type JobStatusResponse struct {
	Success     bool                `json:"success"`
	JobID       string              `json:"job_id"`
	Status      string              `json:"status"`
	Output      string              `json:"output,omitempty"`
	Error       string              `json:"error,omitempty"`
	TestResults []map[string]interface{} `json:"test_results,omitempty"`
}

// Metrics collection
type Metrics struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalJobs          int64
	CompletedJobs      int64
	FailedJobs         int64
	TimeoutJobs        int64

	SubmitLatencies    []time.Duration
	ExecutionLatencies []time.Duration
	mu                 sync.Mutex
}

func (m *Metrics) RecordSubmitLatency(d time.Duration) {
	m.mu.Lock()
	m.SubmitLatencies = append(m.SubmitLatencies, d)
	m.mu.Unlock()
}

func (m *Metrics) RecordExecutionLatency(d time.Duration) {
	m.mu.Lock()
	m.ExecutionLatencies = append(m.ExecutionLatencies, d)
	m.mu.Unlock()
}

// Test code samples
var testCodes = []string{
	// Simple Hello World
	`package main
import "fmt"
func main() {
    fmt.Println("Hello, World!")
}`,

	// Calculate sum
	`package main
import "fmt"
func main() {
    var a, b int
    fmt.Scan(&a, &b)
    fmt.Println(a + b)
}`,

	// String manipulation
	`package main
import (
    "fmt"
    "strings"
)
func main() {
    text := "Hello from load test"
    fmt.Println(strings.ToUpper(text))
    fmt.Println(strings.ToLower(text))
}`,

	// Array operations
	`package main
import (
    "fmt"
    "sort"
)
func main() {
    nums := []int{5, 2, 8, 1, 9, 3}
    sort.Ints(nums)
    fmt.Println(nums)
}`,

	// Fibonacci
	`package main
import "fmt"
func fibonacci(n int) int {
    if n <= 1 {
        return n
    }
    return fibonacci(n-1) + fibonacci(n-2)
}
func main() {
    for i := 0; i < 10; i++ {
        fmt.Printf("%d ", fibonacci(i))
    }
    fmt.Println()
}`,
}

// Load test worker
type Worker struct {
	ID       int
	Config   *Config
	Client   *http.Client
	Metrics  *Metrics
	wg       *sync.WaitGroup
}

func (w *Worker) Run(ctx context.Context) {
	defer w.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			w.executeTest(ctx)
		}
	}
}

func (w *Worker) executeTest(ctx context.Context) {
	// Select random code
	code := testCodes[rand.Intn(len(testCodes))]

	// Submit job
	submitStart := time.Now()
	jobID, err := w.submitJob(ctx, code)
	submitLatency := time.Since(submitStart)

	atomic.AddInt64(&w.Metrics.TotalRequests, 1)

	if err != nil {
		atomic.AddInt64(&w.Metrics.FailedRequests, 1)
		log.Printf("[Worker %d] Failed to submit job: %v", w.ID, err)
		return
	}

	atomic.AddInt64(&w.Metrics.SuccessfulRequests, 1)
	atomic.AddInt64(&w.Metrics.TotalJobs, 1)
	w.Metrics.RecordSubmitLatency(submitLatency)

	// Poll for job completion
	executionStart := time.Now()
	completed, err := w.pollJobStatus(ctx, jobID)
	executionLatency := time.Since(executionStart)

	if err != nil {
		log.Printf("[Worker %d] Job %s failed: %v", w.ID, jobID, err)
		atomic.AddInt64(&w.Metrics.FailedJobs, 1)
	} else if !completed {
		log.Printf("[Worker %d] Job %s timed out", w.ID, jobID)
		atomic.AddInt64(&w.Metrics.TimeoutJobs, 1)
	} else {
		atomic.AddInt64(&w.Metrics.CompletedJobs, 1)
		w.Metrics.RecordExecutionLatency(executionLatency)
	}
}

func (w *Worker) submitJob(ctx context.Context, code string) (string, error) {
	req := ExecuteRequest{
		Language: "go",
		Code:     code,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		w.Config.BaseURL+"/api/v1/execute", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := w.Client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	var execResp ExecuteResponse
	if err := json.NewDecoder(resp.Body).Decode(&execResp); err != nil {
		return "", err
	}

	if !execResp.Success {
		return "", fmt.Errorf("execution failed: %s", execResp.Message)
	}

	return execResp.JobID, nil
}

func (w *Worker) pollJobStatus(ctx context.Context, jobID string) (bool, error) {
	pollCtx, cancel := context.WithTimeout(ctx, w.Config.JobPollTimeout)
	defer cancel()

	ticker := time.NewTicker(w.Config.JobPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pollCtx.Done():
			return false, nil // Timeout
		case <-ticker.C:
			status, err := w.getJobStatus(pollCtx, jobID)
			if err != nil {
				return false, err
			}

			switch status.Status {
			case "completed":
				return true, nil
			case "failed":
				return false, fmt.Errorf("job failed: %s", status.Error)
			}
		}
	}
}

func (w *Worker) getJobStatus(ctx context.Context, jobID string) (*JobStatusResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/api/v1/execute/job/%s", w.Config.BaseURL, jobID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := w.Client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var statusResp JobStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return nil, err
	}

	return &statusResp, nil
}

// Main load test orchestrator
func main() {
	var (
		baseURL         = flag.String("url", "http://localhost:8080", "Base URL of the API")
		totalRequests   = flag.Int("requests", 10000, "Total number of requests")
		concurrentUsers = flag.Int("concurrent", 100, "Number of concurrent users")
		rampUpTime      = flag.Duration("rampup", 30*time.Second, "Ramp up time")
		testDuration    = flag.Duration("duration", 5*time.Minute, "Test duration")
		requestTimeout  = flag.Duration("timeout", 30*time.Second, "Request timeout")
	)
	flag.Parse()

	config := &Config{
		BaseURL:          *baseURL,
		TotalRequests:    *totalRequests,
		ConcurrentUsers:  *concurrentUsers,
		RampUpTime:       *rampUpTime,
		TestDuration:     *testDuration,
		RequestTimeout:   *requestTimeout,
		JobPollInterval:  100 * time.Millisecond,
		JobPollTimeout:   60 * time.Second,
	}

	fmt.Printf("Starting load test:\n")
	fmt.Printf("- Base URL: %s\n", config.BaseURL)
	fmt.Printf("- Total Requests: %d\n", config.TotalRequests)
	fmt.Printf("- Concurrent Users: %d\n", config.ConcurrentUsers)
	fmt.Printf("- Ramp Up Time: %s\n", config.RampUpTime)
	fmt.Printf("- Test Duration: %s\n", config.TestDuration)
	fmt.Println()

	// Initialize metrics
	metrics := &Metrics{
		SubmitLatencies:    make([]time.Duration, 0, config.TotalRequests),
		ExecutionLatencies: make([]time.Duration, 0, config.TotalRequests),
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.TestDuration)
	defer cancel()

	// Create workers
	var wg sync.WaitGroup
	workers := make([]*Worker, config.ConcurrentUsers)

	// HTTP client with connection pooling
	client := &http.Client{
		Timeout: config.RequestTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        config.ConcurrentUsers * 2,
			MaxIdleConnsPerHost: config.ConcurrentUsers,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	// Start workers with ramp up
	fmt.Println("Starting workers...")
	rampUpDelay := config.RampUpTime / time.Duration(config.ConcurrentUsers)

	for i := 0; i < config.ConcurrentUsers; i++ {
		workers[i] = &Worker{
			ID:      i,
			Config:  config,
			Client:  client,
			Metrics: metrics,
			wg:      &wg,
		}

		wg.Add(1)
		go workers[i].Run(ctx)

		if i < config.ConcurrentUsers-1 {
			time.Sleep(rampUpDelay)
		}
	}

	fmt.Println("All workers started. Running test...")

	// Monitor progress
	go monitorProgress(ctx, metrics)

	// Wait for completion
	wg.Wait()

	// Print final results
	printResults(metrics)
}

func monitorProgress(ctx context.Context, metrics *Metrics) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			elapsed := time.Since(startTime)
			totalReqs := atomic.LoadInt64(&metrics.TotalRequests)
			completedJobs := atomic.LoadInt64(&metrics.CompletedJobs)

			fmt.Printf("[%s] Requests: %d, Jobs Completed: %d, Rate: %.2f req/s\n",
				elapsed.Round(time.Second),
				totalReqs,
				completedJobs,
				float64(totalReqs)/elapsed.Seconds(),
			)
		}
	}
}

func printResults(metrics *Metrics) {
	fmt.Println("\n=== LOAD TEST RESULTS ===")
	fmt.Printf("Total Requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Successful Requests: %d (%.2f%%)\n",
		metrics.SuccessfulRequests,
		float64(metrics.SuccessfulRequests)/float64(metrics.TotalRequests)*100)
	fmt.Printf("Failed Requests: %d (%.2f%%)\n",
		metrics.FailedRequests,
		float64(metrics.FailedRequests)/float64(metrics.TotalRequests)*100)

	fmt.Println("\nJob Statistics:")
	fmt.Printf("Total Jobs: %d\n", metrics.TotalJobs)
	fmt.Printf("Completed Jobs: %d (%.2f%%)\n",
		metrics.CompletedJobs,
		float64(metrics.CompletedJobs)/float64(metrics.TotalJobs)*100)
	fmt.Printf("Failed Jobs: %d (%.2f%%)\n",
		metrics.FailedJobs,
		float64(metrics.FailedJobs)/float64(metrics.TotalJobs)*100)
	fmt.Printf("Timeout Jobs: %d (%.2f%%)\n",
		metrics.TimeoutJobs,
		float64(metrics.TimeoutJobs)/float64(metrics.TotalJobs)*100)

	// Calculate latency percentiles
	if len(metrics.SubmitLatencies) > 0 {
		fmt.Println("\nSubmit Latencies:")
		printLatencyStats(metrics.SubmitLatencies)
	}

	if len(metrics.ExecutionLatencies) > 0 {
		fmt.Println("\nExecution Latencies (job completion):")
		printLatencyStats(metrics.ExecutionLatencies)
	}
}

func printLatencyStats(latencies []time.Duration) {
	// Sort for percentile calculation
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)

	// Simple bubble sort for percentiles
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	p50 := sorted[len(sorted)*50/100]
	p90 := sorted[len(sorted)*90/100]
	p95 := sorted[len(sorted)*95/100]
	p99 := sorted[len(sorted)*99/100]

	var sum time.Duration
	for _, d := range latencies {
		sum += d
	}
	avg := sum / time.Duration(len(latencies))

	fmt.Printf("  Average: %v\n", avg)
	fmt.Printf("  P50: %v\n", p50)
	fmt.Printf("  P90: %v\n", p90)
	fmt.Printf("  P95: %v\n", p95)
	fmt.Printf("  P99: %v\n", p99)
	fmt.Printf("  Min: %v\n", sorted[0])
	fmt.Printf("  Max: %v\n", sorted[len(sorted)-1])
}