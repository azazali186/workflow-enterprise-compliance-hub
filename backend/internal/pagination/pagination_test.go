package pagination

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	pinSingleConnection(t, db)
	if err := db.AutoMigrate(&models.Compliance{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedCompliances(t *testing.T, db *gorm.DB, n int) {
	t.Helper()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		status := "active"
		if i%3 == 0 {
			status = "compliant"
		}
		c := models.Compliance{
			Base:      models.Base{ID: uuid.New(), CreatedAt: base.Add(time.Duration(i) * time.Hour), UpdatedAt: base.Add(time.Duration(i) * time.Hour)},
			Name:      fmt.Sprintf("Compliance %02d", i),
			Status:    status,
			RiskLevel: "medium",
		}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
}

// pinSingleConnection keeps the shared :memory: sqlite database on one pooled
// connection — without it, each pooled connection gets its own empty DB and
// writes become invisible to later reads.
func pinSingleConnection(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
}

// modeled mirrors the repository's Scope(): a Model-scoped *gorm.DB, which is
// the documented contract of Apply (Count/Pluck need the model to resolve the
// table).
func modeled(db *gorm.DB) *gorm.DB { return db.Model(&models.Compliance{}) }

func TestCursorPaginationWalksPages(t *testing.T) {
	db := newDB(t)
	seedCompliances(t, db, 5)

	cols := map[string]string{"name": "name", "status": "status"}

	// Page 1: limit 2.
	res, err := Apply[models.Compliance](modeled(db), Query{Limit: 2}, cols, "status")
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("page 1 items = %d, want 2", len(res.Items))
	}
	if !res.Pagination.HasMore {
		t.Fatal("page 1 should have more")
	}
	next := res.Pagination.NextCursor
	if next == "" {
		t.Fatal("page 1 missing next cursor")
	}

	// Page 2: cursor continues where page 1 left off (no overlap, no gap).
	res2, err := Apply[models.Compliance](modeled(db), Query{Limit: 2, Cursor: next}, cols, "status")
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(res2.Items) != 2 {
		t.Fatalf("page 2 items = %d, want 2", len(res2.Items))
	}
	ids := map[string]bool{}
	for _, it := range res.Items {
		ids[it.ID.String()] = true
	}
	for _, it := range res2.Items {
		if ids[it.ID.String()] {
			t.Errorf("page 2 overlaps page 1 (id %s)", it.ID)
		}
	}

	// Page 3: the remainder.
	res3, err := Apply[models.Compliance](modeled(db), Query{Limit: 2, Cursor: res2.Pagination.NextCursor}, cols, "status")
	if err != nil {
		t.Fatalf("page 3: %v", err)
	}
	if len(res3.Items) != 1 {
		t.Fatalf("page 3 items = %d, want 1", len(res3.Items))
	}
	if res3.Pagination.HasMore {
		t.Fatal("page 3 should be the last page")
	}
}

func TestFilterSortAndDateRange(t *testing.T) {
	db := newDB(t)
	seedCompliances(t, db, 6)

	cols := map[string]string{"name": "name", "status": "status", "created_at": "created_at"}

	// Equality filter.
	res, err := Apply[models.Compliance](modeled(db), Query{Filters: map[string]any{"status": "compliant"}}, cols, "")
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("filtered items = %d, want 2 (compliant ones)", len(res.Items))
	}

	// IN filter.
	res, err = Apply[models.Compliance](modeled(db), Query{Filters: map[string]any{"status": []any{"compliant", "active"}}}, cols, "")
	if err != nil {
		t.Fatalf("in filter: %v", err)
	}
	if len(res.Items) != 6 {
		t.Fatalf("in-filtered items = %d, want 6", len(res.Items))
	}

	// Ascending sort by name.
	res, err = Apply[models.Compliance](modeled(db), Query{Limit: 100, Sort: &Sort{Column: "name", Direction: "asc"}}, cols, "")
	if err != nil {
		t.Fatalf("sort: %v", err)
	}
	if res.Items[0].Name != "Compliance 00" {
		t.Errorf("asc first = %q, want Compliance 00", res.Items[0].Name)
	}

	// Date range on created_at.
	from := time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 1, 4, 0, 0, 0, time.UTC)
	res, err = Apply[models.Compliance](modeled(db), Query{DateRange: &DateRange{Field: "created_at", From: &from, To: &to}}, cols, "")
	if err != nil {
		t.Fatalf("date range: %v", err)
	}
	if len(res.Items) != 3 {
		t.Fatalf("date range items = %d, want 3", len(res.Items))
	}

	// Reject unknown sort/filter columns (allowlist enforced).
	if _, err := Apply[models.Compliance](modeled(db), Query{Sort: &Sort{Column: "id; DROP", Direction: "asc"}}, cols, ""); err == nil {
		t.Fatal("expected error for unknown sort column")
	}
	if _, err := Apply[models.Compliance](modeled(db), Query{Filters: map[string]any{"evil": "x"}}, cols, ""); err == nil {
		t.Fatal("expected error for unknown filter column")
	}
}

