package server

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"go-code-runner/internal/repository"
	"go-code-runner/internal/service/coding_test"
	"go-code-runner/internal/service/problems"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-code-runner/internal/code_executor"
	"go-code-runner/internal/config"
	"go-code-runner/internal/middleware"
	"go-code-runner/internal/platform/database"

	executorpb "go-code-runner/go-code-runner-microservice/proto/executor/v1"
	problemspb "go-code-runner/go-code-runner-microservice/proto/problems/v1"
	grpcserver "go-code-runner/internal/grpc"
	grpcproblems "go-code-runner/internal/service/grpc"
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
		PoolSize:     1000,
		MinIdleConns: 200,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	})

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatalf("failed to connect to redis: %v", err)
	}
	defer redisClient.Close()
	logger.Println("redis connection established")

	repo := repository.New(dbpool)

	executorConfig := code_executor.Config{
		WorkerCount:         cfg.ExecutorWorkerCount,
		MaxQueueSize:        cfg.ExecutorMaxQueueSize,
		ExecutionTimeout:    cfg.ExecutionTimeout,
		ResultTTL:           cfg.ExecutorResultTTL,
		EnableContainerPool: cfg.EnableContainerPool,
		ContainerPoolSize:   cfg.ContainerPoolSize,
	}

	executorService := code_executor.NewService(executorConfig, logger, repo, redisClient)

	problemService := problems.New(repo)
	_ = coding_test.New(repo, repo, repo, "http://localhost:8080")

	middleware.InitAPIKeyAuth(dbpool)

	// grpc server
	grpcAddr := cfg.GRPCPort
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Fatalf("failed to listen on %s: %v", grpcAddr, err)
	}

	grpcServer := grpc.NewServer()

	executorServer := grpcserver.NewServer(executorService, logger)
	executorpb.RegisterExecutorServiceServer(grpcServer, executorServer)

	problemServer := grpcproblems.NewProblemServer(problemService, logger)
	problemspb.RegisterProblemServiceServer(grpcServer, problemServer)

	reflection.Register(grpcServer)

	go func() {
		logger.Printf("Staerting gRPC server on %s", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatalf("failed to serve gRPC server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Println("Shutdown signal received, initiating graceful shutdown...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Println("Shutting down gRPC server...")
	grpcServer.GracefulStop()

	logger.Println("Shutting down executor service...")
	executorService.Shutdown()
}
