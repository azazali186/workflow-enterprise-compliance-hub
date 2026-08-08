package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// OutboxEvent is a reliably-queued domain event (transactional outbox). The
// background dispatcher publishes rows with published_at = NULL to the bus
// and marks them delivered, retrying with backoff when the bus is down.
//
// Payload is encrypted at rest with AES-256-GCM (internal/crypto); the
// dispatcher decrypts it immediately before publishing to the bus, so
// consumers always see plaintext while the database stores only ciphertext.
type OutboxEvent struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Subject     string         `gorm:"size:128;not null;index" json:"subject"`
	EventType   string         `gorm:"size:64;not null" json:"event_type"`
	Payload     datatypes.JSON `gorm:"type:jsonb" json:"payload"`
	CreatedAt   time.Time      `json:"created_at"`
	PublishedAt *time.Time     `gorm:"index" json:"published_at,omitempty"`
	Attempts    int            `gorm:"not null;default:0" json:"attempts"`
	LastError   string         `gorm:"type:text" json:"last_error,omitempty"`
}
