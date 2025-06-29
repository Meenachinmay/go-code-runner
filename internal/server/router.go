package server

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"go-code-runner/internal/code_executor"
	"go-code-runner/internal/handler"
	"go-code-runner/internal/middleware"
	"go-code-runner/internal/repository"
)

func NewRouter(
	db *pgxpool.Pool,
	repo repository.Repository,
	execSvc code_executor.Service,
	companyHandler *handler.CompanyHandler,
) *gin.Engine {
	r := gin.Default()

	// Health probe
	r.GET("/health", handler.MakeHealthHandler(db))

	// API v1
	v1 := r.Group("/api/v1")
	{
		v1.POST("/execute", handler.MakeExecuteHandler(execSvc))
		v1.GET("/problems", handler.MakeListProblemsHandler(repo))
		v1.GET("/problems/:id", handler.MakeGetProblemHandler(repo))

		// Company endpoints
		companies := v1.Group("/companies")
		{
			companies.POST("/register", companyHandler.Register)
			companies.POST("/login", companyHandler.Login)

			// Protected routes
			auth := companies.Group("")
			auth.Use(middleware.JWTAuth())
			{
				auth.POST("/api-key", companyHandler.GenerateAPIKey)
				auth.POST("/client-id", companyHandler.GenerateClientID)
			}
		}
	}

	return r
}
