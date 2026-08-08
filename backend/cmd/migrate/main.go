// Command migrate applies the versioned GORM schema migrations.
//
// Usage:
//
//	migrate            # alias for "up"
//	migrate up         # apply pending migrations + AutoMigrate drift pass
//	migrate status     # list applied migrations
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/aeroxe/compliance-hub/backend/internal/config"
	"github.com/aeroxe/compliance-hub/backend/internal/database"
)

func main() {
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

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

	switch cmd {
	case "up":
		if err := database.Migrate(db); err != nil {
			slog.Error("migrate", "error", err)
			os.Exit(1)
		}
		slog.Info("database migrations applied")
	case "status":
		rows, err := database.AppliedMigrations(context.Background(), db)
		if err != nil {
			slog.Error("status", "error", err)
			os.Exit(1)
		}
		for _, r := range rows {
			fmt.Printf("%s\t%s\t%s\n", r.Version, r.Name, r.AppliedAt.Format("2006-01-02 15:04:05"))
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q (want up|status)\n", cmd)
		os.Exit(2)
	}
}
