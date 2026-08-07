// Package config loads and validates application configuration from
// environment variables (optionally seeded from a .env file via godotenv).
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for the ComplianceHub backend.
type Config struct {
	ServerAddr             string
	Env                    string
	LogLevel               string
	DatabaseURL            string
	RedisURL               string
	NATSURL                string
	JWTSecret              string
	JWTExpiry              time.Duration
	WSMaxConnections       int
	WSPingInterval         time.Duration
	RateLimitPerMinute     int
	DeadlineJobInterval    time.Duration
	MaxBodySize            int
	OutboxPollInterval     time.Duration
	AdminUsername          string
	AdminPassword          string
	SyncPermissionsOnStart bool
}

// Load reads configuration from the environment. A .env file is loaded first
// (if present) so local development matches docker-compose values.
func Load() (*Config, error) {
	_ = godotenv.Load() // ignore missing .env

	port := getEnv("PORT", "8080")
	jwtExpiry, err := parseDuration("JWT_EXPIRY", 24*time.Hour)
	if err != nil {
		return nil, err
	}
	wsPing, err := parseDuration("WS_PING_INTERVAL", 30*time.Second)
	if err != nil {
		return nil, err
	}
	wsMax, err := parseInt("WS_MAX_CONNECTIONS", 1000)
	if err != nil {
		return nil, err
	}
	rateLimit, err := parseInt("RATE_LIMIT_PER_MINUTE", 60)
	if err != nil {
		return nil, err
	}
	deadlineInterval, err := parseDuration("DEADLINE_JOB_INTERVAL", time.Minute)
	if err != nil {
		return nil, err
	}
	outboxInterval, err := parseDuration("OUTBOX_POLL_INTERVAL", time.Second)
	if err != nil {
		return nil, err
	}
	maxBodySize, err := parseInt("MAX_BODY_SIZE", 8<<20)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		ServerAddr:             ":" + port,
		Env:                    getEnv("ENV", "development"),
		LogLevel:               getEnv("LOG_LEVEL", "debug"),
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/compliance-hub?sslmode=disable"),
		RedisURL:               getEnv("REDIS_URL", "redis://localhost:6379"),
		NATSURL:                getEnv("NATS_URL", "nats://localhost:4222"),
		JWTSecret:              getEnv("JWT_SECRET", "dev-only-secret"),
		JWTExpiry:              jwtExpiry,
		WSMaxConnections:       wsMax,
		WSPingInterval:         wsPing,
		RateLimitPerMinute:     rateLimit,
		DeadlineJobInterval:    deadlineInterval,
		MaxBodySize:            maxBodySize,
		OutboxPollInterval:     outboxInterval,
		AdminUsername:          getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword:          getEnv("ADMIN_PASSWORD", "admin123"),
		SyncPermissionsOnStart: os.Getenv("AUTO_SYNC_PERMISSIONS") != "false",
	}

	// Forgeable tokens must be impossible outside development: refuse to boot
	// unless a real secret was supplied.
	if cfg.Env != "development" && (cfg.JWTSecret == "" || cfg.JWTSecret == "dev-only-secret") {
		return nil, fmt.Errorf("JWT_SECRET must be set to a non-default value when ENV != development")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %q", key, v)
	}
	return n, nil
}

func parseDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %q", key, v)
	}
	return d, nil
}
