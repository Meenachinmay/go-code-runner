package grpc

import (
	"context"
	"fmt"
	executorpb "go-code-runner/go-code-runner-microservice/proto/executor/v1"
	"go-code-runner/internal/code_executor"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
)

type Server struct {
	executorpb.UnimplementedExecutorServiceServer
	executorService code_executor.Service
	logger          *log.Logger
}

func NewServer(executorService code_executor.Service, logger *log.Logger) *Server {
	return &Server{
		executorService: executorService,
		logger:          logger,
	}
}

func (s *Server) Execute(ctx context.Context, req *executorpb.ExecuteRequest) (*executorpb.ExecuteResponse, error) {
	if req.Language != "go" {
		return &executorpb.ExecuteResponse{
			Success: false,
			Error:   "Unsupported language. Only 'go' is supported.",
		}, nil
	}

	job := &code_executor.ExecutionJob{
		Code:     req.Code,
		Language: req.Language,
	}

	if req.ProblemId > 0 {
		job.ProblemID = int(req.ProblemId)
		s.logger.Printf("Submitting async job for problem ID: %d", req.ProblemId)
	} else {
		s.logger.Printf("Submitting async job for code execution")
	}

	jobID, err := s.executorService.SubmitJob(ctx, job)
	if err != nil {
		s.logger.Printf("Failed to submit job: %v", err)
		return &executorpb.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to submit job: %v", err),
		}, nil
	}

	return &executorpb.ExecuteResponse{
		Success: true,
		JobId:   jobID,
		Message: fmt.Sprintf("Job submitted successfully. Use GET /api/v1/execute/job/%s to check status.", jobID),
	}, nil
}

func (s *Server) GetJobStatus(ctx context.Context, req *executorpb.GetJobStatusRequest) (*executorpb.GetJobStatusResponse, error) {
	result, err := s.executorService.GetJobResult(ctx, req.JobId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Job not found")
	}

	response := &executorpb.GetJobStatusResponse{
		Success: true,
		JobId:   req.JobId,
		Status:  string(result.Status),
	}

	if result.Status == code_executor.JobStatusCompleted {
		if result.ExecutionResult != nil {
			response.Output = result.ExecutionResult.Output
			response.Error = result.ExecutionResult.Error
		}
		if result.ExecutionResults != nil {
			response.TestResults = make([]*executorpb.TestResult, len(result.ExecutionResults.TestResults))
			for i, tr := range result.ExecutionResults.TestResults {
				response.TestResults[i] = &executorpb.TestResult{
					TestCaseId:     int32(tr.TestCaseID),
					Input:          tr.Input,
					ExpectedOutput: tr.ExpectedOutput,
					ActualOutput:   tr.ActualOutput,
					Error:          tr.Error,
					Passed:         tr.Passed,
				}
			}
		}
	} else if result.Status == code_executor.JobStatusFailed {
		response.Success = false
		response.Error = result.Error
	}

	return response, nil
}