package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// :memory: sqlite is per-connection; pin the pool so every query in the
	// test sees the same database.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(models.All()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCreateGetUpdateDelete(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := New[models.Regulation](db, cache.New(ctx, ""), "regulation")

	reg := models.Regulation{Title: "ISO 27001", Code: "ISO-27001", Jurisdiction: "Global"}
	if err := repo.Create(ctx, &reg); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if reg.ID == uuid.Nil {
		t.Fatal("Create did not assign a UUID")
	}

	got, err := repo.GetByID(ctx, reg.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Code != "ISO-27001" {
		t.Errorf("Code = %q, want ISO-27001", got.Code)
	}

	// Cache hit path must return the same entity.
	if err := repo.UpdatePartial(ctx, reg.ID, map[string]any{"status": "archived"}); err != nil {
		t.Fatalf("UpdatePartial: %v", err)
	}
	cached, err := repo.GetByID(ctx, reg.ID)
	if err != nil {
		t.Fatalf("GetByID (cached): %v", err)
	}
	if cached.Status != "archived" {
		t.Errorf("Status = %q, want archived", cached.Status)
	}

	if err := repo.Delete(ctx, reg.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, reg.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetByID after delete = %v, want ErrNotFound", err)
	}
}

func TestListPagination(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := New[models.Deadline](db, cache.New(ctx, ""), "deadline")

	base := time.Now().UTC().Add(24 * time.Hour)
	for i := 0; i < 5; i++ {
		d := models.Deadline{Title: "dl", DeadlineAt: base}
		if err := repo.Create(ctx, &d); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	items, total, err := repo.List(ctx, 1, 2, "created_at")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("page size = %d, want 2", len(items))
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}

	// Second page should not overlap the first.
	items2, _, err := repo.List(ctx, 2, 2, "created_at")
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	if len(items2) != 2 {
		t.Errorf("page 2 size = %d, want 2", len(items2))
	}
	if items[0].ID == items2[0].ID {
		t.Error("pages overlap")
	}
}

func TestSoftDeleteKeepsRow(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := New[models.Alert](db, cache.New(ctx, ""), "alert")

	a := models.Alert{Type: "x", Title: "t", Message: "m"}
	if err := repo.Create(ctx, &a); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Delete(ctx, a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// The row still physically exists; GORM's default scope excludes it.
	var count int64
	if err := db.Model(&models.Alert{}).Unscoped().Where("id = ?", a.ID).Count(&count).Error; err != nil {
		t.Fatalf("unscoped count: %v", err)
	}
	if count != 1 {
		t.Errorf("unscoped count = %d, want 1 (soft delete must keep the row)", count)
	}
}

func TestUpdateMissingReturnsNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := New[models.Compliance](db, cache.New(ctx, ""), "compliance")

	err := repo.UpdatePartial(ctx, uuid.New(), map[string]any{"status": "compliant"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdatePartial on missing id = %v, want ErrNotFound", err)
	}
}
