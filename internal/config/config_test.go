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

func TestLoadFromEnv_rateLimitDefaults(t *testing.T) {
	t.Setenv(envDatabaseURL, "postgres://localhost/test")
	clearRateLimitEnv(t)

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.TrustedProxy != defaultTrustedProxy {
		t.Errorf("TrustedProxy = %q, want %q", cfg.TrustedProxy, defaultTrustedProxy)
	}
	if cfg.IPv6Prefix != defaultIPv6Prefix {
		t.Errorf("IPv6Prefix = %d, want %d", cfg.IPv6Prefix, defaultIPv6Prefix)
	}
	if cfg.RateLimit.CreateRate != defaultRateLimitCreateRate {
		t.Errorf("CreateRate = %d, want %d", cfg.RateLimit.CreateRate, defaultRateLimitCreateRate)
	}
	if cfg.RateLimit.CreateWindow != defaultRateLimitCreateWindow {
		t.Errorf("CreateWindow = %v, want %v", cfg.RateLimit.CreateWindow, defaultRateLimitCreateWindow)
	}
	if cfg.RateLimit.AvailRate != defaultRateLimitAvailRate {
		t.Errorf("AvailRate = %d, want %d", cfg.RateLimit.AvailRate, defaultRateLimitAvailRate)
	}
	if cfg.RateLimit.MaxKeys != defaultRateLimitMaxKeys {
		t.Errorf("MaxKeys = %d, want %d", cfg.RateLimit.MaxKeys, defaultRateLimitMaxKeys)
	}
}

func TestLoadFromEnv_rateLimitOverride(t *testing.T) {
	t.Setenv(envDatabaseURL, "postgres://localhost/test")
	clearRateLimitEnv(t)
	t.Setenv(envTrustedProxy, "10.0.0.1")
	t.Setenv(envIPv6Prefix, "48")
	t.Setenv(envRateLimitCreateRate, "5")
	t.Setenv(envRateLimitCreateWindow, "30s")
	t.Setenv(envRateLimitAvailRate, "20")
	t.Setenv(envRateLimitAvailWindow, "2m")
	t.Setenv(envRateLimitMaxKeys, "500")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.TrustedProxy != "10.0.0.1" {
		t.Errorf("TrustedProxy = %q, want 10.0.0.1", cfg.TrustedProxy)
	}
	if cfg.IPv6Prefix != 48 {
		t.Errorf("IPv6Prefix = %d, want 48", cfg.IPv6Prefix)
	}
	if cfg.RateLimit.CreateRate != 5 {
		t.Errorf("CreateRate = %d, want 5", cfg.RateLimit.CreateRate)
	}
	if cfg.RateLimit.CreateWindow != 30*time.Second {
		t.Errorf("CreateWindow = %v, want 30s", cfg.RateLimit.CreateWindow)
	}
	if cfg.RateLimit.AvailRate != 20 {
		t.Errorf("AvailRate = %d, want 20", cfg.RateLimit.AvailRate)
	}
	if cfg.RateLimit.AvailWindow != 2*time.Minute {
		t.Errorf("AvailWindow = %v, want 2m", cfg.RateLimit.AvailWindow)
	}
	if cfg.RateLimit.MaxKeys != 500 {
		t.Errorf("MaxKeys = %d, want 500", cfg.RateLimit.MaxKeys)
	}
}

func TestLoadFromEnv_malformedRateLimit(t *testing.T) {
	t.Setenv(envDatabaseURL, "postgres://localhost/test")
	clearRateLimitEnv(t)

	cases := []struct {
		name string
		key  string
		val  string
	}{
		{"create rate", envRateLimitCreateRate, "abc"},
		{"create window", envRateLimitCreateWindow, "nope"},
		{"avail rate", envRateLimitAvailRate, "0"},
		{"max keys", envRateLimitMaxKeys, "-1"},
		{"ipv6 prefix", envIPv6Prefix, "999"},
		{"trusted proxy", envTrustedProxy, "not-an-ip"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearRateLimitEnv(t)
			t.Setenv(tc.key, tc.val)
			_, err := LoadFromEnv()
			if err == nil {
				t.Fatal("LoadFromEnv() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Errorf("error = %q, want it to name %s", err.Error(), tc.key)
			}
		})
	}
}

func clearRateLimitEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		envTrustedProxy, envIPv6Prefix,
		envRateLimitCreateRate, envRateLimitCreateWindow,
		envRateLimitAvailRate, envRateLimitAvailWindow,
		envRateLimitMaxKeys,
	} {
		t.Setenv(key, "")
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
