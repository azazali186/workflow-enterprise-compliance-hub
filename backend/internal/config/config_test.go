package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	clearEnv()
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ServerAddr != ":8080" {
		t.Errorf("ServerAddr = %q, want %q", cfg.ServerAddr, ":8080")
	}
	if cfg.Env != "development" {
		t.Errorf("Env = %q, want development", cfg.Env)
	}
	if cfg.WSMaxConnections != 1000 {
		t.Errorf("WSMaxConnections = %d, want 1000", cfg.WSMaxConnections)
	}
	if cfg.WSPingInterval != 30*time.Second {
		t.Errorf("WSPingInterval = %v, want 30s", cfg.WSPingInterval)
	}
	if cfg.RateLimitPerMinute != 60 {
		t.Errorf("RateLimitPerMinute = %d, want 60", cfg.RateLimitPerMinute)
	}
	if cfg.DeadlineJobInterval != time.Minute {
		t.Errorf("DeadlineJobInterval = %v, want 1m", cfg.DeadlineJobInterval)
	}
}

func TestLoadOverrides(t *testing.T) {
	clearEnv()
	t.Setenv("PORT", "9090")
	t.Setenv("WS_PING_INTERVAL", "15s")
	t.Setenv("RATE_LIMIT_PER_MINUTE", "10")
	t.Setenv("DEADLINE_JOB_INTERVAL", "5m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ServerAddr != ":9090" {
		t.Errorf("ServerAddr = %q, want :9090", cfg.ServerAddr)
	}
	if cfg.WSPingInterval != 15*time.Second {
		t.Errorf("WSPingInterval = %v, want 15s", cfg.WSPingInterval)
	}
	if cfg.RateLimitPerMinute != 10 {
		t.Errorf("RateLimitPerMinute = %d, want 10", cfg.RateLimitPerMinute)
	}
	if cfg.DeadlineJobInterval != 5*time.Minute {
		t.Errorf("DeadlineJobInterval = %v, want 5m", cfg.DeadlineJobInterval)
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	clearEnv()
	t.Setenv("WS_PING_INTERVAL", "not-a-duration")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error for invalid WS_PING_INTERVAL")
	}
}

func clearEnv() {
	for _, k := range []string{"PORT", "ENV", "LOG_LEVEL", "DATABASE_URL", "REDIS_URL", "NATS_URL", "JWT_SECRET", "JWT_EXPIRY", "WS_MAX_CONNECTIONS", "WS_PING_INTERVAL", "RATE_LIMIT_PER_MINUTE", "DEADLINE_JOB_INTERVAL"} {
		_ = os.Unsetenv(k)
	}
}
