package options

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/bus"
	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/ws"
)

func newTestDeps(t *testing.T) deps.Deps {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	// One pooled connection so :memory: data is visible to every query.
	sqlDB.SetMaxOpenConns(1)
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

// newTestServer mounts only the options routes. A non-empty role simulates the
// rbac middleware by stamping auth_role_code before the handler runs.
func newTestServer(d deps.Deps, role string) *server.Hertz {
	h := server.New()
	if role != "" {
		h.Use(func(ctx context.Context, c *app.RequestContext) {
			c.Set("auth_role_code", role)
			c.Next(ctx)
		})
	}
	RegisterRoutes(h.Group("/api/v1"), d)
	return h
}

func postOptions(t *testing.T, h *server.Hertz, body string) (int, map[string]any) {
	t.Helper()
	w := ut.PerformRequest(h.Engine, "POST", "/api/v1/options",
		&ut.Body{Body: bytes.NewBufferString(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func seedCompliance(t *testing.T, db *gorm.DB, name string) string {
	t.Helper()
	c := models.Compliance{Name: name, Status: "active"}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("seed compliance: %v", err)
	}
	return c.ID.String()
}

func seedRegulation(t *testing.T, db *gorm.DB, title string) string {
	t.Helper()
	r := models.Regulation{Title: title, Code: title}
	if err := db.Create(&r).Error; err != nil {
		t.Fatalf("seed regulation: %v", err)
	}
	return r.ID.String()
}

func TestOptionsSearchAndIDResolution(t *testing.T) {
	d := newTestDeps(t)
	h := newTestServer(d, "")

	compID := seedCompliance(t, d.DB, "GDPR annual review")
	seedCompliance(t, d.DB, "SOC 2 readiness")
	regID := seedRegulation(t, d.DB, "ISO 27001")

	// Search across entities in one request.
	code, out := postOptions(t, h, `{"entities":["compliances","regulations","users"],"search":"GDPR"}`)
	if code != 200 {
		t.Fatalf("search status = %d, body = %v", code, out)
	}
	data, _ := out["data"].(map[string]any)
	comps, _ := data["compliances"].([]any)
	if len(comps) != 1 {
		t.Fatalf("compliances matches = %d, want 1 (case-insensitive search)", len(comps))
	}
	first, _ := comps[0].(map[string]any)
	if first["name"] != "GDPR annual review" || first["id"] != compID {
		t.Errorf("compliance option = %v, want id=%s name=GDPR annual review", first, compID)
	}
	if regs, _ := data["regulations"].([]any); len(regs) != 0 {
		t.Errorf("regulations matches = %d, want 0 for search GDPR", len(regs))
	}

	// ids filter resolves the stored value of an edit form.
	code, out = postOptions(t, h, `{"entities":["compliances","regulations"],"ids":{"compliances":["`+compID+`"],"regulations":["`+regID+`"]}}`)
	if code != 200 {
		t.Fatalf("ids status = %d, body = %v", code, out)
	}
	data, _ = out["data"].(map[string]any)
	comps, _ = data["compliances"].([]any)
	if len(comps) != 1 {
		t.Fatalf("ids compliances = %d, want 1", len(comps))
	}
	regs, _ := data["regulations"].([]any)
	if len(regs) != 1 {
		t.Fatalf("ids regulations = %d, want 1", len(regs))
	}
	if r, _ := regs[0].(map[string]any); r["name"] != "ISO 27001" {
		t.Errorf("regulation option = %v, want ISO 27001", r)
	}
}

func TestOptionsSearchByNameColumnPerEntity(t *testing.T) {
	d := newTestDeps(t)
	h := newTestServer(d, "")

	seedRegulation(t, d.DB, "PCI DSS")
	seedCompliance(t, d.DB, "SOC 2 readiness")

	// "PCI" only matches the regulation title, not compliance names.
	code, out := postOptions(t, h, `{"entities":["compliances","regulations"],"search":"PCI"}`)
	if code != 200 {
		t.Fatalf("status = %d, body = %v", code, out)
	}
	data, _ := out["data"].(map[string]any)
	if regs, _ := data["regulations"].([]any); len(regs) != 1 {
		t.Errorf("regulations = %d, want 1", len(regs))
	}
	if comps, _ := data["compliances"].([]any); len(comps) != 0 {
		t.Errorf("compliances = %d, want 0", len(comps))
	}
}

func TestOptionsLimitAndDedupe(t *testing.T) {
	d := newTestDeps(t)
	h := newTestServer(d, "")

	id := seedCompliance(t, d.DB, "Limit me")
	// Same id in both filters must not duplicate the row.
	code, out := postOptions(t, h, `{"entities":["compliances"],"search":"Limit","ids":{"compliances":["`+id+`"]},"limit":1}`)
	if code != 200 {
		t.Fatalf("status = %d, body = %v", code, out)
	}
	data, _ := out["data"].(map[string]any)
	if comps, _ := data["compliances"].([]any); len(comps) != 1 {
		t.Errorf("compliances = %d, want 1 (deduped)", len(comps))
	}
}

func TestOptionsViewerCannotSeeIdentityEntities(t *testing.T) {
	d := newTestDeps(t)
	h := newTestServer(d, "viewer")

	user := models.User{Username: "alice", Status: "active"}
	if err := d.DB.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	seedCompliance(t, d.DB, "GDPR annual review")

	code, out := postOptions(t, h, `{"entities":["compliances","users","roles"],"search":""}`)
	if code != 200 {
		t.Fatalf("status = %d, body = %v", code, out)
	}
	data, _ := out["data"].(map[string]any)
	if _, ok := data["users"]; ok {
		t.Errorf("viewer must not receive users options, got %v", data["users"])
	}
	if _, ok := data["roles"]; ok {
		t.Errorf("viewer must not receive roles options, got %v", data["roles"])
	}
	if comps, _ := data["compliances"].([]any); len(comps) != 1 {
		t.Errorf("viewer compliances = %d, want 1", len(comps))
	}
}

func TestOptionsErrors(t *testing.T) {
	d := newTestDeps(t)
	h := newTestServer(d, "")

	// Malformed body -> 400.
	code, _ := postOptions(t, h, `{"entities":`)
	if code != 400 {
		t.Fatalf("malformed body status = %d, want 400", code)
	}

	// Unknown entity keys are ignored (no error, empty result).
	code, out := postOptions(t, h, `{"entities":["bogus","also_bogus"],"search":"x"}`)
	if code != 200 {
		t.Fatalf("unknown entity status = %d, want 200, body = %v", code, out)
	}
	if data, _ := out["data"].(map[string]any); len(data) != 0 {
		t.Errorf("data = %v, want empty map", data)
	}

	// Empty request body (no entities) -> 200 with empty data.
	code, out = postOptions(t, h, `{}`)
	if code != 200 {
		t.Fatalf("empty body status = %d, want 200, body = %v", code, out)
	}
}
