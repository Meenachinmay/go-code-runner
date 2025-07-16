package server

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"
)

type HTTPRequest struct {
	W         http.ResponseWriter
	R         *http.Request
	Handler   http.Handler
	Done      chan struct{}
	StartTime time.Time
}

type WorkerPool struct {
	workerCount  int
	jobQueue     chan *HTTPRequest
	shutdownCh   chan struct{}
	workerWg     sync.WaitGroup
	logger       *log.Logger
	maxQueueSize int
}

func NewWorkerPool(workerCount, maxQueueSize int, logger *log.Logger) *WorkerPool {
	return &WorkerPool{
		workerCount:  workerCount,
		jobQueue:     make(chan *HTTPRequest, maxQueueSize),
		shutdownCh:   make(chan struct{}),
		logger:       logger,
		maxQueueSize: maxQueueSize,
	}
}

func (wp *WorkerPool) Start() {
	wp.logger.Printf("Starting HTTP worker pool with %d workers", wp.workerCount)

	for i := 0; i < wp.workerCount; i++ {
		wp.workerWg.Add(1)
		go wp.worker(i)
	}

	wp.logger.Printf("HTTP worker pool started successfully")
}

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

type ResponseBuffer struct {
	http.ResponseWriter
	statusCode int
	buffer     []byte
}

func NewResponseBuffer(w http.ResponseWriter) *ResponseBuffer {
	return &ResponseBuffer{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rb *ResponseBuffer) WriteHeader(statusCode int) {
	rb.statusCode = statusCode
}

func (rb *ResponseBuffer) Write(b []byte) (int, error) {
	rb.buffer = append(rb.buffer, b...)
	return len(b), nil
}

func (rb *ResponseBuffer) Flush() {
	rb.ResponseWriter.WriteHeader(rb.statusCode)
	rb.ResponseWriter.Write(rb.buffer)
}

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
