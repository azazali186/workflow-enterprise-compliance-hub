// Command server runs the ComplianceHub HTTP API, WebSocket hub and event bus.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aeroxe/compliance-hub/backend/internal/bus"
	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/config"
	"github.com/aeroxe/compliance-hub/backend/internal/crypto"
	"github.com/aeroxe/compliance-hub/backend/internal/database"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/lock"
	"github.com/aeroxe/compliance-hub/backend/internal/permissions"
	"github.com/aeroxe/compliance-hub/backend/internal/rbac"
	"github.com/aeroxe/compliance-hub/backend/internal/reencrypt"
	sagacore "github.com/aeroxe/compliance-hub/backend/internal/saga"
	"github.com/aeroxe/compliance-hub/backend/internal/server"
	"github.com/aeroxe/compliance-hub/backend/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	// Install the AES-256-GCM key ring (models.Secret, outbox payloads). The
	// current ENCRYPTION_KEY encrypts new data; ENCRYPTION_KEY_PREVIOUS
	// entries stay readable (dual-read) until the re-encryption worker has
	// migrated everything to the current key.
	if err := crypto.Setup(cfg.EncryptionKey, cfg.PreviousEncryptionKeys...); err != nil {
		slog.Error("encryption key", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(cfg.DatabaseURL, cfg.LogLevel)
	if err != nil {
		logger.Error("database", "error", err)
		os.Exit(1)
	}
	if err := database.Migrate(db); err != nil {
		logger.Error("migrate", "error", err)
		os.Exit(1)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	c := cache.New(pingCtx, cfg.RedisURL)
	cancel()

	b := bus.New(ctx, cfg.NATSURL)

	hub := ws.NewHub(cfg.WSMaxConnections, cfg.WSPingInterval)
	if _, err := b.Subscribe(">", hub.Broadcast); err != nil {
		logger.Warn("failed to forward bus to websocket hub", "error", err)
	}

	// The saga engine (README Saga Orchestrator) subscribes to its driving
	// event subjects and advances the four business sagas in Redis state.
	sagaEngine := sagacore.New(db, c, b, logger)

	d := deps.Deps{DB: db, Bus: b, Cache: c, Hub: hub, Logger: logger, Saga: sagaEngine}

	srv := server.New(cfg, d)

	// Startup seeding (AUTO_SYNC_PERMISSIONS, default on): extract every
	// registered route into the permission table + api-gateway-permissions
	// cache key, then build the RBAC roles and bootstrap admin from it. Both
	// steps are idempotent. In production a failed seed means an unusable
	// (or unauthenticated) API, so it is fatal outside development; in dev it
	// degrades to a warning so the server can still boot for exploration.
	if cfg.SyncPermissionsOnStart {
		if _, err := permissions.Generate(srv, d, "routes.json"); err != nil {
			if cfg.Env != "development" {
				logger.Error("permissions sync failed — aborting boot", "error", err)
				os.Exit(1)
			}
			logger.Warn("permissions sync on start failed", "error", err)
		}
	}
	if err := rbac.Seed(ctx, d, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		if cfg.Env != "development" {
			logger.Error("rbac seed failed — aborting boot", "error", err)
			os.Exit(1)
		}
		logger.Warn("rbac seed failed", "error", err)
	}

	// Key-rotation migration: re-encrypt any outbox payloads (and future
	// Secret columns) still carrying the previous key so it can eventually be
	// retired. Idempotent and dual-read-safe — a failure only delays the
	// cleanup, never breaks reads.
	if cfg.AutoReencrypt {
		m := reencrypt.New(db)
		if n, err := m.Run(ctx, lock.New(ctx, cfg.RedisURL)); err != nil {
			logger.Warn("re-encryption migration failed", "error", err)
		} else if n > 0 {
			logger.Info("re-encrypted rows to the current key", "count", n, "key_id", crypto.CurrentKeyID())
		}
	}

	server.Run(ctx, cfg, d, srv)
}

// newLogger builds the structured logger: JSON in production-friendly
// environments, text locally. Log lines carry only request metadata (method,
// path, status, latency, IP, request_id) — never bodies, headers or tokens.
func newLogger(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: logLevel(cfg.LogLevel)}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func logLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
