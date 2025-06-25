package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go-code-runner/internal/code_executor"
	"go-code-runner/internal/config"
	"go-code-runner/internal/handler"
	"go-code-runner/internal/platform/database"
	"go-code-runner/internal/repository"
	"log"
	"os"
)

func main() {
	logger := log.New(os.Stdout, "CODE-RUNNER: ", log.LstdFlags|log.Lmicroseconds)

	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("failed to load configuration: %v", err)
	}

	ctx := context.Background()
	dbpool, err := database.New(ctx, cfg.DBConnStr)
	if err != nil {
		logger.Fatalf("failed to connect to database: %v", err)
	}
	defer dbpool.Close()
	logger.Println("database connection pool established")

	logger.Println("checking for pending database migrations…")
	if err := database.Migrate(ctx, dbpool, "db/migrations", logger); err != nil {
		logger.Fatalf("migration failed: %v", err)
	}
	logger.Println("database is up-to-date")

	repo := repository.New(dbpool)
	executorService := code_executor.NewService(cfg.ExecutionTimeout, logger, repo)

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		if err := dbpool.Ping(c.Request.Context()); err != nil {
			c.JSON(503, gin.H{"status": "db_error"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
	{
		v1.POST("/execute", handler.MakeExecuteHandler(executorService))
		v1.GET("/problems", handler.MakeListProblemsHandler(repo))
		v1.GET("/problems/:id", handler.MakeGetProblemHandler(repo))
	}

	addr := ":" + cfg.ServerPort
	logger.Printf("starting HTTP server on %s", addr)
	if err := r.Run(addr); err != nil {
		logger.Fatalf("server error: %v", err)
	}
}
