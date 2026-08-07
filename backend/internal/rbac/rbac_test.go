package rbac

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/aeroxe/compliance-hub/backend/internal/auth"
	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/config"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/respond"
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

func newTestEnv(t *testing.T) (deps.Deps, *config.Config) {
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
	ctx := context.Background()
	d := deps.Deps{DB: db, Cache: cache.New(ctx, "")}
	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiry: time.Hour}
	return d, cfg
}

func seedPermissions(t *testing.T, db *gorm.DB) {
	t.Helper()
	perms := []models.Permission{
		{Name: "Compliances", Route: "POST /api/v1/compliances", Path: "/api/v1/compliances", Method: "POST", Service: "api-gateway"},
		{Name: "Compliances Search", Route: "POST /api/v1/compliances/search", Path: "/api/v1/compliances/search", Method: "POST", Service: "api-gateway"},
		{Name: "Compliances", Route: "PATCH /api/v1/compliances", Path: "/api/v1/compliances", Method: "PATCH", Service: "api-gateway"},
		{Name: "Compliances", Route: "DELETE /api/v1/compliances", Path: "/api/v1/compliances", Method: "DELETE", Service: "api-gateway"},
		{Name: "Analytics Summary", Route: "POST /api/v1/analytics/summary", Path: "/api/v1/analytics/summary", Method: "POST", Service: "api-gateway"},
		{Name: "Me", Route: "POST /api/v1/auth/me", Path: "/api/v1/auth/me", Method: "POST", Service: "api-gateway"},
		{Name: "Login", Route: "POST /api/v1/auth/login", Path: "/api/v1/auth/login", Method: "POST", Service: "api-gateway"},
	}
	for i := range perms {
		perms[i].ID = uuid.New()
		if err := db.Create(&perms[i]).Error; err != nil {
			t.Fatalf("seed permission: %v", err)
		}
	}
}

func TestSeedCreatesRolesAndAdmin(t *testing.T) {
	d, _ := newTestEnv(t)
	seedPermissions(t, d.DB)

	ctx := context.Background()
	if err := Seed(ctx, d, "admin", "S3cretPass!"); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	// Three roles exist.
	for _, code := range []string{RoleAdmin, RoleComplianceOfficer, RoleViewer} {
		var role models.Role
		if err := d.DB.Where("code = ?", code).First(&role).Error; err != nil {
			t.Fatalf("role %s missing: %v", code, err)
		}
	}

	// Admin has every permission.
	var admin models.Role
	if err := d.DB.Preload("Permissions").Where("code = ?", RoleAdmin).First(&admin).Error; err != nil {
		t.Fatalf("admin role: %v", err)
	}
	if len(admin.Permissions) != 7 {
		t.Errorf("admin permissions = %d, want 7", len(admin.Permissions))
	}

	// Viewer only has read-only + own-session routes.
	var viewer models.Role
	if err := d.DB.Preload("Permissions").Where("code = ?", RoleViewer).First(&viewer).Error; err != nil {
		t.Fatalf("viewer role: %v", err)
	}
	allowed := map[string]bool{}
	for _, p := range viewer.Permissions {
		allowed[p.Route] = true
	}
	if !allowed["POST /api/v1/compliances/search"] {
		t.Error("viewer missing search permission")
	}
	if !allowed["POST /api/v1/analytics/summary"] {
		t.Error("viewer missing analytics permission")
	}
	if !allowed["POST /api/v1/auth/me"] {
		t.Error("viewer missing own-profile permission")
	}
	if allowed["POST /api/v1/compliances"] || allowed["PATCH /api/v1/compliances"] || allowed["DELETE /api/v1/compliances"] {
		t.Error("viewer must not get write permissions")
	}

	// Bootstrap admin user exists with a checkable password.
	var user models.User
	if err := d.DB.Where("username = ?", "admin").First(&user).Error; err != nil {
		t.Fatalf("admin user: %v", err)
	}
	if !auth.CheckPassword(user.PasswordHash, "S3cretPass!") {
		t.Fatal("admin password hash does not verify")
	}

	// Seeding twice is idempotent.
	if err := Seed(ctx, d, "admin", "S3cretPass!"); err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	var n int64
	d.DB.Model(&models.User{}).Where("username = ?", "admin").Count(&n)
	if n != 1 {
		t.Errorf("admin users after re-seed = %d, want 1", n)
	}
}

