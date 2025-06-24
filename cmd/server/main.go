package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go-code-runner/internal/code_executor"
	"go-code-runner/internal/config"
	"go-code-runner/internal/handler"
	"go-code-runner/internal/platform/database"
	"log"
	"os"
)

func main() {
	logger := log.New(os.Stdout, "CODE-RUNNER: ", log.LstdFlags|log.Lmicroseconds)

	// Load .env file
	if err := godotenv.Load(); err != nil {
		logger.Printf("Warning: Error loading .env file: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	dbCtx := context.Background()
	dbpool, err := database.New(dbCtx, cfg.DBConnStr)
	if err != nil {
		logger.Fatalf("failed to connect to database: %v", err)
	}
	defer dbpool.Close()
	logger.Println("Database connection pool established.")

	executorService := code_executor.NewService(cfg.ExecutionTimeout, logger)

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
	}

	serverAddr := ":" + cfg.ServerPort
	logger.Printf("Starting server on %s", serverAddr)
	if err := r.Run(serverAddr); err != nil {
		logger.Fatalf("Failed to run server: %v", err)
	}
}
