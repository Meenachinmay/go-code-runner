package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	yaml "gopkg.in/yaml.v3"
)

type rawConfig struct {
	ServerPort              string `yaml:"server_port"`
	ExecutionTimeoutSeconds int    `yaml:"execution_timeout_seconds"`
	Postgres                struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		DB       string `yaml:"db"`
		SSLMode  string `yaml:"sslmode"`
	} `yaml:"postgres"`
}

type Config struct {
	ServerPort string
	DBConnStr  string

	RedisAddr     string
	RedisPassword string

	ExecutionTimeout     time.Duration
	ExecutorWorkerCount  int
	ExecutorMaxQueueSize int
	ExecutorResultTTL    time.Duration

	EnableContainerPool bool
	ContainerPoolSize   int

	HTTPWorkerCount  int
	HTTPMaxQueueSize int

	FrontendURL string
}

func Load() (*Config, error) {
	env := os.Getenv("APP_ENVIRONMENT")
	if env == "" {
		env = "local"
	}

	cfgPath := filepath.Join("internal", "config", env+".yml")
	f, err := os.Open(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", cfgPath, err)
	}
	defer f.Close()

	var raw rawConfig
	if err := yaml.NewDecoder(f).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", cfgPath, err)
	}

	if v := os.Getenv("SERVER_PORT"); v != "" {
		raw.ServerPort = v
	}
	if v := os.Getenv("EXECUTION_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			raw.ExecutionTimeoutSeconds = n
		}
	}

	if v := os.Getenv("POSTGRES_HOST"); v != "" {
		raw.Postgres.Host = v
	}
	if v := os.Getenv("POSTGRES_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			raw.Postgres.Port = n
		}
	}
	if v := os.Getenv("POSTGRES_USER"); v != "" {
		raw.Postgres.User = v
	}
	if v := os.Getenv("POSTGRES_PASSWORD"); v != "" {
		raw.Postgres.Password = v
	}
	if v := os.Getenv("POSTGRES_DB"); v != "" {
		raw.Postgres.DB = v
	}
	if v := os.Getenv("POSTGRES_SSLMODE"); v != "" {
		raw.Postgres.SSLMode = v
	}

	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		raw.Postgres.Host,
		raw.Postgres.Port,
		raw.Postgres.User,
		raw.Postgres.Password,
		raw.Postgres.DB,
		raw.Postgres.SSLMode,
	)

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {

		if env == "local" && os.Getenv("APP_ENVIRONMENT") == "local" {
			redisAddr = "redis:6379"
		} else {
			redisAddr = "localhost:6379"
		}
	}

	return &Config{
		ServerPort:           raw.ServerPort,
		DBConnStr:            connStr,
		ExecutionTimeout:     time.Duration(raw.ExecutionTimeoutSeconds) * time.Second,
		RedisAddr:            redisAddr,
		RedisPassword:        "",
		ExecutorWorkerCount:  500,
		ExecutorMaxQueueSize: 20000,
		ExecutorResultTTL:    15 * time.Minute,
		EnableContainerPool:  true,
		ContainerPoolSize:    50,
		HTTPWorkerCount:      200,
		HTTPMaxQueueSize:     10000,
		FrontendURL:          "http://localhost:8080",
	}, nil
}
