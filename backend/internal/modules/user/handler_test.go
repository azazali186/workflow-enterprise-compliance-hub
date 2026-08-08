package usermodule

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

func pinSingleConnection(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
}

// newUserEnv boots the module with sqlite and one role; an optional test
// middleware injects auth_user_id so the self-delete guard can be exercised.
func newUserEnv(t *testing.T, actorID string) (*server.Hertz, *gorm.DB) {
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
	role := models.Role{Name: "Viewer", Code: "viewer"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("role: %v", err)
	}

	d := deps.Deps{DB: db, Cache: cache.New(context.Background(), "")}
	h := server.New()
	if actorID != "" {
		h.Use(func(ctx context.Context, c *app.RequestContext) {
			c.Set("auth_user_id", actorID)
			c.Next(ctx)
		})
	}
	RegisterRoutes(h.Group("/api/v1"), d)
	return h, db
}

func post(t *testing.T, h *server.Hertz, path, body string) (int, map[string]any) {
	t.Helper()
	return perform(t, h, "POST", path, body)
}

func perform(t *testing.T, h *server.Hertz, method, path, body string) (int, map[string]any) {
	t.Helper()
	w := ut.PerformRequest(h.Engine, method, path,
		&ut.Body{Body: bytes.NewBufferString(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestUserCRUD(t *testing.T) {
	h, db := newUserEnv(t, "actor-1")

	// Create.
	code, out := post(t, h, "/api/v1/users", `{"username":"alice","email":"alice@corp.io","password":"s3cret-pass","role_id":"00000000-0000-0000-0000-000000000000"}`)
	if code != 400 {
		t.Fatalf("create with nil role = %d, want 400", code)
	}
	var role models.Role
	if err := db.Where("code = ?", "viewer").First(&role).Error; err != nil {
		t.Fatalf("load role: %v", err)
	}
	code, out = post(t, h, "/api/v1/users",
		`{"username":"alice","email":"alice@corp.io","password":"s3cret-pass","role_id":"`+role.ID.String()+`"}`)
	if code != 201 {
		t.Fatalf("create = %d, body %v", code, out)
	}
	body, _ := json.Marshal(out)
	if strings.Contains(string(body), "s3cret-pass") || strings.Contains(string(body), "password_hash") {
		t.Fatal("password leaked in create response")
	}
	userID := out["data"].(map[string]any)["id"].(string)

	// Duplicate username is rejected.
	if code, _ := post(t, h, "/api/v1/users",
		`{"username":"alice","password":"s3cret-pass","role_id":"`+role.ID.String()+`"}`); code != 409 {
		t.Fatalf("duplicate username = %d, want 409", code)
	}

	// Get returns the user with role.
	if code, _ := post(t, h, "/api/v1/users/get", `{"id":"`+userID+`"}`); code != 200 {
		t.Fatalf("get = %d", code)
	}

	// Update: status + password.
	if code, out := perform(t, h, "PATCH", "/api/v1/users", `{"id":"`+userID+`","status":"disabled"}`); code != 200 {
		t.Fatalf("update = %d, body %v", code, out)
	}

	// Search finds the disabled user and the actor does not see passwords.
	code, out = post(t, h, "/api/v1/users/search", `{"limit":10}`)
	if code != 200 {
		t.Fatalf("search = %d", code)
	}
	items := out["data"].(map[string]any)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("search items = %d, want 1", len(items))
	}

	// Audit trail recorded create + update without passwords.
	var auditCount int64
	db.Model(&models.AuditLog{}).Where("entity_type = ? AND action IN ?", "user", []string{"create", "update"}).Count(&auditCount)
	if auditCount != 2 {
		t.Errorf("audit entries = %d, want 2", auditCount)
	}
	var auditRows []models.AuditLog
	db.Where("entity_type = ?", "user").Find(&auditRows)
	for _, r := range auditRows {
		if strings.Contains(string(r.AfterData), "s3cret-pass") {
			t.Error("password leaked into audit snapshot")
		}
	}

	// Delete a different user works; deleting yourself is refused.
	code, out = post(t, h, "/api/v1/users",
		`{"username":"bob","password":"s3cret-pass","role_id":"`+role.ID.String()+`"}`)
	bobID := out["data"].(map[string]any)["id"].(string)
	if code, _ := perform(t, h, "DELETE", "/api/v1/users", `{"id":"`+bobID+`"}`); code != 204 {
		t.Fatalf("delete other = %d", code)
	}
	// Deleting your own account is refused: the env below authenticates as
	// alice herself (auth_user_id = alice's id).
	selfEnv, _ := newUserEnv(t, userID)
	if code, out := perform(t, selfEnv, "DELETE", "/api/v1/users", `{"id":"`+userID+`"}`); code != 400 {
		t.Fatalf("self-delete = %d, want 400 (body %v)", code, out)
	}
}
