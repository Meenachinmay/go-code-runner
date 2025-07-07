package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go-code-runner/internal/repository/company"
	"net/http"
)

// companyRepo is a package-level variable to store the company repository
var companyRepo company.Repository

// InitAPIKeyAuth initializes the company repository for API key authentication
func InitAPIKeyAuth(db *pgxpool.Pool) {
	companyRepo = company.New(db)
}

func APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
			c.Abort()
			return
		}

		company, err := companyRepo.GetCompanyByAPIKey(c.Request.Context(), apiKey)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		c.Set("company_id", company.ID)
		c.Set("company", company)
		c.Next()
	}
}
