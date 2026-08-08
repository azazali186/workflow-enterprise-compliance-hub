package models

import (
	"time"

	"github.com/google/uuid"
)

// Violation represents a detected compliance violation.
type Violation struct {
	Base
	Title        string     `gorm:"size:255;not null;index" json:"title" vd:"$ != ''"`
	Description  string     `gorm:"type:text" json:"description"`
	Severity     string     `gorm:"size:16;not null;default:medium;index" json:"severity"`
	Status       string     `gorm:"size:32;not null;default:open;index" json:"status"`
	ComplianceID uuid.UUID  `gorm:"type:uuid;index" json:"compliance_id"`
	RegulationID uuid.UUID  `gorm:"type:uuid;index" json:"regulation_id"`
	DetectedAt   *time.Time `json:"detected_at"`
	ResolvedAt   *time.Time `json:"resolved_at"`
}
