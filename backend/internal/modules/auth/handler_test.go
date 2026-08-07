package authmodule

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

	"github.com/aeroxe/compliance-hub/backend/internal/auth"
	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/config"
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

func newLoginEnv(t *testing.T) (*server.Hertz, *gorm.DB, *config.Config) {
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
	cfg := &config.Config{JWTSecret: "test-secret", JWTExpiry: time.Hour}
	d := deps.Deps{DB: db, Cache: cache.New(ctx, "")}

	role := models.Role{Name: "Viewer", Code: "viewer"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("role: %v", err)
	}
	hash, _ := auth.HashPassword("correct-password")
	user := models.User{
		Username: "viewer1", PasswordHash: hash,
		RoleID: role.ID, Status: "active",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("user: %v", err)
	}

	h := server.New()
	RegisterRoutes(h.Group("/api/v1"), d, cfg)
	return h, db, cfg
}

func postLogin(t *testing.T, h *server.Hertz, body string) (int, map[string]any) {
	t.Helper()
	w := ut.PerformRequest(h.Engine, "POST", "/api/v1/auth/login",
		&ut.Body{Body: bytes.NewBufferString(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
		ut.Header{Key: "User-Agent", Value: "test-agent/1.0"})
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func countLogs(db *gorm.DB, status string) int64 {
	var n int64
	db.Model(&models.AuditLog{}).Where("action = ? AND status = ?", "login", status).Count(&n)
	return n
}

func TestLoginFailureIsLogged(t *testing.T) {
	h, db, _ := newLoginEnv(t)

	code, _ := postLogin(t, h, `{"username":"viewer1","password":"wrong-password"}`)
	if code != 401 {
		t.Fatalf("login status = %d, want 401", code)
	}
	if n := countLogs(db, "failure"); n != 1 {
		t.Fatalf("failure logs = %d, want 1", n)
	}

	var row models.AuditLog
	if err := db.Where("action = ? AND status = ?", "login", "failure").First(&row).Error; err != nil {
		t.Fatalf("load failure log: %v", err)
	}
	if row.UserAgent != "test-agent/1.0" {
		t.Errorf("user_agent = %q, want test-agent/1.0", row.UserAgent)
	}
	var meta map[string]any
	_ = json.Unmarshal(row.Metadata, &meta)
	if meta["reason"] != "invalid password" {
		t.Errorf("metadata reason = %v, want invalid password", meta["reason"])
	}
}

func TestLoginUnknownUserIsLogged(t *testing.T) {
	h, db, _ := newLoginEnv(t)

	code, _ := postLogin(t, h, `{"username":"ghost","password":"whatever"}`)
	if code != 401 {
		t.Fatalf("login status = %d, want 401", code)
	}
	var row models.AuditLog
	if err := db.Where("action = ? AND status = ?", "login", "failure").First(&row).Error; err != nil {
		t.Fatalf("load failure log: %v", err)
	}
	if row.ActorID != "ghost" {
		t.Errorf("actor = %q, want attempted username ghost", row.ActorID)
	}
}

func TestLoginSuccessIsLogged(t *testing.T) {
	h, db, _ := newLoginEnv(t)

	code, out := postLogin(t, h, `{"username":"viewer1","password":"correct-password"}`)
	if code != 200 {
		t.Fatalf("login status = %d, body = %v", code, out)
	}
	if n := countLogs(db, "success"); n != 1 {
		t.Fatalf("success logs = %d, want 1", n)
	}

	var row models.AuditLog
	if err := db.Where("action = ? AND status = ?", "login", "success").First(&row).Error; err != nil {
		t.Fatalf("load success log: %v", err)
	}
	if row.EntityType != "user" || row.EntityID == uuid.Nil {
		t.Errorf("entity = %s %s, want user with id", row.EntityType, row.EntityID)
	}
	var meta map[string]any
	_ = json.Unmarshal(row.Metadata, &meta)
	if meta["role_code"] != "viewer" {
		t.Errorf("metadata role_code = %v, want viewer", meta["role_code"])
	}
}
