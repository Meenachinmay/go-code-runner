package handler

import (
	"github.com/gin-gonic/gin"
	"go-code-runner/internal/service/coding_test"
	"net/http"
)

type CodingTestHandler struct {
	service coding_test.Service
}

func NewCodingTestHandler(service coding_test.Service) *CodingTestHandler {
	return &CodingTestHandler{service: service}
}

// GenerateTest handles POST /api/v1/companies/tests/generate
func (h *CodingTestHandler) GenerateTest(c *gin.Context) {
	companyID, exists := c.Get("company_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		ProblemID       int `json:"problem_id" binding:"required"`
		ExpiresInHours  int `json:"expires_in_hours" binding:"required,min=1,max=168"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	test, link, err := h.service.GenerateTest(c.Request.Context(), companyID.(int), req.ProblemID, req.ExpiresInHours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"test": test,
		"link": link,
	})
}

// VerifyTest handles GET /api/v1/tests/:test_id/verify
func (h *CodingTestHandler) VerifyTest(c *gin.Context) {
	testID := c.Param("test_id")

	test, err := h.service.VerifyTest(c.Request.Context(), testID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"test_id": test.ID,
		"problem_id": test.ProblemID,
		"status": test.Status,
		"test_duration_minutes": test.TestDurationMinutes,
	})
}

// StartTest handles POST /api/v1/tests/:test_id/start
func (h *CodingTestHandler) StartTest(c *gin.Context) {
	testID := c.Param("test_id")

	var req struct {
		CandidateName  string `json:"candidate_name" binding:"required"`
		CandidateEmail string `json:"candidate_email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.StartTest(c.Request.Context(), testID, req.CandidateName, req.CandidateEmail); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "test started successfully"})
}

// SubmitTest handles POST /api/v1/tests/:test_id/submit
func (h *CodingTestHandler) SubmitTest(c *gin.Context) {
	testID := c.Param("test_id")

	var req struct {
		Code             string `json:"code" binding:"required"`
		PassedPercentage int    `json:"passed_percentage" binding:"min=0,max=100"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.SubmitTest(c.Request.Context(), testID, req.Code, req.PassedPercentage); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "test submitted successfully"})
}

// GetCompanyTests handles GET /api/v1/companies/tests
func (h *CodingTestHandler) GetCompanyTests(c *gin.Context) {
	companyID, _ := c.Get("company_id")

	tests, err := h.service.GetCompanyTests(c.Request.Context(), companyID.(int))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tests": tests})
}