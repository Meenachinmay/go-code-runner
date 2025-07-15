package handler

import (
	"go-code-runner/internal/code_executor"
	"go-code-runner/internal/models"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ExecuteRequest struct {
	Language  string `json:"language" binding:"required"`
	Code      string `json:"code" binding:"required"`
	ProblemID int    `json:"problem_id,omitempty"`
}

type ExecuteResponse struct {
	Success     bool                `json:"success"`
	Output      string              `json:"output,omitempty"`
	Error       string              `json:"error,omitempty"`
	TestResults []models.TestResult `json:"test_results,omitempty"`
}

type AsyncExecuteResponse struct {
	Success bool   `json:"success"`
	JobID   string `json:"job_id"`
	Message string `json:"message,omitempty"`
}

func MakeExecuteHandler(executorService code_executor.Service) gin.HandlerFunc {
	return func(c *gin.Context) {

		var req ExecuteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ExecuteResponse{
				Success: false,
				Error:   "Invalid request payload: " + err.Error(),
			})
			return
		}

		if req.Language != "go" {
			c.JSON(http.StatusBadRequest, ExecuteResponse{
				Success: false,
				Error:   "Unsupported language. Only 'go' is supported.",
			})
			return
		}

		var job *code_executor.ExecutionJob
		var jobID string
		var err error

		if req.ProblemID > 0 {
			log.Printf("Submitting async job for problem ID: %d", req.ProblemID)
			job = &code_executor.ExecutionJob{
				Code:      req.Code,
				Language:  req.Language,
				ProblemID: req.ProblemID,
			}
		} else {
			log.Printf("Submitting async job for code execution")
			job = &code_executor.ExecutionJob{
				Code:     req.Code,
				Language: req.Language,
			}
		}

		jobID, err = executorService.SubmitJob(c.Request.Context(), job)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ExecuteResponse{
				Success: false,
				Error:   "Failed to submit job: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusAccepted, AsyncExecuteResponse{
			Success: true,
			JobID:   jobID,
			Message: "Job submitted successfully. Use GET /api/v1/execute/job/" + jobID + " to check status.",
		})
	}
}

func MakeJobStatusHandler(executorService code_executor.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobID := c.Param("job_id")

		result, err := executorService.GetJobResult(c.Request.Context(), jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Job not found",
			})
			return
		}

		response := gin.H{
			"success": true,
			"job_id":  jobID,
			"status":  string(result.Status),
		}

		if result.Status == code_executor.JobStatusCompleted {
			if result.ExecutionResult != nil {
				response["output"] = result.ExecutionResult.Output
				response["error"] = result.ExecutionResult.Error
			}
			if result.ExecutionResults != nil {
				response["test_results"] = result.ExecutionResults.TestResults
			}
		} else if result.Status == code_executor.JobStatusFailed {
			response["success"] = false
			response["error"] = result.Error
		}

		c.JSON(http.StatusOK, response)
	}
}
