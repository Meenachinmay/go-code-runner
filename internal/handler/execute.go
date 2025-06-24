package handler

import (
	"go-code-runner/internal/code_executor"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ExecuteRequest struct {
	Language string `json:"language" binding:"required"`
	Code     string `json:"code" binding:"required"`
}

type ExecuteResponse struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

func MakeExecuteHandler(executorService code_executor.Service) gin.HandlerFunc {
	return func(c *gin.Context) {

		log.Println("--- Execute handler invoked ---")
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