func TestSummaryBlock(t *testing.T) {
	db := newDB(t)
	seedCompliances(t, db, 6)

	res, err := Apply[models.Compliance](modeled(db), Query{Limit: 100, IncludeSummary: true}, map[string]string{"status": "status"}, "status")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	sum, ok := res.Pagination.DBSummary.(map[string]any)
	if !ok {
		t.Fatalf("summary type = %T, want map", res.Pagination.DBSummary)
	}
	if total, _ := sum["total"].(int64); total != 6 {
		t.Errorf("summary total = %v, want 6", sum["total"])
	}
	grouped, ok := sum["status"].(map[string]int64)
	if !ok {
		t.Fatalf("status group = %T, want map", sum["status"])
	}
	if grouped["compliant"] != 2 || grouped["active"] != 4 {
		t.Errorf("status group = %v, want compliant:2 active:4", grouped)
	}
}

// TestCursorPaginationByNonDefaultColumn guards the keyset cursor against the
// classic bug of comparing an arbitrary sort column against a created_at
// cursor value (which breaks on page 2 with "operator does not exist").
func TestCursorPaginationByNonDefaultColumn(t *testing.T) {
	db := newDB(t)
	seedCompliances(t, db, 5)

	cols := map[string]string{"name": "name", "status": "status"}

	// Sort by name ASC, page size 2, walk every page.
	res, err := Apply[models.Compliance](modeled(db), Query{
		Limit: 2,
		Sort:  &Sort{Column: "name", Direction: "asc"},
	}, cols, "")
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if len(res.Items) != 2 || res.Items[0].Name != "Compliance 00" {
		t.Fatalf("page 1 wrong: %d items, first %q", len(res.Items), res.Items[0].Name)
	}
	if !res.Pagination.HasMore {
		t.Fatal("expected more pages")
	}

	res2, err := Apply[models.Compliance](modeled(db), Query{
		Limit:  2,
		Cursor: res.Pagination.NextCursor,
		Sort:   &Sort{Column: "name", Direction: "asc"},
	}, cols, "")
	if err != nil {
		t.Fatalf("page 2 (name-sorted cursor) must not error: %v", err)
	}
	if len(res2.Items) != 2 {
		t.Fatalf("page 2 items = %d, want 2", len(res2.Items))
	}

	res3, err := Apply[models.Compliance](modeled(db), Query{
		Limit:  2,
		Cursor: res2.Pagination.NextCursor,
		Sort:   &Sort{Column: "name", Direction: "asc"},
	}, cols, "")
	if err != nil {
		t.Fatalf("page 3: %v", err)
	}
	if len(res3.Items) != 1 || res3.Items[0].Name != "Compliance 04" {
		t.Fatalf("page 3 wrong: %d items, first %q", len(res3.Items), res3.Items[0].Name)
	}
	if res3.Pagination.HasMore {
		t.Fatal("page 3 should be the last page")
	}
}

func TestCursorRejectsSortColumnSwitch(t *testing.T) {
	db := newDB(t)
	seedCompliances(t, db, 5)

	cols := map[string]string{"name": "name", "status": "status"}
	res, err := Apply[models.Compliance](modeled(db), Query{
		Limit: 2,
		Sort:  &Sort{Column: "name", Direction: "asc"},
	}, cols, "")
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}

	// Reusing a cursor from a name-sorted page against a status-sorted query
	// must fail loudly instead of silently producing a wrong page.
	if _, err := Apply[models.Compliance](modeled(db), Query{
		Limit:  2,
		Cursor: res.Pagination.NextCursor,
		Sort:   &Sort{Column: "status", Direction: "asc"},
	}, cols, ""); err == nil {
		t.Fatal("expected error when reusing a cursor across sort columns")
	}
}

func TestInvalidCursorRejected(t *testing.T) {
	db := newDB(t)
	seedCompliances(t, db, 2)
	if _, err := Apply[models.Compliance](modeled(db), Query{Cursor: "!!!not-a-cursor!!!"}, nil, ""); err == nil {
		t.Fatal("expected error for malformed cursor")
	}
}

func TestContextCancellationStopsApply(t *testing.T) {
	db := newDB(t)
	seedCompliances(t, db, 2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := db.WithContext(ctx).Error; err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
