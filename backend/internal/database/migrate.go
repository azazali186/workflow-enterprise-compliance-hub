// Versioned schema migrations on top of GORM. Every migration is a Go
// function (GORM Migrator only — no raw SQL) recorded in the
// schema_migrations table, applied exactly once in version order. AutoMigrate
// remains as the final "drift" pass so brand-new models without an explicit
// migration still get created.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

// Migration is a single forward-only schema change.
type Migration struct {
	Version string // unique, sortable (e.g. "0002_add_user_status")
	Name    string
	Up      func(db *gorm.DB) error
}

// SchemaMigration records an applied migration version.
type SchemaMigration struct {
	Version   string    `gorm:"primaryKey;size:128" json:"version"`
	Name      string    `gorm:"size:255" json:"name"`
	AppliedAt time.Time `json:"applied_at"`
}

// migrations is the ordered list of schema migrations. Append new versions —
// never edit or reorder existing entries.
var migrations = []Migration{
	{
		Version: "0001_baseline",
		Name:    "baseline schema (all current models)",
		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(models.All()...)
		},
	},
}

// Migrate ensures the schema_migrations table exists, applies every pending
// migration in version order (each in a transaction), then runs AutoMigrate as
// a non-destructive drift pass for any model not covered by a migration.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&SchemaMigration{}); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied := map[string]bool{}
	var rows []SchemaMigration
	if err := db.Find(&rows).Error; err != nil {
		return fmt.Errorf("load schema_migrations: %w", err)
	}
	for _, r := range rows {
		applied[r.Version] = true
	}

	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}
		if err := applyMigration(db, m); err != nil {
			return fmt.Errorf("migration %s (%s): %w", m.Version, m.Name, err)
		}
	}

	// Drift pass: create tables for models added without a versioned
	// migration (additive only; AutoMigrate never drops columns).
	if err := db.AutoMigrate(models.All()...); err != nil {
		return fmt.Errorf("auto migrate (drift): %w", err)
	}

	slog.Info("database schema is up to date")
	return nil
}

// applyMigration runs a single migration inside a transaction and records it.
func applyMigration(db *gorm.DB, m Migration) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := m.Up(tx); err != nil {
			return err
		}
		return tx.Create(&SchemaMigration{Version: m.Version, Name: m.Name, AppliedAt: time.Now().UTC()}).Error
	})
}

// AppliedMigrations lists the recorded migrations (for `migrate status`).
func AppliedMigrations(ctx context.Context, db *gorm.DB) ([]SchemaMigration, error) {
	var rows []SchemaMigration
	err := db.WithContext(ctx).Order("version ASC").Find(&rows).Error
	return rows, err
}
