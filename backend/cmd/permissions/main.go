// Command permissions extracts every registered HTTP route from the server,
// writes routes.json, stores the manifest in the cache (api-gateway-permissions)
// and upserts one Permission row per route in the database.
//
// Run it after changing routes: `make permissions`.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aeroxe/compliance-hub/backend/internal/bus"
	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/config"
	"github.com/aeroxe/compliance-hub/backend/internal/database"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/permissions"
	"github.com/aeroxe/compliance-hub/backend/internal/server"
	"github.com/aeroxe/compliance-hub/backend/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	db, err := database.Connect(cfg.DatabaseURL, cfg.LogLevel)
	if err != nil {
		slog.Error("database", "error", err)
		os.Exit(1)
	}
	if err := database.Migrate(db); err != nil {
		slog.Error("migrate", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	d := deps.Deps{
		DB:     db,
		Bus:    bus.New(ctx, cfg.NATSURL),
		Cache:  cache.New(ctx, cfg.RedisURL),
		Hub:    ws.NewHub(cfg.WSMaxConnections, cfg.WSPingInterval),
		Logger: slog.Default(),
	}

	// Build the server purely to enumerate its registered routes (not started).
	srv := server.New(cfg, d)

	routes, err := permissions.Generate(srv, d, "routes.json")
	if err != nil {
		slog.Error("permissions sync failed", "error", err)
		os.Exit(1)
	}

	slog.Info("permissions synced", "count", len(routes))
}
