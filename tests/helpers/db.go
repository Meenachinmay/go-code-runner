package helpers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
	_ "github.com/pressly/goose/v3"

	"go-code-runner/internal/platform/database"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	if err := loadTestDotEnv(); err != nil {
		t.Fatalf("cannot load .env.test: %v", err)
	}

	dbUser := getenvDefault("POSTGRES_USER", "")
	dbPass := getenvDefault("POSTGRES_PASSWORD", "")
	dbHost := getenvDefault("POSTGRES_HOST", "")
	dbPort := getenvDefault("POSTGRES_PORT", "")
	testDB := getenvDefault("POSTGRES_TEST_DB", "")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, testDB)

	pool, err := database.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	logger := log.New(os.Stdout, "TEST-MIGRATE: ", log.LstdFlags|log.Lmicroseconds)
	migrationsDir, err := findMigrationsDir()

	if err := database.Migrate(context.Background(), pool, migrationsDir, logger); err != nil {
		pool.Close()
		t.Fatalf("migrate test db: %v", err)
	}

	// Sample data loading removed to avoid duplicate key errors

	cleanup := func() {
		pool.Close()
	}

	return pool, cleanup
}

func getenvDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

// isDuplicateKey function removed as it's no longer needed

func loadTestDotEnv() error {
	const file = ".env.test"

	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	for {
		envPath := filepath.Join(dir, file)
		if _, statErr := os.Stat(envPath); statErr == nil {
			// Found it → load and return
			return godotenv.Load(envPath)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return errors.New(".env.test not found in working directory or any parent directory")
}

func findMigrationsDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(dir, "db", "migrations")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", errors.New("migrations directory not found in working directory or any parent directory")
}

// findSampleDataFile function removed as it's no longer needed
