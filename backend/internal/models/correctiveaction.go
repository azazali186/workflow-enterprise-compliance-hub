package models

import (
	"time"

	"github.com/google/uuid"
)

// CorrectiveAction represents the remediation plan for a violation.
type CorrectiveAction struct {
	Base
	Title       string     `gorm:"size:255;not null;index" json:"title" vd:"$ != ''"`
	Description string     `gorm:"type:text" json:"description"`
	Status      string     `gorm:"size:32;not null;default:open;index" json:"status"`
	ViolationID uuid.UUID  `gorm:"type:uuid;index" json:"violation_id"`
	OwnerID     string     `gorm:"size:64;index" json:"owner_id"`
	DueDate     *time.Time `json:"due_date"`
	CompletedAt *time.Time `json:"completed_at"`
}
