package models

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AuditLog records who did what, when, for which entity, with full before/
// after snapshots and a field-level change diff. Every mutating operation
// (CRUD, lifecycle actions, logins) writes an entry, so the trail is a
// complete, queryable history.
//
// Snapshots and metadata are redacted before persistence (see
// internal/auditlog): values under sensitive keys such as password, token or
// secret are stored as "[REDACTED]" so the trail never leaks secrets.
type AuditLog struct {
	Base
	Action     string         `gorm:"size:64;not null;index" json:"action"`
	Status     string         `gorm:"size:16;not null;default:success;index" json:"status"`
	EntityType string         `gorm:"size:64;index" json:"entity_type"`
	EntityID   uuid.UUID      `gorm:"type:uuid;index" json:"entity_id"`
	ActorID    string         `gorm:"size:64;index" json:"actor_id"`
	IP         string         `gorm:"size:64" json:"ip"`
	UserAgent  string         `gorm:"size:255" json:"user_agent"`
	BeforeData datatypes.JSON `gorm:"type:jsonb" json:"before_data,omitempty"`
	AfterData  datatypes.JSON `gorm:"type:jsonb" json:"after_data,omitempty"`
	Changes    datatypes.JSON `gorm:"type:jsonb" json:"changes,omitempty"`
	Metadata   datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"`
}
