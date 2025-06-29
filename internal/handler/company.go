package handler

import (
	"net/http"

	svc "go-code-runner/internal/service/company"

	"github.com/gin-gonic/gin"
)

type CompanyHandler struct{ svc svc.Service }

func NewCompanyHandler(s svc.Service) *CompanyHandler { return &CompanyHandler{svc: s} }

func (h *CompanyHandler) Register(c *gin.Context) {
	var in struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	comp, err := h.svc.Register(c.Request.Context(), in.Name, in.Email, in.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "company": comp})
}

func (h *CompanyHandler) Login(c *gin.Context) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	comp, token, err := h.svc.Login(c.Request.Context(), in.Email, in.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "company": comp, "token": token})
}

func (h *CompanyHandler) GenerateAPIKey(c *gin.Context) {
	// Get company ID from JWT token
	companyID, exists := c.Get("company_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	key, err := h.svc.GenerateAPIKey(c.Request.Context(), companyID.(int))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "api_key": key})
}

func (h *CompanyHandler) GenerateClientID(c *gin.Context) {
	// Get company ID from JWT token
	companyID, exists := c.Get("company_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	clientID, err := h.svc.GenerateClientID(c.Request.Context(), companyID.(int))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "client_id": clientID})
}
