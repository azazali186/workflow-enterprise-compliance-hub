package auditlog

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

func TestPruneBefore(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	pinSingleConnection(t, db)
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()

	now := time.Now().UTC()
	old := now.Add(-400 * 24 * time.Hour)
	recent := now.Add(-10 * time.Hour)
	for i, createdAt := range []time.Time{old, old, recent} {
		row := models.AuditLog{Action: "login", Status: "success"}
		row.CreatedAt = createdAt
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}

	// Cutoff at 365 days: the two 400-day-old rows are pruned, the recent one
	// survives.
	cutoff := now.Add(-365 * 24 * time.Hour)
	n, err := PruneBefore(ctx, db, cutoff)
	if err != nil {
		t.Fatalf("PruneBefore: %v", err)
	}
	if n != 2 {
		t.Fatalf("pruned = %d, want 2", n)
	}

	var remaining int64
	if err := db.Model(&models.AuditLog{}).Count(&remaining).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 1 {
		t.Errorf("remaining = %d, want 1", remaining)
	}

	// Second run prunes nothing.
	if n2, _ := PruneBefore(ctx, db, cutoff); n2 != 0 {
		t.Errorf("second prune = %d, want 0", n2)
	}
}
