package handler

import (
	"go-code-runner/internal/models"
	"go-code-runner/internal/service/problems"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// MakeGetProblemHandler creates a handler for retrieving a problem by ID
func MakeGetProblemHandler(problemService problems.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse problem ID from URL
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid problem ID",
			})
			return
		}

		problem, err := problemService.GetProblemByID(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to get problem: " + err.Error(),
			})
			return
		}

		testCases, err := problemService.GetTestCasesByProblemID(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to get test cases: " + err.Error(),
			})
			return
		}

		var visibleTestCases []*models.TestCase
		for _, tc := range testCases {
			if !tc.IsHidden {
				visibleTestCases = append(visibleTestCases, tc)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"problem":    problem,
			"test_cases": visibleTestCases,
		})
	}
}

// MakeListProblemsHandler creates a handler for listing all problems
func MakeListProblemsHandler(problemService problems.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		problems, err := problemService.ListProblems(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to list problems: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"problems": problems,
		})
	}
}
