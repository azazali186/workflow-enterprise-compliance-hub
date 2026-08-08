package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Compliance represents a compliance requirement being monitored.
type Compliance struct {
	Base
	Name           string         `gorm:"size:255;not null;index" json:"name" vd:"$ != ''"`
	Description    string         `gorm:"type:text" json:"description"`
	Status         string         `gorm:"size:32;not null;default:draft;index" json:"status"`
	RiskLevel      string         `gorm:"size:16;default:medium" json:"risk_level"`
	OwnerID        string         `gorm:"size:64;index" json:"owner_id"`
	RegulationID   uuid.UUID      `gorm:"type:uuid;index" json:"regulation_id"`
	DueDate        *time.Time     `json:"due_date"`
	LastReviewedAt *time.Time     `json:"last_reviewed_at"`
	Metadata       datatypes.JSON `gorm:"type:jsonb" json:"metadata"`
}
