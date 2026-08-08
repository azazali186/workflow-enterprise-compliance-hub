// Package config loads and validates application configuration from
// environment variables (optionally seeded from a .env file via godotenv).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for the ComplianceHub backend.
type Config struct {
	ServerAddr             string
	Env                    string
	LogLevel               string
	LogFormat              string
	DatabaseURL            string
	RedisURL               string
	NATSURL                string
	JWTSecret              string
	JWTExpiry              time.Duration
	EncryptionKey          string   // AES-256-GCM at-rest key (hex/base64/raw 32B)
	PreviousEncryptionKeys []string // keys still readable during rotation (dual-read)
	AutoReencrypt          bool     // migrate old-key rows to the current key on boot
	CORSAllowedOrigins     []string
	WSMaxConnections       int
	WSPingInterval         time.Duration
	RateLimitPerMinute     int
	DeadlineJobInterval    time.Duration
	MaxBodySize            int
	OutboxPollInterval     time.Duration
	AuditRetentionDays     int
	AuditRetentionInterval time.Duration
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
	retentionDays, err := parseInt("AUDIT_RETENTION_DAYS", 365)
	if err != nil {
		return nil, err
	}
	retentionInterval, err := parseDuration("AUDIT_RETENTION_INTERVAL", 24*time.Hour)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		ServerAddr:             ":" + port,
		Env:                    getEnv("ENV", "development"),
		LogLevel:               getEnv("LOG_LEVEL", "debug"),
		LogFormat:              getEnv("LOG_FORMAT", "text"),
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/compliance-hub?sslmode=disable"),
		RedisURL:               getEnv("REDIS_URL", "redis://localhost:6379"),
		NATSURL:                getEnv("NATS_URL", "nats://localhost:4222"),
		JWTSecret:              getEnv("JWT_SECRET", "dev-only-secret"),
		JWTExpiry:              jwtExpiry,
		EncryptionKey:          getEnv("ENCRYPTION_KEY", ""),
		PreviousEncryptionKeys: splitList(os.Getenv("ENCRYPTION_KEY_PREVIOUS")),
		AutoReencrypt:          os.Getenv("AUTO_REENCRYPT") != "false",
		CORSAllowedOrigins:     splitList(os.Getenv("CORS_ALLOWED_ORIGINS")),
		WSMaxConnections:       wsMax,
		WSPingInterval:         wsPing,
		RateLimitPerMinute:     rateLimit,
		DeadlineJobInterval:    deadlineInterval,
		MaxBodySize:            maxBodySize,
		OutboxPollInterval:     outboxInterval,
		AuditRetentionDays:     retentionDays,
		AuditRetentionInterval: retentionInterval,
		AdminUsername:          getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword:          getEnv("ADMIN_PASSWORD", "admin123"),
		SyncPermissionsOnStart: os.Getenv("AUTO_SYNC_PERMISSIONS") != "false",
	}

	// Forgeable tokens must be impossible outside development: refuse to boot
	// unless a real secret was supplied.
	if cfg.Env != "development" && (cfg.JWTSecret == "" || cfg.JWTSecret == "dev-only-secret") {
		return nil, fmt.Errorf("JWT_SECRET must be set to a non-default value when ENV != development")
	}

	// The at-rest encryption key protects outbox payloads and Secret columns.
	// Outside development it must be a real, configured key — never the
	// dev-only fallback.
	if cfg.Env != "development" && cfg.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be set (32 bytes, hex or base64) when ENV != development")
	}

	return cfg, nil
}

// splitList splits a comma-separated env value into trimmed, non-empty items.
func splitList(v string) []string {
	if v == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
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
