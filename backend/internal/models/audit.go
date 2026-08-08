package models

import (
	"time"

	"github.com/google/uuid"
)

// Audit represents a scheduled or executed compliance audit.
type Audit struct {
	Base
	Title        string     `gorm:"size:255;not null;index" json:"title" vd:"$ != ''"`
	Description  string     `gorm:"type:text" json:"description"`
	Status       string     `gorm:"size:32;not null;default:scheduled;index" json:"status"`
	ComplianceID uuid.UUID  `gorm:"type:uuid;index" json:"compliance_id"`
	AuditorID    string     `gorm:"size:64;index" json:"auditor_id"`
	ScheduledAt  *time.Time `json:"scheduled_at"`
	StartedAt    *time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
}
