// Package models defines the full database schema for ComplianceHub.
//
// Every entity lives in its own file and follows the README contract: UUID v7
// primary keys, timestamps and soft delete (gorm.DeletedAt). All persistence
// is performed through GORM — no raw SQL is used anywhere in this codebase.
// Sensitive columns use the Secret type, which encrypts values at rest with
// AES-256-GCM (see secret.go and internal/crypto).
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Base is embedded by every entity and provides UUID v7, timestamps and soft
// delete.
type Base struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// GetID returns the entity's UUID. A value receiver keeps the method in the
// value method set so models satisfy the Entity constraint.
func (b Base) GetID() uuid.UUID { return b.ID }

// BeforeCreate generates a time-ordered UUID v7 when not supplied by the
// caller. UUID v7 values sort chronologically, which keeps the primary-key
// index append-only under write-heavy workloads.
func (b *Base) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		b.ID = id
	}
	return nil
}

// Entity is the minimal contract satisfied by all persisted models.
type Entity interface {
	GetID() uuid.UUID
}
