package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Report represents a generated compliance report.
type Report struct {
	Base
	Title        string         `gorm:"size:255;not null;index" json:"title" vd:"$ != ''"`
	Type         string         `gorm:"size:64;not null;default:summary;index" json:"type"`
	Status       string         `gorm:"size:32;not null;default:generated;index" json:"status"`
	Description  string         `gorm:"type:text" json:"description"`
	ComplianceID uuid.UUID      `gorm:"type:uuid;index" json:"compliance_id"`
	Data         datatypes.JSON `gorm:"type:jsonb" json:"data"`
	GeneratedAt  *time.Time     `json:"generated_at"`
}
