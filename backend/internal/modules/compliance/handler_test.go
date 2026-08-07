package compliance

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/glebarez/sqlite"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/bus"
	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/ws"
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

func newTestDeps(t *testing.T) deps.Deps {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	pinSingleConnection(t, db)
	if err := db.AutoMigrate(models.All()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	return deps.Deps{
		DB:     db,
		Bus:    bus.New(ctx, ""),
		Cache:  cache.New(ctx, ""),
		Hub:    ws.NewHub(100, 30*time.Second),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func newTestServer(deps deps.Deps) *server.Hertz {
	h := server.New()
	RegisterRoutes(h.Group("/api/v1"), deps)
	return h
}

func post(t *testing.T, h *server.Hertz, path, body string) (int, map[string]any) {
	t.Helper()
	w := ut.PerformRequest(h.Engine, "POST", path, &ut.Body{Body: bytes.NewBufferString(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestCreateGetSearchCheck(t *testing.T) {
	deps := newTestDeps(t)
	h := newTestServer(deps)

	// Create (due date in the future).
	code, out := post(t, h, "/api/v1/compliances",
		`{"name":"GDPR annual review","status":"active","risk_level":"high","due_date":"2026-08-09T00:00:00Z"}`)
	if code != 201 {
		t.Fatalf("create status = %d, body = %v", code, out)
	}
	data, _ := out["data"].(map[string]any)
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatalf("create returned no id: %v", out)
	}

	// Get by id (body-only: POST /get with {"id": ...}).
	code, out = post(t, h, "/api/v1/compliances/get", `{"id":"`+id+`"}`)
	if code != 200 {
		t.Fatalf("get status = %d, body = %v", code, out)
	}
	if d, _ := out["data"].(map[string]any); d["name"] != "GDPR annual review" {
		t.Errorf("name = %v, want GDPR annual review", d["name"])
	}

	// Search with cursor pagination (body-only: POST /search).
	code, out = post(t, h, "/api/v1/compliances/search", `{"limit":10,"include_summary":true}`)
	if code != 200 {
		t.Fatalf("search status = %d, body = %v", code, out)
	}
	data, _ = out["data"].(map[string]any)
	items, _ := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("search items = %d, want 1", len(items))
	}
	pagination, _ := data["pagination"].(map[string]any)
	if pagination["count"] != float64(1) {
		t.Errorf("pagination count = %v, want 1", pagination["count"])
	}
	if _, ok := pagination["summary"]; !ok {
		t.Errorf("pagination summary missing, want db summary block")
	}

	// Status check within due date => compliant.
	code, out = post(t, h, "/api/v1/compliances/check", `{"id":"`+id+`"}`)
	if code != 200 {
		t.Fatalf("check status = %d, body = %v", code, out)
	}
	checkData, _ := out["data"].(map[string]any)
	complianceData, _ := checkData["compliance"].(map[string]any)
	if complianceData["status"] != "compliant" {
		t.Errorf("status after check = %v, want compliant", complianceData["status"])
	}
}

func TestCheckMarksNonCompliant(t *testing.T) {
	deps := newTestDeps(t)
	h := newTestServer(deps)

	// Due date in the past and no review => non_compliant.
	code, out := post(t, h, "/api/v1/compliances",
		`{"name":"Expired review","status":"active","due_date":"2020-01-01T00:00:00Z"}`)
	if code != 201 {
		t.Fatalf("create status = %d, body = %v", code, out)
	}
	id, _ := out["data"].(map[string]any)["id"].(string)

	code, out = post(t, h, "/api/v1/compliances/check", `{"id":"`+id+`"}`)
	if code != 200 {
		t.Fatalf("check status = %d, body = %v", code, out)
	}
	data, _ := out["data"].(map[string]any)
	complianceData, _ := data["compliance"].(map[string]any)
	if complianceData["status"] != "non_compliant" {
		t.Errorf("status after check = %v, want non_compliant", complianceData["status"])
	}
}

func TestCreateRequiresName(t *testing.T) {
	deps := newTestDeps(t)
	h := newTestServer(deps)

	code, out := post(t, h, "/api/v1/compliances", `{"status":"active"}`)
	if code != 400 {
		t.Fatalf("create without name status = %d, want 400, body = %v", code, out)
	}
}

func TestUpdateAndDeleteAreBodyOnly(t *testing.T) {
	deps := newTestDeps(t)
	h := newTestServer(deps)

	code, out := post(t, h, "/api/v1/compliances", `{"name":"To update"}`)
	if code != 201 {
		t.Fatalf("create status = %d, body = %v", code, out)
	}
	id, _ := out["data"].(map[string]any)["id"].(string)

	// PATCH with {"id": ..., "metadata": {"a":1}} — jsonb columns arrive as
	// nested maps and must be persisted (not rejected by GORM).
	w := ut.PerformRequest(h.Engine, "PATCH", "/api/v1/compliances",
		&ut.Body{Body: bytes.NewBufferString(`{"id":"` + id + `","metadata":{"reviewer":"alice"}}`), Len: len(`{"id":"` + id + `","metadata":{"reviewer":"alice"}}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	if w.Code != 200 {
		t.Fatalf("patch metadata status = %d, body = %s", w.Code, w.Body.String())
	}
	var metaPatch map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &metaPatch)
	meta, _ := metaPatch["data"].(map[string]any)["metadata"].(map[string]any)
	if meta["reviewer"] != "alice" {
		t.Errorf("metadata after patch = %v, want reviewer=alice", meta)
	}

	// PATCH with {"id": ..., "risk_level": "critical"}.
	w = ut.PerformRequest(h.Engine, "PATCH", "/api/v1/compliances",
		&ut.Body{Body: bytes.NewBufferString(`{"id":"` + id + `","risk_level":"critical"}`), Len: len(`{"id":"` + id + `","risk_level":"critical"}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	if w.Code != 200 {
		t.Fatalf("patch status = %d, body = %s", w.Code, w.Body.String())
	}
	var patched map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &patched)
	if d, _ := patched["data"].(map[string]any); d["risk_level"] != "critical" {
		t.Errorf("risk_level after patch = %v, want critical", d["risk_level"])
	}

	// DELETE with {"id": ...}; subsequent get must 404.
	w = ut.PerformRequest(h.Engine, "DELETE", "/api/v1/compliances",
		&ut.Body{Body: bytes.NewBufferString(`{"id":"` + id + `"}`), Len: len(`{"id":"` + id + `"}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	if w.Code != 204 {
		t.Fatalf("delete status = %d, body = %s", w.Code, w.Body.String())
	}
	code, out = post(t, h, "/api/v1/compliances/get", `{"id":"`+id+`"}`)
	if code != 404 {
		t.Fatalf("get after delete status = %d, want 404, body = %v", code, out)
	}

	// --- Audit trail: every mutation must be recorded with snapshots ---
	isNull := func(b datatypes.JSON) bool { return len(b) == 0 || string(b) == "null" }

	var created models.AuditLog
	if err := deps.DB.Where("action = ? AND entity_id = ?", "create", id).First(&created).Error; err != nil {
		t.Fatalf("create audit row: %v", err)
	}
	if !isNull(created.BeforeData) || isNull(created.AfterData) {
		t.Errorf("create audit: want empty before + snapshot after, got before=%q after=%q", created.BeforeData, created.AfterData)
	}

	// The test performs two PATCHes (metadata, then risk_level), so two update
	// rows exist; find the one carrying the risk_level change.
	var updateRows []models.AuditLog
	if err := deps.DB.Where("action = ? AND entity_id = ?", "update", id).Find(&updateRows).Error; err != nil {
		t.Fatalf("update audit rows: %v", err)
	}
	riskChanged := false
	for _, u := range updateRows {
		var changes map[string]map[string]any
		_ = json.Unmarshal(u.Changes, &changes)
		if c, ok := changes["risk_level"]; ok && c["before"] == "medium" && c["after"] == "critical" {
			riskChanged = true
		}
	}
	if !riskChanged {
		t.Error("no update audit row records risk_level medium -> critical")
	}

	var deleted models.AuditLog
	if err := deps.DB.Where("action = ? AND entity_id = ?", "delete", id).First(&deleted).Error; err != nil {
		t.Fatalf("delete audit row: %v", err)
	}
	if isNull(deleted.BeforeData) || !isNull(deleted.AfterData) {
		t.Errorf("delete audit: want snapshot before + empty after, got before=%q after=%q", deleted.BeforeData, deleted.AfterData)
	}
}
