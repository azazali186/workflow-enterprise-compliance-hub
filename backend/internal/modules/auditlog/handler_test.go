package auditlog

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

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

func newSearchEnv(t *testing.T) (*server.Hertz, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	pinSingleConnection(t, db)
	if err := db.AutoMigrate(models.All()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	entityID := uuid.New()
	logs := []models.AuditLog{
		{Action: "login", Status: "success", EntityType: "user", EntityID: uuid.New(), ActorID: "u1", IP: "10.0.0.1"},
		{Action: "login", Status: "failure", EntityType: "user", ActorID: "ghost", IP: "10.0.0.2"},
		{Action: "create", Status: "success", EntityType: "compliance", EntityID: entityID, ActorID: "u1", IP: "10.0.0.1"},
		{Action: "update", Status: "success", EntityType: "compliance", EntityID: entityID, ActorID: "u2", IP: "10.0.0.3"},
		{Action: "delete", Status: "success", EntityType: "compliance", EntityID: entityID, ActorID: "u2", IP: "10.0.0.3"},
	}
	for i := range logs {
		logs[i].CreatedAt = time.Date(2026, 1, 1, i, 0, 0, 0, time.UTC) // hour i
		if err := db.Create(&logs[i]).Error; err != nil {
			t.Fatalf("seed log %d: %v", i, err)
		}
	}

	d := deps.Deps{DB: db, Cache: cache.New(context.Background(), "")}
	h := server.New()
	RegisterRoutes(h.Group("/api/v1"), d)
	return h, db
}

func search(t *testing.T, h *server.Hertz, body string) (int, map[string]any) {
	t.Helper()
	w := ut.PerformRequest(h.Engine, "POST", "/api/v1/audit-logs/search",
		&ut.Body{Body: bytes.NewBufferString(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestAuditLogSearchFilters(t *testing.T) {
	h, _ := newSearchEnv(t)

	// Filter by actor.
	code, out := search(t, h, `{"limit":50,"filters":{"actor_id":"u2"}}`)
	if code != 200 {
		t.Fatalf("search status = %d, body = %v", code, out)
	}
	data, _ := out["data"].(map[string]any)
	items, _ := data["items"].([]any)
	if len(items) != 2 {
		t.Errorf("u2 logs = %d, want 2 (update + delete)", len(items))
	}

	// Filter by action + status.
	code, out = search(t, h, `{"limit":50,"filters":{"action":"login","status":"failure"}}`)
	if code != 200 {
		t.Fatalf("search status = %d, body = %v", code, out)
	}
	data, _ = out["data"].(map[string]any)
	items, _ = data["items"].([]any)
	if len(items) != 1 {
		t.Errorf("login failures = %d, want 1", len(items))
	}

	// Date range on created_at (hour 0 and 1 only).
	code, out = search(t, h, `{"limit":50,"date_range":{"field":"created_at","from":"2026-01-01T00:00:00Z","to":"2026-01-01T01:59:59Z"}}`)
	if code != 200 {
		t.Fatalf("search status = %d, body = %v", code, out)
	}
	data, _ = out["data"].(map[string]any)
	items, _ = data["items"].([]any)
	if len(items) != 2 {
		t.Errorf("date-range logs = %d, want 2", len(items))
	}
}

func TestAuditLogSearchSummary(t *testing.T) {
	h, _ := newSearchEnv(t)

	code, out := search(t, h, `{"limit":50,"include_summary":true}`)
	if code != 200 {
		t.Fatalf("search status = %d, body = %v", code, out)
	}
	data, _ := out["data"].(map[string]any)
	pagination, _ := data["pagination"].(map[string]any)
	summary, _ := pagination["summary"].(map[string]any)
	if total, _ := summary["total"].(float64); total != 5 {
		t.Errorf("summary total = %v, want 5", summary["total"])
	}
	actions, _ := summary["action"].(map[string]any)
	if actions["login"] != float64(2) || actions["create"] != float64(1) {
		t.Errorf("action summary = %v, want login:2 create:1", actions)
	}
}

func TestAuditLogGetByID(t *testing.T) {
	h, db := newSearchEnv(t)

	var row models.AuditLog
	if err := db.Where("action = ?", "update").First(&row).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}

	w := ut.PerformRequest(h.Engine, "POST", "/api/v1/audit-logs/get",
		&ut.Body{Body: bytes.NewBufferString(`{"id":"` + row.ID.String() + `"}`), Len: len(`{"id":"` + row.ID.String() + `"}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	if w.Code != 200 {
		t.Fatalf("get status = %d, body = %s", w.Code, w.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	data, _ := out["data"].(map[string]any)
	if data["action"] != "update" || data["entity_type"] != "compliance" {
		t.Errorf("get data = %v", data)
	}
}
