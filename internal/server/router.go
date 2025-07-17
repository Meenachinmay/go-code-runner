package server

import (
	"github.com/gin-contrib/cors"
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
	r := gin.New()
	r.Use(gin.Recovery())

	// Configure CORS middleware
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:5173"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	config.AllowCredentials = true
	r.Use(cors.New(config))

	r.Use(middleware.RateLimit(100000))

	r.GET("/health", handler.MakeHealthHandler(db))

	v1 := r.Group("/api/v1")
	{

		v1.POST("/execute", handler.MakeExecuteHandler(execSvc))

		v1.GET("/execute/job/:job_id", handler.MakeJobStatusHandler(execSvc))

		v1.GET("/problems", handler.MakeListProblemsHandler(problemService))
		v1.GET("/problems/:id", handler.MakeGetProblemHandler(problemService))

		companies := v1.Group("/companies")
		{
			companies.POST("/register", companyHandler.Register)
			companies.POST("/login", companyHandler.Login)

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

		codingTests := v1.Group("/tests")
		{
			codingTests.GET("/:test_id/verify", codingTestHandler.VerifyTest)
			codingTests.POST("/:test_id/start", codingTestHandler.StartTest)
			codingTests.POST("/:test_id/submit", codingTestHandler.SubmitTest)
		}
	}

	return r
}
