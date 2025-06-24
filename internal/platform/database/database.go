package database

import (
	"context"
	"database/sql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq" // Goose needs the driver registered
	"github.com/pressly/goose/v3"
	"log"
	"path/filepath"
)

// New creates a new database connection pool
func New(ctx context.Context, connStr string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return pool, nil
}

// Migrate runs all pending Goose migrations found in dir.
// It is safe to call on every application start-up.
func Migrate(ctx context.Context, pool *pgxpool.Pool, dir string, logger *log.Logger) error {
	// Wrap pgx pool in *sql.DB so Goose can use it.
	var db *sql.DB = stdlib.OpenDB(*pool.Config().ConnConfig)
	defer db.Close()

	goose.SetBaseFS(nil) // we read from the filesystem, not embed.FS
	goose.SetLogger(logger)
	goose.SetDialect("postgres")

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	if err := goose.UpContext(ctx, db, absDir); err != nil {
		return err
	}
	return nil
}
