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

		// Get problem from service
		problem, err := problemService.GetProblemByID(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to get problem: " + err.Error(),
			})
			return
		}

		// Get test cases for the problem (only non-hidden ones)
		testCases, err := problemService.GetTestCasesByProblemID(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to get test cases: " + err.Error(),
			})
			return
		}

		// Filter out hidden test cases
		var visibleTestCases []*models.TestCase
		for _, tc := range testCases {
			if !tc.IsHidden {
				visibleTestCases = append(visibleTestCases, tc)
			}
		}

		// Return problem with test cases
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
		// Get all problems from service
		problems, err := problemService.ListProblems(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to list problems: " + err.Error(),
			})
			return
		}

		// Return problems
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"problems": problems,
		})
	}
}