func TestRoleAllowsGrantedAndDenied(t *testing.T) {
	d, _ := newTestEnv(t)
	seedPermissions(t, d.DB)
	if err := Seed(context.Background(), d, "admin", "pw"); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	ctx := context.Background()
	ok, err := roleAllows(ctx, d.DB, d.Cache, RoleViewer, "POST /api/v1/compliances/search")
	if err != nil || !ok {
		t.Fatalf("viewer search allowed = %v err = %v, want true", ok, err)
	}
	ok, err = roleAllows(ctx, d.DB, d.Cache, RoleViewer, "POST /api/v1/compliances")
	if err != nil || ok {
		t.Fatalf("viewer write allowed = %v err = %v, want false", ok, err)
	}
	ok, err = roleAllows(ctx, d.DB, d.Cache, RoleComplianceOfficer, "PATCH /api/v1/compliances")
	if err != nil || !ok {
		t.Fatalf("officer patch allowed = %v err = %v, want true", ok, err)
	}
	ok, err = roleAllows(ctx, d.DB, d.Cache, RoleComplianceOfficer, "DELETE /api/v1/compliances")
	if err != nil || ok {
		t.Fatalf("officer delete allowed = %v err = %v, want false", ok, err)
	}

	// Cache path: second lookup must not touch the DB and stay correct.
	ok, err = roleAllows(ctx, d.DB, d.Cache, RoleViewer, "POST /api/v1/compliances/search")
	if err != nil || !ok {
		t.Fatalf("cached viewer search allowed = %v err = %v, want true", ok, err)
	}
	if raw, present := d.Cache.Get(ctx, roleCacheKey(RoleViewer)); !present || raw == "" {
		t.Fatal("role cache key missing after first lookup")
	}
}

func TestMiddlewareChain(t *testing.T) {
	d, cfg := newTestEnv(t)
	seedPermissions(t, d.DB)
	if err := Seed(context.Background(), d, "admin", "pw"); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	h := server.New()
	h.Use(Auth(cfg, d.Cache))
	h.Use(Permission(d.DB, d.Cache))

	// Public route.
	h.POST("/api/v1/auth/login", func(ctx context.Context, c *app.RequestContext) {
		respond.OK(c, map[string]any{"logged_in": true})
	})
	// Protected route.
	h.POST("/api/v1/compliances", func(ctx context.Context, c *app.RequestContext) {
		respond.OK(c, map[string]any{"role": c.GetString("auth_role_code")})
	})

	issue := func(username string) string {
		role := RoleViewer
		if username == "admin" {
			role = RoleAdmin
		}
		userID := uuid.NewString()
		tok, err := auth.IssueToken(cfg.JWTSecret, cfg.JWTExpiry, userID, username, role)
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		if err := auth.StoreSession(context.Background(), d.Cache, userID, tok, cfg.JWTExpiry); err != nil {
			t.Fatalf("session: %v", err)
		}
		return tok
	}

	body := `{}`
	req := func(token string) *ut.ResponseRecorder {
		headers := []ut.Header{{Key: "Content-Type", Value: "application/json"}}
		if token != "" {
			headers = append(headers, ut.Header{Key: "Authorization", Value: "Bearer " + token})
		}
		return ut.PerformRequest(h.Engine, "POST", "/api/v1/compliances",
			&ut.Body{Body: bytes.NewBufferString(body), Len: len(body)}, headers...)
	}

	// Public login without a token: 200.
	w := ut.PerformRequest(h.Engine, "POST", "/api/v1/auth/login",
		&ut.Body{Body: bytes.NewBufferString(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	if w.Code != 200 {
		t.Fatalf("public login status = %d, want 200", w.Code)
	}

	// No token -> 401.
	if w := req(""); w.Code != 401 {
		t.Fatalf("no token status = %d, want 401", w.Code)
	}

	// Garbage token -> 401.
	if w := req("not-a-real-token"); w.Code != 401 {
		t.Fatalf("bad token status = %d, want 401", w.Code)
	}

	// Valid viewer token on a write route -> 403 (viewer lacks the permission).
	if w := req(issue("viewer")); w.Code != 403 {
		t.Fatalf("viewer write status = %d, want 403 (body %s)", w.Code, w.Body.String())
	}

	// Valid admin token -> 200 (admin bypasses the permission check).
	w = req(issue("admin"))
	if w.Code != 200 {
		t.Fatalf("admin write status = %d, want 200 (body %s)", w.Code, w.Body.String())
	}
}
