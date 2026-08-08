package auditlog

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

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

func TestDiffScalarChanges(t *testing.T) {
	before := models.Compliance{
		Base:      models.Base{ID: uuid.New()},
		Name:      "GDPR review",
		Status:    "draft",
		RiskLevel: "medium",
	}
	after := models.Compliance{
		Base:      models.Base{ID: uuid.New()},
		Name:      "GDPR review",
		Status:    "active",
		RiskLevel: "high",
	}

	changes := Diff(&before, &after)

	// Status and risk_level changed.
	if c, ok := changes["status"]; !ok || c.Before != "draft" || c.After != "active" {
		t.Errorf("status change = %+v, want draft->active", changes["status"])
	}
	if c, ok := changes["risk_level"]; !ok || c.Before != "medium" || c.After != "high" {
		t.Errorf("risk_level change = %+v, want medium->high", changes["risk_level"])
	}
	// Unchanged field omitted.
	if _, ok := changes["name"]; ok {
		t.Error("unchanged field 'name' must not appear in the diff")
	}
	// System columns never appear.
	for _, key := range []string{"id", "created_at", "updated_at", "deleted_at"} {
		if _, ok := changes[key]; ok {
			t.Errorf("system column %q leaked into the diff", key)
		}
	}
}

func TestDiffNilSides(t *testing.T) {
	// Create: no before.
	changes := Diff(nil, &models.Compliance{Name: "New", Status: "draft"})
	if _, ok := changes["name"]; !ok {
		t.Error("create diff should include the new fields")
	}
	if changes["name"].Before != nil {
		t.Errorf("create before = %v, want nil", changes["name"].Before)
	}

	// Delete: no after.
	changes = Diff(&models.Compliance{Name: "Gone", Status: "active"}, nil)
	if _, ok := changes["status"]; !ok {
		t.Error("delete diff should include removed fields")
	}
	if changes["status"].After != nil {
		t.Errorf("delete after = %v, want nil", changes["status"].After)
	}
}

func TestDiffJSONBByValue(t *testing.T) {
	mk := func(m map[string]any) *models.Compliance {
		b, _ := json.Marshal(m)
		return &models.Compliance{Name: "X", Status: "active", Metadata: b}
	}

	// Identical jsonb content on both sides => no change.
	changes := Diff(mk(map[string]any{"owner": "alice", "tags": []any{"a", "b"}}), mk(map[string]any{"tags": []any{"a", "b"}, "owner": "alice"}))
	if len(changes) != 0 {
		t.Errorf("identical jsonb produced changes: %+v", changes)
	}

	// Different jsonb content => change recorded.
	changes = Diff(mk(map[string]any{"owner": "alice"}), mk(map[string]any{"owner": "bob"}))
	if _, ok := changes["metadata"]; !ok {
		t.Error("metadata change not recorded")
	}
}

