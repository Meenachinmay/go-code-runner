package server

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"
)

// HTTPRequest represents a request to be processed by the worker pool
type HTTPRequest struct {
	W         http.ResponseWriter
	R         *http.Request
	Handler   http.Handler
	Done      chan struct{}
	StartTime time.Time
}

// WorkerPool manages a pool of workers for processing HTTP requests
type WorkerPool struct {
	workerCount  int
	jobQueue     chan *HTTPRequest
	shutdownCh   chan struct{}
	workerWg     sync.WaitGroup
	logger       *log.Logger
	maxQueueSize int
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(workerCount, maxQueueSize int, logger *log.Logger) *WorkerPool {
	return &WorkerPool{
		workerCount:  workerCount,
		jobQueue:     make(chan *HTTPRequest, maxQueueSize),
		shutdownCh:   make(chan struct{}),
		logger:       logger,
		maxQueueSize: maxQueueSize,
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	wp.logger.Printf("Starting HTTP worker pool with %d workers", wp.workerCount)

	for i := 0; i < wp.workerCount; i++ {
		wp.workerWg.Add(1)
		go wp.worker(i)
	}

	wp.logger.Printf("HTTP worker pool started successfully")
}

// worker processes jobs from the job queue
func (wp *WorkerPool) worker(id int) {
	defer wp.workerWg.Done()

	wp.logger.Printf("[Worker-%d] HTTP worker started", id)
	jobsProcessed := 0

	for {
		select {
		case job := <-wp.jobQueue:
			jobsProcessed++
			processingTime := time.Since(job.StartTime)
			wp.logger.Printf("[Worker-%d] Processing HTTP request after %v in queue", id, processingTime)

			job.Handler.ServeHTTP(job.W, job.R)
			close(job.Done)

			wp.logger.Printf("[Worker-%d] Completed HTTP request (took %v, total: %d)",
				id, time.Since(job.StartTime), jobsProcessed)

		case <-wp.shutdownCh:
			wp.logger.Printf("[Worker-%d] Shutting down after processing %d requests", id, jobsProcessed)
			return
		}
	}
}

// Submit submits a request to the worker pool
func (wp *WorkerPool) Submit(w http.ResponseWriter, r *http.Request, handler http.Handler) (bool, chan struct{}) {
	done := make(chan struct{})
	job := &HTTPRequest{
		W:         w,
		R:         r,
		Handler:   handler,
		Done:      done,
		StartTime: time.Now(),
	}

	select {
	case wp.jobQueue <- job:
		return true, done
	default:

		wp.logger.Printf("Worker pool queue is full, processing request directly")
		handler.ServeHTTP(w, r)
		close(done)
		return false, done
	}
}

// Shutdown gracefully shuts down the worker pool
func (wp *WorkerPool) Shutdown(ctx context.Context) {
	wp.logger.Println("Shutting down HTTP worker pool...")
	close(wp.shutdownCh)

	done := make(chan struct{})
	go func() {
		wp.workerWg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		wp.logger.Println("Shutdown deadline exceeded, some HTTP workers may still be running")
	case <-done:
		wp.logger.Println("All HTTP workers shut down gracefully")
	}
}

// ResponseBuffer is a custom ResponseWriter that buffers the response
type ResponseBuffer struct {
	http.ResponseWriter
	statusCode int
	buffer     []byte
}

// NewResponseBuffer creates a new ResponseBuffer
func NewResponseBuffer(w http.ResponseWriter) *ResponseBuffer {
	return &ResponseBuffer{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader captures the status code
func (rb *ResponseBuffer) WriteHeader(statusCode int) {
	rb.statusCode = statusCode
}

// Write captures the response body
func (rb *ResponseBuffer) Write(b []byte) (int, error) {
	rb.buffer = append(rb.buffer, b...)
	return len(b), nil
}

// Flush writes the buffered response to the original ResponseWriter
func (rb *ResponseBuffer) Flush() {
	rb.ResponseWriter.WriteHeader(rb.statusCode)
	rb.ResponseWriter.Write(rb.buffer)
}

// WorkerPoolHandler wraps an http.Handler with a worker pool
func WorkerPoolHandler(handler http.Handler, pool *WorkerPool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		r2 := r.WithContext(ctx)

		bufferHandler := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {

			buffer := NewResponseBuffer(rw)

			handler.ServeHTTP(buffer, r)

			buffer.Flush()
		})

		_, done := pool.Submit(w, r2, bufferHandler)

		<-done
	})
}
