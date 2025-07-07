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

		if req.ProblemID > 0 {
			log.Printf("Executing code for problem ID: %d", req.ProblemID)

			results, err := executorService.ExecuteForProblem(c.Request.Context(), req.Code, req.Language, req.ProblemID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, ExecuteResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}

			c.JSON(http.StatusOK, ExecuteResponse{
				Success:     results.Success,
				TestResults: results.TestResults,
			})
			return
		}

		result, err := executorService.Execute(c.Request.Context(), req.Code, req.Language)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ExecuteResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		if result.Error != "" {
			c.JSON(http.StatusOK, ExecuteResponse{
				Success: false,
				Output:  result.Output,
				Error:   result.Error,
			})
			return
		}

		c.JSON(http.StatusOK, ExecuteResponse{
			Success: true,
			Output:  result.Output,
		})
	}
}
