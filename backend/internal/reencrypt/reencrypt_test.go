package reencrypt

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/aeroxe/compliance-hub/backend/internal/crypto"
	"github.com/aeroxe/compliance-hub/backend/internal/lock"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/outbox"
)

// keyHex returns the hex form of a deterministic 32-byte key.
func keyHex(seed byte) string {
	b := make([]byte, 32)
	for i := range b {
		b[i] = seed + byte(i)
	}
	return hex.EncodeToString(b)
}

// encryptedRow is a stand-in for any entity carrying a models.Secret column.
type encryptedRow struct {
	ID    uuid.UUID     `gorm:"type:uuid;primaryKey"`
	Token models.Secret `gorm:"type:text"`
}

func pinSingleConnection(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
}

func newMigratorEnv(t *testing.T) (*gorm.DB, *Migrator) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	pinSingleConnection(t, db)
	if err := db.AutoMigrate(&models.OutboxEvent{}, &encryptedRow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db, New(db)
}

func TestOutboxMigratedToCurrentKey(t *testing.T) {
	oldKey, newKey := keyHex(0x10), keyHex(0x20)
	db, m := newMigratorEnv(t)
	ctx := context.Background()
	defer func() { _ = crypto.SetDefault("") }()

	// Seed a row while the OLD key is current.
	if err := crypto.Setup(oldKey); err != nil {
		t.Fatalf("Setup(old): %v", err)
	}
	if _, err := outbox.Insert(ctx, db, "compliance.created", "compliance.created",
		map[string]any{"name": "GDPR", "secret": "s3cr3t"}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Rotate: new key current, old key readable.
	if err := crypto.Setup(newKey, oldKey); err != nil {
		t.Fatalf("Setup(new, old): %v", err)
	}

	n, err := m.MigrateOutbox(ctx)
	if err != nil {
		t.Fatalf("MigrateOutbox: %v", err)
	}
	if n != 1 {
		t.Fatalf("migrated = %d, want 1", n)
	}

	// The stored payload now uses the current key…
	var row models.OutboxEvent
	if err := db.First(&row).Error; err != nil {
		t.Fatalf("load row: %v", err)
	}
	var enc string
	if err := json.Unmarshal(row.Payload, &enc); err != nil {
		t.Fatalf("payload is not a JSON string: %v", err)
	}
	if !crypto.UsesCurrentKey(enc) {
		t.Fatalf("payload %q not under the current key", enc[:min(len(enc), 24)])
	}

	// …and still decrypts to the original payload (dispatcher path).
	plain, err := crypto.DecryptString(enc)
	if err != nil {
		t.Fatalf("decrypt migrated payload: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(plain), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["secret"] != "s3cr3t" {
		t.Errorf("payload secret = %v, want s3cr3t", payload["secret"])
	}

	// Idempotent: nothing left to migrate.
	if n2, err := m.MigrateOutbox(ctx); err != nil || n2 != 0 {
		t.Errorf("second run = %d, %v; want 0, nil", n2, err)
	}
}

func TestSecretColumnMigrated(t *testing.T) {
	oldKey, newKey := keyHex(0x30), keyHex(0x40)
	db, m := newMigratorEnv(t)
	ctx := context.Background()
	defer func() { _ = crypto.SetDefault("") }()

	// Seed with the OLD key current.
	if err := crypto.Setup(oldKey); err != nil {
		t.Fatalf("Setup(old): %v", err)
	}
	seeded := []encryptedRow{
		{ID: mustV7(t), Token: models.NewSecret("tok_live_alpha")},
		{ID: mustV7(t), Token: models.NewSecret("tok_live_beta")},
	}
	for _, r := range seeded {
		if err := db.Create(&r).Error; err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	if err := crypto.Setup(newKey, oldKey); err != nil {
		t.Fatalf("Setup(new, old): %v", err)
	}

	n, err := m.MigrateColumn(ctx, &encryptedRow{}, "token")
	if err != nil {
		t.Fatalf("MigrateColumn: %v", err)
	}
	if n != 2 {
		t.Fatalf("migrated = %d, want 2", n)
	}

	// Raw column now uses the current key and still decrypts via Secret.Scan.
	var probe struct {
		Token string
	}
	if err := db.Model(&encryptedRow{}).First(&probe).Error; err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !crypto.UsesCurrentKey(probe.Token) {
		t.Fatalf("stored token %q not under the current key", probe.Token[:min(len(probe.Token), 24)])
	}
	var rows []encryptedRow
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("load rows: %v", err)
	}
	for i, r := range rows {
		if r.Token.String() != seeded[i].Token.String() {
			t.Errorf("row %d decrypted = %q, want %q", i, r.Token.String(), seeded[i].Token.String())
		}
	}

	// Idempotent.
	if n2, err := m.MigrateColumn(ctx, &encryptedRow{}, "token"); err != nil || n2 != 0 {
		t.Errorf("second run = %d, %v; want 0, nil", n2, err)
	}
}

func TestPlaintextRowsLeftUntouched(t *testing.T) {
	oldKey, newKey := keyHex(0x50), keyHex(0x60)
	db, m := newMigratorEnv(t)
	ctx := context.Background()
	defer func() { _ = crypto.SetDefault("") }()

	if err := crypto.Setup(oldKey); err != nil {
		t.Fatalf("Setup(old): %v", err)
	}
	// An old-key row and a legacy plaintext (JSON object) row.
	if _, err := outbox.Insert(ctx, db, "a.subject", "a.event", map[string]any{"n": 1}); err != nil {
		t.Fatalf("insert old: %v", err)
	}
	plainRow := models.OutboxEvent{
		ID:        mustV7(t),
		Subject:   "b.subject",
		EventType: "b.event",
		Payload:   []byte(`{"n":2}`),
	}
	if err := db.Create(&plainRow).Error; err != nil {
		t.Fatalf("create plaintext row: %v", err)
	}

	if err := crypto.Setup(newKey, oldKey); err != nil {
		t.Fatalf("Setup(new, old): %v", err)
	}

	n, err := m.MigrateOutbox(ctx)
	if err != nil {
		t.Fatalf("MigrateOutbox: %v", err)
	}
	if n != 1 {
		t.Fatalf("migrated = %d, want 1 (plaintext row untouched)", n)
	}

	var raw models.OutboxEvent
	if err := db.Where("subject = ?", "b.subject").First(&raw).Error; err != nil {
		t.Fatalf("load plaintext row: %v", err)
	}
	if string(raw.Payload) != `{"n":2}` {
		t.Errorf("plaintext row payload changed: %q", string(raw.Payload))
	}
}

func TestRunUnderLockMigratesAndSkipsCurrent(t *testing.T) {
	oldKey, newKey := keyHex(0x70), keyHex(0x80)
	db, m := newMigratorEnv(t)
	ctx := context.Background()
	defer func() { _ = crypto.SetDefault("") }()

	if err := crypto.Setup(oldKey); err != nil {
		t.Fatalf("Setup(old): %v", err)
	}
	if _, err := outbox.Insert(ctx, db, "c.subject", "c.event", map[string]any{"x": 1}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := db.Create(&encryptedRow{ID: mustV7(t), Token: models.NewSecret("sk_live_9")}).Error; err != nil {
		t.Fatalf("create secret row: %v", err)
	}

	if err := crypto.Setup(newKey, oldKey); err != nil {
		t.Fatalf("Setup(new, old): %v", err)
	}

	total, err := m.Run(ctx, lock.New(ctx, ""), ColumnSpec{Model: &encryptedRow{}, Column: "token"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if total != 2 {
		t.Fatalf("Run migrated = %d, want 2 (outbox + column)", total)
	}

	// A second Run is a no-op (idempotent).
	total2, err := m.Run(ctx, lock.New(ctx, ""), ColumnSpec{Model: &encryptedRow{}, Column: "token"})
	if err != nil || total2 != 0 {
		t.Errorf("second Run = %d, %v; want 0, nil", total2, err)
	}
}

func mustV7(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("NewV7: %v", err)
	}
	return id
}
