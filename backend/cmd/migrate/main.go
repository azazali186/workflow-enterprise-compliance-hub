// Command migrate applies the database schema via GORM AutoMigrate.
package main

import (
	"log/slog"
	"os"

	"github.com/aeroxe/compliance-hub/backend/internal/config"
	"github.com/aeroxe/compliance-hub/backend/internal/database"
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

	slog.Info("database migrations applied")
}
