package server

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"

	"go-code-runner/internal/code_executor"
	"go-code-runner/internal/config"
	_ "go-code-runner/internal/handler"
	"go-code-runner/internal/platform/database"
	"go-code-runner/internal/repository"
)

// Run boot-straps every dependency, starts migrations and launches the HTTP server.
// It is the old main() logic extracted into a reusable unit.
func Run() {
	// -----------------------------------------------------------------
	// 0. logger + env
	// -----------------------------------------------------------------
	logger := log.New(os.Stdout, "CODE-RUNNER: ", log.LstdFlags|log.Lmicroseconds)
	_ = godotenv.Load() // .env is optional

	// -----------------------------------------------------------------
	// 1. configuration
	// -----------------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("failed to load configuration: %v", err)
	}

	// -----------------------------------------------------------------
	// 2. Postgres connection & migrations
	// -----------------------------------------------------------------
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

	// -----------------------------------------------------------------
	// 3. domain services & repositories
	// -----------------------------------------------------------------
	repo := repository.New(dbpool)
	executorService := code_executor.NewService(cfg.ExecutionTimeout, logger, repo)

	// -----------------------------------------------------------------
	// 4. HTTP router + handlers
	// -----------------------------------------------------------------
	r := NewRouter(dbpool, repo, executorService)

	addr := ":" + cfg.ServerPort
	logger.Printf("starting HTTP server on %s", addr)
	if err := r.Run(addr); err != nil {
		logger.Fatalf("server error: %v", err)
	}
}
