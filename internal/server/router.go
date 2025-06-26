package server

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"go-code-runner/internal/code_executor"
	"go-code-runner/internal/handler"
	"go-code-runner/internal/repository"
)

func NewRouter(
	db *pgxpool.Pool,
	repo repository.Repository,
	execSvc code_executor.Service,
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
	}

	return r
}
