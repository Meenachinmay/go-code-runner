package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DBConnStr        string
	ServerPort       string
	ExecutionTimeout time.Duration
}

func Load() (*Config, error) {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	dbUser := os.Getenv("POSTGRES_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}

	dbPassword := os.Getenv("POSTGRES_PASSWORD")
	if dbPassword == "" {
		dbPassword = "password"
	}

	dbName := os.Getenv("POSTGRES_DB")
	if dbName == "" {
		dbName = "code_runner_db"
	}

	dbHost := os.Getenv("POSTGRES_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbPort := os.Getenv("POSTGRES_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbConnStr := "host=" + dbHost + " port=" + dbPort + " user=" + dbUser + " password=" + dbPassword + " dbname=" + dbName + " sslmode=disable"

	timeoutStr := os.Getenv("EXECUTION_TIMEOUT_SECONDS")
	if timeoutStr == "" {
		timeoutStr = "15"
	}
	timeoutSeconds, err := strconv.Atoi(timeoutStr)
	if err != nil {
		return nil, err
	}

	return &Config{
		ServerPort:       port,
		DBConnStr:        dbConnStr,
		ExecutionTimeout: time.Duration(timeoutSeconds) * time.Second,
	}, nil
}
