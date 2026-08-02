package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort                  = 8080
	defaultMigrationsDir         = "db/migrations"
	defaultSweepInterval         = time.Minute
	defaultTrustedProxy          = "127.0.0.1"
	defaultIPv6Prefix            = 64
	defaultRateLimitCreateRate   = 10
	defaultRateLimitCreateWindow = time.Minute
	defaultRateLimitAvailRate    = 30
	defaultRateLimitAvailWindow  = time.Minute
	defaultRateLimitMaxKeys      = 10000

	envDatabaseURL           = "DATABASE_URL"
	envPort                  = "PORT"
	envMigrationsDir         = "MIGRATIONS_DIR"
	envSweepInterval         = "SWEEP_INTERVAL"
	envTrustedProxy          = "TRUSTED_PROXY"
	envIPv6Prefix            = "IPV6_PREFIX"
	envRateLimitCreateRate   = "RATELIMIT_CREATE_RATE"
	envRateLimitCreateWindow = "RATELIMIT_CREATE_WINDOW"
	envRateLimitAvailRate    = "RATELIMIT_AVAIL_RATE"
	envRateLimitAvailWindow  = "RATELIMIT_AVAIL_WINDOW"
	envRateLimitMaxKeys      = "RATELIMIT_MAX_KEYS"
	envPublicBaseURL         = "PUBLIC_BASE_URL"
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

	// PublicBaseURL is the absolute HTTPS origin of the deployment (no path,
	// query, fragment, or userinfo). Empty in local development. In production
	// this is the edge-facing origin; the process itself serves plain HTTP.
	PublicBaseURL string
	// TLSHostname is derived from PublicBaseURL; empty when PublicBaseURL is unset.
	TLSHostname string
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

	trustedProxy, err := loadTrustedProxy()
	if err != nil {
		return Config{}, err
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

	publicBaseURL, tlsHostname, err := loadPublicBaseURL()
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
		PublicBaseURL: publicBaseURL,
		TLSHostname:   tlsHostname,
	}, nil
}

// loadTrustedProxy distinguishes unset (dev default 127.0.0.1) from explicitly
// empty (no forwarded hop trusted). Production behind an edge proxy sets the
// edge's IP; only that peer's X-Forwarded-For is honoured.
func loadTrustedProxy() (string, error) {
	v, ok := os.LookupEnv(envTrustedProxy)
	if !ok {
		return defaultTrustedProxy, nil
	}
	if v == "" {
		return "", nil
	}
	if net.ParseIP(v) == nil {
		return "", fmt.Errorf("configuration error: %s must be a valid IP address, got %q", envTrustedProxy, v)
	}
	return v, nil
}

func loadPublicBaseURL() (publicBaseURL, hostname string, err error) {
	raw := strings.TrimSpace(os.Getenv(envPublicBaseURL))
	if raw == "" {
		return "", "", nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("configuration error: %s is not a valid URL: %w", envPublicBaseURL, err)
	}
	if u.Scheme != "https" {
		return "", "", fmt.Errorf("configuration error: %s must use the https scheme, got %q", envPublicBaseURL, u.Scheme)
	}
	if u.Host == "" {
		return "", "", fmt.Errorf("configuration error: %s must include a host", envPublicBaseURL)
	}
	hostname = u.Hostname()
	if hostname == "" {
		return "", "", fmt.Errorf("configuration error: %s must include a host", envPublicBaseURL)
	}
	if u.User != nil {
		return "", "", fmt.Errorf("configuration error: %s must not include userinfo", envPublicBaseURL)
	}
	if u.Path != "" && u.Path != "/" {
		return "", "", fmt.Errorf("configuration error: %s must not include a path, got %q", envPublicBaseURL, u.Path)
	}
	if u.RawQuery != "" {
		return "", "", fmt.Errorf("configuration error: %s must not include a query string", envPublicBaseURL)
	}
	if u.Fragment != "" {
		return "", "", fmt.Errorf("configuration error: %s must not include a fragment", envPublicBaseURL)
	}
	origin := u.Scheme + "://" + u.Host
	if strings.TrimRight(raw, "/") != origin {
		return "", "", fmt.Errorf("configuration error: %s must be exactly one absolute HTTPS origin, got %q", envPublicBaseURL, raw)
	}
	return origin, hostname, nil
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
