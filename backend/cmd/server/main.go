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
	"github.com/aeroxe/compliance-hub/backend/internal/database"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/permissions"
	"github.com/aeroxe/compliance-hub/backend/internal/rbac"
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

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel(cfg.LogLevel)}))
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

	server.Run(ctx, cfg, d, srv)
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
