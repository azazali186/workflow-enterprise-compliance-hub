package models

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// secretHolder is a tiny entity that carries a Secret column, standing in for
// any model that stores sensitive data at rest.
type secretHolder struct {
	ID     uint `gorm:"primaryKey"`
	Handle string
	Token  Secret `gorm:"type:text"`
}

func TestSecretEncryptsAtRest(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&secretHolder{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	const secret = "sk_live_super_secret_value"
	if err := db.WithContext(context.Background()).Create(&secretHolder{Handle: "h1", Token: NewSecret(secret)}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	// Peek at the raw column through a plain-string probe struct (no Secret.Scan
	// involved) so we can assert the stored bytes are ciphertext.
	var probe struct {
		Token string
	}
	if err := db.Model(&secretHolder{}).First(&probe).Error; err != nil {
		t.Fatalf("probe: %v", err)
	}
	// Versioned ciphertext prefix: enc:k<keyID>: (see internal/crypto).
	if !strings.HasPrefix(probe.Token, "enc:k") {
		t.Errorf("stored value %q is not encrypted (missing versioned enc:k prefix)", probe.Token)
	}
	if strings.Contains(probe.Token, secret) {
		t.Fatal("plaintext leaked into the stored column")
	}

	// Reading through GORM decrypts transparently.
	var row secretHolder
	if err := db.First(&row).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if row.Token.String() != secret {
		t.Errorf("decrypted = %q, want %q", row.Token.String(), secret)
	}

	// Searching by plaintext must find nothing — the column holds ciphertext.
	var count int64
	if err := db.Model(&secretHolder{}).Where("token = ?", secret).Count(&count).Error; err != nil {
		t.Fatalf("plaintext search: %v", err)
	}
	if count != 0 {
		t.Error("plaintext found via SQL — column is not encrypted")
	}
}

func TestSecretEmptyValue(t *testing.T) {
	s := NewSecret("")
	v, err := s.Value()
	if err != nil || v != "" {
		t.Errorf("empty Value = %v, %v; want empty", v, err)
	}
}
