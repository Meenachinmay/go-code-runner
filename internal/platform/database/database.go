package database

import (
	"context"
	"database/sql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
	goosev3 "github.com/pressly/goose/v3"
	"log"
	"path/filepath"
)

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

func Migrate(ctx context.Context, pool *pgxpool.Pool, dir string, logger *log.Logger) error {
	var db *sql.DB = stdlib.OpenDB(*pool.Config().ConnConfig)
	defer db.Close()

	goosev3.SetBaseFS(nil)
	goosev3.SetLogger(logger)

	if err := goosev3.SetDialect("postgres"); err != nil {
		return err
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	if err := goosev3.UpContext(ctx, db, absDir); err != nil {
		return err
	}
	return nil
}
