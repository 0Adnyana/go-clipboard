package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	defaultPort                 = 8080
	defaultMigrationsDir        = "db/migrations"
	defaultSweepInterval        = time.Minute
	defaultTrustedProxy         = "127.0.0.1"
	defaultIPv6Prefix           = 64
	defaultRateLimitCreateRate  = 10
	defaultRateLimitCreateWindow = time.Minute
	defaultRateLimitAvailRate   = 30
	defaultRateLimitAvailWindow = time.Minute
	defaultRateLimitMaxKeys     = 10000

	envDatabaseURL            = "DATABASE_URL"
	envPort                   = "PORT"
	envMigrationsDir          = "MIGRATIONS_DIR"
	envSweepInterval          = "SWEEP_INTERVAL"
	envTrustedProxy           = "TRUSTED_PROXY"
	envIPv6Prefix             = "IPV6_PREFIX"
	envRateLimitCreateRate    = "RATELIMIT_CREATE_RATE"
	envRateLimitCreateWindow  = "RATELIMIT_CREATE_WINDOW"
	envRateLimitAvailRate     = "RATELIMIT_AVAIL_RATE"
	envRateLimitAvailWindow   = "RATELIMIT_AVAIL_WINDOW"
	envRateLimitMaxKeys       = "RATELIMIT_MAX_KEYS"
)

// RateLimitConfig holds tunables for the anonymous IP-keyed limiters.
type RateLimitConfig struct {
	CreateRate   int
	CreateWindow time.Duration
	AvailRate    int
	AvailWindow  time.Duration
	MaxKeys      int
}

type Config struct {
	DatabaseURL   string
	Port          int
	MigrationsDir string
	SweepInterval time.Duration
	TrustedProxy  string
	IPv6Prefix    int
	RateLimit     RateLimitConfig
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

	trustedProxy := defaultTrustedProxy
	if v := os.Getenv(envTrustedProxy); v != "" {
		if net.ParseIP(v) == nil {
			return Config{}, fmt.Errorf("configuration error: %s must be a valid IP address, got %q", envTrustedProxy, v)
		}
		trustedProxy = v
	}

	ipv6Prefix := defaultIPv6Prefix
	if v := os.Getenv(envIPv6Prefix); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 0 || parsed > 128 {
			return Config{}, fmt.Errorf("configuration error: %s must be an integer between 0 and 128, got %q", envIPv6Prefix, v)
		}
		ipv6Prefix = parsed
	}

	rateLimit, err := loadRateLimitConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		DatabaseURL:   databaseURL,
		Port:          port,
		MigrationsDir: migrationsDir,
		SweepInterval: sweepInterval,
		TrustedProxy:  trustedProxy,
		IPv6Prefix:    ipv6Prefix,
		RateLimit:     rateLimit,
	}, nil
}

func loadRateLimitConfig() (RateLimitConfig, error) {
	cfg := RateLimitConfig{
		CreateRate:   defaultRateLimitCreateRate,
		CreateWindow: defaultRateLimitCreateWindow,
		AvailRate:    defaultRateLimitAvailRate,
		AvailWindow:  defaultRateLimitAvailWindow,
		MaxKeys:      defaultRateLimitMaxKeys,
	}

	if v := os.Getenv(envRateLimitCreateRate); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 1 {
			return RateLimitConfig{}, fmt.Errorf("configuration error: %s must be a positive integer, got %q", envRateLimitCreateRate, v)
		}
		cfg.CreateRate = parsed
	}

	if v := os.Getenv(envRateLimitCreateWindow); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil || parsed <= 0 {
			return RateLimitConfig{}, fmt.Errorf("configuration error: %s must be a positive duration, got %q", envRateLimitCreateWindow, v)
		}
		cfg.CreateWindow = parsed
	}

	if v := os.Getenv(envRateLimitAvailRate); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 1 {
			return RateLimitConfig{}, fmt.Errorf("configuration error: %s must be a positive integer, got %q", envRateLimitAvailRate, v)
		}
		cfg.AvailRate = parsed
	}

	if v := os.Getenv(envRateLimitAvailWindow); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil || parsed <= 0 {
			return RateLimitConfig{}, fmt.Errorf("configuration error: %s must be a positive duration, got %q", envRateLimitAvailWindow, v)
		}
		cfg.AvailWindow = parsed
	}

	if v := os.Getenv(envRateLimitMaxKeys); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 1 {
			return RateLimitConfig{}, fmt.Errorf("configuration error: %s must be a positive integer, got %q", envRateLimitMaxKeys, v)
		}
		cfg.MaxKeys = parsed
	}

	return cfg, nil
}
