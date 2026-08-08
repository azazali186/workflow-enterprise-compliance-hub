package database

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	return db
}

func TestMigrateAppliesOnceAndIsIdempotent(t *testing.T) {
	db := newTestDB(t)

	// First run creates the schema_migrations table, applies the baseline and
	// creates every model table.
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate (first): %v", err)
	}

	rows, err := AppliedMigrations(context.Background(), db)
	if err != nil {
		t.Fatalf("AppliedMigrations: %v", err)
	}
	if len(rows) != len(migrations) {
		t.Fatalf("applied = %d, want %d", len(rows), len(migrations))
	}
	if rows[0].Version != migrations[0].Version {
		t.Errorf("first applied = %q, want %q", rows[0].Version, migrations[0].Version)
	}

	// A second run must not re-apply (idempotent) and must not error.
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate (second): %v", err)
	}
	rows2, _ := AppliedMigrations(context.Background(), db)
	if len(rows2) != len(migrations) {
		t.Fatalf("applied after second run = %d, want %d (no duplicates)", len(rows2), len(migrations))
	}
}
