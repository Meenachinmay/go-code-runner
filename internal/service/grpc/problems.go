package grpc

import (
	"context"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	problemspb "go-code-runner/go-code-runner-microservice/proto/problems/v1"
	"go-code-runner/internal/models"
	"go-code-runner/internal/service/problems"
)

type ProblemServer struct {
	problemspb.UnimplementedProblemServiceServer
	problemService problems.Service
	logger         *log.Logger
}

func NewProblemServer(problemService problems.Service, logger *log.Logger) *ProblemServer {
	return &ProblemServer{
		problemService: problemService,
		logger:         logger,
	}
}

func (s *ProblemServer) GetProblem(ctx context.Context, req *problemspb.GetProblemRequest) (*problemspb.GetProblemResponse, error) {
	problem, err := s.problemService.GetProblemByID(ctx, int(req.Id))
	if err != nil {
		s.logger.Printf("Failed to get problem with ID %d: %v", req.Id, err)
		return nil, status.Errorf(codes.NotFound, "Problem not found: %v", err)
	}

	return &problemspb.GetProblemResponse{
		Problem: convertProblemToProto(problem),
	}, nil
}

func (s *ProblemServer) ListProblems(ctx context.Context, req *problemspb.ListProblemsRequest) (*problemspb.ListProblemsResponse, error) {
	problems, err := s.problemService.ListProblems(ctx)
	if err != nil {
		s.logger.Printf("Failed to list problems: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to list problems: %v", err)
	}

	protoProblems := make([]*problemspb.Problem, len(problems))
	for i, problem := range problems {
		protoProblems[i] = convertProblemToProto(problem)
	}

	return &problemspb.ListProblemsResponse{
		Problems: protoProblems,
	}, nil
}

func (s *ProblemServer) GetTestCasesByProblemID(ctx context.Context, req *problemspb.GetTestCasesByProblemIDRequest) (*problemspb.GetTestCasesByProblemIDResponse, error) {
	testCases, err := s.problemService.GetTestCasesByProblemID(ctx, int(req.ProblemId))
	if err != nil {
		s.logger.Printf("Failed to get test cases for problem ID %d: %v", req.ProblemId, err)
		return nil, status.Errorf(codes.Internal, "Failed to get test cases: %v", err)
	}

	protoTestCases := make([]*problemspb.TestCase, len(testCases))
	for i, testCase := range testCases {
		protoTestCases[i] = convertTestCaseToProto(testCase)
	}

	return &problemspb.GetTestCasesByProblemIDResponse{
		TestCases: protoTestCases,
	}, nil
}

func convertProblemToProto(problem *models.Problem) *problemspb.Problem {
	return &problemspb.Problem{
		Id:          int32(problem.ID),
		Title:       problem.Title,
		Description: problem.Description,
		Difficulty:  problem.Difficulty,
		CreatedAt:   timestamppb.New(problem.CreatedAt),
		UpdatedAt:   timestamppb.New(problem.UpdatedAt),
	}
}

func convertTestCaseToProto(testCase *models.TestCase) *problemspb.TestCase {
	return &problemspb.TestCase{
		Id:             int32(testCase.ID),
		ProblemId:      int32(testCase.ProblemID),
		Input:          testCase.Input,
		ExpectedOutput: testCase.ExpectedOutput,
		IsHidden:       testCase.IsHidden,
		CreatedAt:      timestamppb.New(testCase.CreatedAt),
		UpdatedAt:      timestamppb.New(testCase.UpdatedAt),
	}
}