func TestRecordPersistsSnapshotsAndDiff(t *testing.T) {
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

	id := uuid.New()
	before := &models.Compliance{Base: models.Base{ID: id}, Name: "A", Status: "draft"}
	after := &models.Compliance{Base: models.Base{ID: id}, Name: "A", Status: "active"}

	err = Record(context.Background(), db, Entry{
		Action:     "update",
		EntityType: "compliance",
		EntityID:   id,
		ActorID:    "user-1",
		IP:         "10.0.0.1",
		UserAgent:  "test-agent",
		Before:     before,
		After:      after,
		Metadata:   map[string]any{"reason": "review"},
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	var row models.AuditLog
	if err := db.First(&row).Error; err != nil {
		t.Fatalf("load row: %v", err)
	}
	if row.Action != "update" || row.Status != "success" || row.EntityType != "compliance" || row.ActorID != "user-1" {
		t.Errorf("row basics = %+v", row)
	}
	if row.IP != "10.0.0.1" || row.UserAgent != "test-agent" {
		t.Errorf("row request info = ip %q ua %q", row.IP, row.UserAgent)
	}

	var beforeMap, afterMap map[string]any
	_ = json.Unmarshal(row.BeforeData, &beforeMap)
	_ = json.Unmarshal(row.AfterData, &afterMap)
	if beforeMap["status"] != "draft" || afterMap["status"] != "active" {
		t.Errorf("snapshots = before %v after %v", beforeMap["status"], afterMap["status"])
	}

	var changes map[string]Change
	_ = json.Unmarshal(row.Changes, &changes)
	if c, ok := changes["status"]; !ok || c.Before != "draft" || c.After != "active" {
		t.Errorf("persisted changes = %+v, want status draft->active", changes)
	}
}

func TestDiffRedactsSensitiveKeys(t *testing.T) {
	// A password added between snapshots surfaces as [REDACTED] — never the value.
	changes := Diff(
		map[string]any{"name": "api"},
		map[string]any{"name": "api", "password": "hunter2"},
	)
	c, ok := changes["password"]
	if !ok {
		t.Fatal("password change not recorded")
	}
	if c.Before != nil || c.After != "[REDACTED]" {
		t.Errorf("password change = %+v, want nil -> [REDACTED]", c)
	}

	// A nested token that changed on both sides redacts to the same value, so
	// the parent field must NOT surface as a change (its only difference is a
	// secret that must never reach the trail).
	hasChange := Diff(
		map[string]any{"config": map[string]any{"access_token": "a", "owner": "alice"}},
		map[string]any{"config": map[string]any{"access_token": "b", "owner": "alice"}},
	)
	if _, ok := hasChange["config"]; ok {
		t.Error("config must not appear — its only difference is a redacted token")
	}
}

func TestRedactCatchesCamelCaseAndSparesLegitimateKeys(t *testing.T) {
	snap := map[string]any{
		"accessToken":    "jwt-abc",
		"passwordHash":   "$2a$10$...",
		"bypass_reason":  "control bypassed", // must survive — only the bare "pass"
		"key":            "document-42",      // bare "key" must survive
		"compliance_key": "id-7",             // qualified with _key but not a secret form
	}
	redacted := redact(toMap(snap)).(map[string]any)
	if redacted["accessToken"] != "[REDACTED]" {
		t.Errorf("accessToken = %v, want [REDACTED]", redacted["accessToken"])
	}
	if redacted["passwordHash"] != "[REDACTED]" {
		t.Errorf("passwordHash = %v, want [REDACTED]", redacted["passwordHash"])
	}
	if redacted["bypass_reason"] != "control bypassed" {
		t.Errorf("bypass_reason = %v, want unchanged", redacted["bypass_reason"])
	}
	if redacted["key"] != "document-42" {
		t.Errorf("key = %v, want unchanged", redacted["key"])
	}
	if redacted["compliance_key"] != "id-7" {
		t.Errorf("compliance_key = %v, want unchanged", redacted["compliance_key"])
	}
}

func TestRecordRedactsMetadata(t *testing.T) {
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

	if err := Record(context.Background(), db, Entry{
		Action:   "login",
		Status:   "failure",
		Metadata: map[string]any{"username": "alice", "password": "s3cr3t", "nested": map[string]any{"api_key": "ak-live-123"}},
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	var row models.AuditLog
	if err := db.First(&row).Error; err != nil {
		t.Fatalf("load row: %v", err)
	}
	var meta map[string]any
	_ = json.Unmarshal(row.Metadata, &meta)
	if meta["username"] != "alice" {
		t.Errorf("username = %v, want alice (non-sensitive key must survive)", meta["username"])
	}
	if meta["password"] != "[REDACTED]" {
		t.Errorf("password = %v, want [REDACTED]", meta["password"])
	}
	nested, ok := meta["nested"].(map[string]any)
	if !ok || nested["api_key"] != "[REDACTED]" {
		t.Errorf("nested api_key = %v, want [REDACTED]", meta["nested"])
	}
}

func TestRecordFailureStatus(t *testing.T) {
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
	if err := Record(context.Background(), db, Entry{
		Action:   "login",
		Status:   "failure",
		Metadata: map[string]any{"username": "nobody"},
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	var row models.AuditLog
	if err := db.First(&row).Error; err != nil {
		t.Fatalf("load row: %v", err)
	}
	if row.Status != "failure" {
		t.Errorf("status = %q, want failure", row.Status)
	}
}
