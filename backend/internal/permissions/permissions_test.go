package permissions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

func newTestDeps(t *testing.T) deps.Deps {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
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
	return deps.Deps{DB: db, Cache: cache.New(context.Background(), "")}
}

// newTestServer registers a small, representative route set without spinning
// up the full application.
func newTestServer() *server.Hertz {
	h := server.New()
	api := h.Group("/api/v1")
	api.GET("/compliances", func(c context.Context, _ *app.RequestContext) {})
	api.POST("/compliances", func(c context.Context, _ *app.RequestContext) {})
	api.GET("/compliances/:id", func(c context.Context, _ *app.RequestContext) {})
	// /ws lives on the root group, mirroring the real server.
	h.GET("/ws", func(c context.Context, _ *app.RequestContext) {})
	return h
}

func TestGenerateExtractsRoutes(t *testing.T) {
	deps := newTestDeps(t)
	dir := t.TempDir()
	outPath := filepath.Join(dir, "routes.json")

	routes, err := Generate(newTestServer(), deps, outPath)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(routes) != 4 {
		t.Fatalf("route count = %d, want 4: %+v", len(routes), routes)
	}

	// Manifest file written and parseable.
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read routes.json: %v", err)
	}
	var written []RouteInfo
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("parse routes.json: %v", err)
	}
	if len(written) != 4 {
		t.Errorf("routes.json entries = %d, want 4", len(written))
	}

	// Cache key holds the same manifest (ttl<=0 must not expire immediately).
	raw, ok := deps.Cache.Get(context.Background(), CacheKey)
	if !ok || raw == "" {
		t.Fatal("cache key api-gateway-permissions not set")
	}
	var cached []RouteInfo
	if err := json.Unmarshal([]byte(raw), &cached); err != nil {
		t.Fatalf("parse cached manifest: %v", err)
	}
	if len(cached) != 4 {
		t.Errorf("cached entries = %d, want 4", len(cached))
	}

	// Permission rows upserted.
	var count int64
	if err := deps.DB.Model(&models.Permission{}).Count(&count).Error; err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	if count != 4 {
		t.Errorf("permission rows = %d, want 4", count)
	}

	// /ws gets the special human-readable name.
	var wsPerm models.Permission
	if err := deps.DB.Where("route = ?", "GET /ws").First(&wsPerm).Error; err != nil {
		t.Fatalf("find GET /ws permission: %v", err)
	}
	if wsPerm.Name != "WebSocket Connection" {
		t.Errorf("ws permission name = %q, want WebSocket Connection", wsPerm.Name)
	}
	if wsPerm.Service != "api-gateway" {
		t.Errorf("ws permission service = %q, want api-gateway", wsPerm.Service)
	}
}

func TestGenerateIsIdempotent(t *testing.T) {
	deps := newTestDeps(t)
	outPath := filepath.Join(t.TempDir(), "routes.json")

	if _, err := Generate(newTestServer(), deps, outPath); err != nil {
		t.Fatalf("first Generate: %v", err)
	}
	if _, err := Generate(newTestServer(), deps, outPath); err != nil {
		t.Fatalf("second Generate: %v", err)
	}

	// Running twice must not create duplicate permission rows.
	var count int64
	if err := deps.DB.Model(&models.Permission{}).Count(&count).Error; err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	if count != 4 {
		t.Errorf("permission rows after second run = %d, want still 4", count)
	}
}

func TestGenerateUpdatesChangedRows(t *testing.T) {
	deps := newTestDeps(t)
	outPath := filepath.Join(t.TempDir(), "routes.json")

	if _, err := Generate(newTestServer(), deps, outPath); err != nil {
		t.Fatalf("first Generate: %v", err)
	}

	// Tamper with a row, then re-sync: the drift must be corrected.
	if err := deps.DB.Model(&models.Permission{}).
		Where("route = ?", "GET /api/v1/compliances/:id").
		Update("name", "Tampered Name").Error; err != nil {
		t.Fatalf("tamper: %v", err)
	}

	if _, err := Generate(newTestServer(), deps, outPath); err != nil {
		t.Fatalf("second Generate: %v", err)
	}

	var perm models.Permission
	if err := deps.DB.Where("route = ?", "GET /api/v1/compliances/:id").First(&perm).Error; err != nil {
		t.Fatalf("find updated permission: %v", err)
	}
	if perm.Name != "Compliances :Id" {
		t.Errorf("name after re-sync = %q, want %q (update path)", perm.Name, "Compliances :Id")
	}
}

func TestGeneratePrunesRemovedRoutes(t *testing.T) {
	deps := newTestDeps(t)
	outPath := filepath.Join(t.TempDir(), "routes.json")

	if _, err := Generate(newTestServer(), deps, outPath); err != nil {
		t.Fatalf("first Generate: %v", err)
	}

	// New server without /ws: the GET /ws permission must be pruned.
	lean := server.New()
	api := lean.Group("/api/v1")
	api.GET("/compliances", func(c context.Context, _ *app.RequestContext) {})
	api.POST("/compliances", func(c context.Context, _ *app.RequestContext) {})
	api.GET("/compliances/:id", func(c context.Context, _ *app.RequestContext) {})

	if _, err := Generate(lean, deps, outPath); err != nil {
		t.Fatalf("second Generate: %v", err)
	}

	var count int64
	if err := deps.DB.Model(&models.Permission{}).Count(&count).Error; err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	if count != 3 {
		t.Errorf("permission rows after prune = %d, want 3", count)
	}

	var stale int64
	deps.DB.Unscoped().Model(&models.Permission{}).Where("route = ?", "GET /ws").Count(&stale)
	if stale != 0 {
		t.Errorf("stale GET /ws rows = %d, want 0 (pruned)", stale)
	}
}
