package config

import (
	"strings"
	"testing"
)

func TestLoadFromEnv_defaults(t *testing.T) {
	t.Setenv(envDatabaseURL, "postgres://localhost/test")
	t.Setenv(envPort, "")
	t.Setenv(envMigrationsDir, "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.Port != defaultPort {
		t.Errorf("Port = %d, want %d", cfg.Port, defaultPort)
	}
	if cfg.MigrationsDir != defaultMigrationsDir {
		t.Errorf("MigrationsDir = %q, want %q", cfg.MigrationsDir, defaultMigrationsDir)
	}
}

func TestLoadFromEnv_override(t *testing.T) {
	t.Setenv(envDatabaseURL, "postgres://localhost/test")
	t.Setenv(envPort, "9090")
	t.Setenv(envMigrationsDir, "custom/migrations")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.MigrationsDir != "custom/migrations" {
		t.Errorf("MigrationsDir = %q, want custom/migrations", cfg.MigrationsDir)
	}
}

func TestLoadFromEnv_missingDatabaseURL(t *testing.T) {
	t.Setenv(envDatabaseURL, "")
	t.Setenv(envPort, "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() expected error, got nil")
	}
	if !strings.Contains(err.Error(), envDatabaseURL) {
		t.Errorf("error = %q, want it to name %s", err.Error(), envDatabaseURL)
	}
}

func TestLoadFromEnv_malformedPort(t *testing.T) {
	t.Setenv(envDatabaseURL, "postgres://localhost/test")
	t.Setenv(envPort, "not-a-port")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() expected error, got nil")
	}
	if !strings.Contains(err.Error(), envPort) {
		t.Errorf("error = %q, want it to name %s", err.Error(), envPort)
	}
}
