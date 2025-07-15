package server

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"go-code-runner/internal/repository"
	"go-code-runner/internal/service/coding_test"
	"go-code-runner/internal/service/problems"
	"log"
	"os"
	"time"

	"go-code-runner/internal/code_executor"
	"go-code-runner/internal/config"
	"go-code-runner/internal/handler"
	"go-code-runner/internal/middleware"
	"go-code-runner/internal/platform/database"
	"go-code-runner/internal/service/company"
)

func Run() {

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

	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           0,
		PoolSize:     300,
		MinIdleConns: 50,
		MaxRetries:   3,
		DialTimeout: 5 * time.Second,
		ReadTimeout: 1 * time.Second,
		WriteTimeout: 1 * time.Second,
	})

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatalf("failed to connect to redis: %v", err)
	}
	defer redisClient.Close()
	logger.Println("redis connection established")

	repo := repository.New(dbpool)

	executorConfig := code_executor.Config{
		WorkerCount:      cfg.ExecutorWorkerCount,
		MaxQueueSize:     cfg.ExecutorMaxQueueSize,
		ExecutionTimeout: cfg.ExecutionTimeout,
		ResultTTL:        cfg.ExecutorResultTTL,
		EnableContainerPool: cfg.EnableContainerPool,
		ContainerPoolSize:  cfg.ContainerPoolSize,
	}

	executorService := code_executor.NewService(executorConfig, logger, repo, redisClient)

	companyService := company.New(repo)
	companyHandler := handler.NewCompanyHandler(companyService)
	problemService := problems.New(repo)
	codingTestService := coding_test.New(repo, repo, repo, "http://localhost:8080")
	codingTestHandler := handler.NewCodingTestHandler(codingTestService)

	middleware.InitAPIKeyAuth(dbpool)

	r := NewRouter(dbpool, problemService, executorService, companyHandler, codingTestHandler)

	addr := ":" + cfg.ServerPort
	logger.Printf("starting HTTP server on %s", addr)
	if err := r.Run(addr); err != nil {
		logger.Fatalf("server error: %v", err)
	}
}
