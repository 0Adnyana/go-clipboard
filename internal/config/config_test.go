package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadFromEnv_defaults(t *testing.T) {
	t.Setenv(envDatabaseURL, "postgres://localhost/test")
	t.Setenv(envPort, "")
	t.Setenv(envMigrationsDir, "")
	t.Setenv(envSweepInterval, "")

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
	if cfg.SweepInterval != defaultSweepInterval {
		t.Errorf("SweepInterval = %v, want %v", cfg.SweepInterval, defaultSweepInterval)
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

func TestLoadFromEnv_sweepInterval(t *testing.T) {
	t.Setenv(envDatabaseURL, "postgres://localhost/test")
	t.Setenv(envSweepInterval, "30s")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.SweepInterval != 30*time.Second {
		t.Errorf("SweepInterval = %v, want 30s", cfg.SweepInterval)
	}
}

func TestLoadFromEnv_malformedSweepInterval(t *testing.T) {
	t.Setenv(envDatabaseURL, "postgres://localhost/test")
	t.Setenv(envSweepInterval, "nope")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() expected error, got nil")
	}
	if !strings.Contains(err.Error(), envSweepInterval) {
		t.Errorf("error = %q, want it to name %s", err.Error(), envSweepInterval)
	}
}
