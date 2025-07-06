package server

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go-code-runner/internal/code_executor"
	"go-code-runner/internal/handler"
	"go-code-runner/internal/middleware"
	"go-code-runner/internal/service/problems"
)

func NewRouter(
	db *pgxpool.Pool,
	problemService problems.Service,
	execSvc code_executor.Service,
	companyHandler *handler.CompanyHandler,
	codingTestHandler *handler.CodingTestHandler,
) *gin.Engine {
	r := gin.Default()

	// Health probe
	r.GET("/health", handler.MakeHealthHandler(db))

	// API v1
	v1 := r.Group("/api/v1")
	{
		v1.POST("/execute", handler.MakeExecuteHandler(execSvc))
		v1.GET("/problems", handler.MakeListProblemsHandler(problemService))
		v1.GET("/problems/:id", handler.MakeGetProblemHandler(problemService))

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
				auth.GET("/tests", codingTestHandler.GetCompanyTests)
			}

			apiAuth := companies.Group("")
			apiAuth.Use(middleware.APIKeyAuth())
			{
				apiAuth.POST("/tests/generate", codingTestHandler.GenerateTest)
			}
		}

		// Coding test end points.
		codingTests := v1.Group("/tests")
		{
			codingTests.GET("/:test_id/verify", codingTestHandler.VerifyTest)
			codingTests.POST("/:test_id/start", codingTestHandler.StartTest)
			codingTests.POST("/:test_id/submit", codingTestHandler.SubmitTest)
		}

	}

	return r
}
