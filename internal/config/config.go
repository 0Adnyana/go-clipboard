package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultPort          = 8080
	defaultMigrationsDir = "db/migrations"
	defaultSweepInterval = time.Minute
	envDatabaseURL       = "DATABASE_URL"
	envPort              = "PORT"
	envMigrationsDir     = "MIGRATIONS_DIR"
	envSweepInterval     = "SWEEP_INTERVAL"
)

type Config struct {
	DatabaseURL   string
	Port          int
	MigrationsDir string
	SweepInterval time.Duration
}

func LoadFromEnv() (Config, error) {
	databaseURL := os.Getenv(envDatabaseURL)
	if databaseURL == "" {
		return Config{}, fmt.Errorf("configuration error: %s is required but unset or empty", envDatabaseURL)
	}

	port := defaultPort
	if portStr := os.Getenv(envPort); portStr != "" {
		parsed, err := strconv.Atoi(portStr)
		if err != nil || parsed < 1 || parsed > 65535 {
			return Config{}, fmt.Errorf("configuration error: %s must be a valid port number, got %q", envPort, portStr)
		}
		port = parsed
	}

	migrationsDir := defaultMigrationsDir
	if dir := os.Getenv(envMigrationsDir); dir != "" {
		migrationsDir = dir
	}

	sweepInterval := defaultSweepInterval
	if sweepStr := os.Getenv(envSweepInterval); sweepStr != "" {
		parsed, err := time.ParseDuration(sweepStr)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("configuration error: %s must be a positive duration, got %q", envSweepInterval, sweepStr)
		}
		sweepInterval = parsed
	}

	return Config{
		DatabaseURL:   databaseURL,
		Port:          port,
		MigrationsDir: migrationsDir,
		SweepInterval: sweepInterval,
	}, nil
}
