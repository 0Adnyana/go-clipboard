package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	defaultPort          = 8080
	defaultMigrationsDir = "db/migrations"
	envDatabaseURL       = "DATABASE_URL"
	envPort              = "PORT"
	envMigrationsDir     = "MIGRATIONS_DIR"
)

type Config struct {
	DatabaseURL   string
	Port          int
	MigrationsDir string
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

	return Config{
		DatabaseURL:   databaseURL,
		Port:          port,
		MigrationsDir: migrationsDir,
	}, nil
}
