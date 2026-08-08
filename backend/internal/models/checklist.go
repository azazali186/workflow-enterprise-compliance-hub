package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Checklist represents a compliance checklist associated with an entity.
type Checklist struct {
	Base
	Title        string         `gorm:"size:255;not null;index" json:"title" vd:"$ != ''"`
	Description  string         `gorm:"type:text" json:"description"`
	Status       string         `gorm:"size:32;not null;default:open;index" json:"status"`
	ComplianceID uuid.UUID      `gorm:"type:uuid;index" json:"compliance_id"`
	OwnerID      string         `gorm:"size:64;index" json:"owner_id"`
	DueDate      *time.Time     `json:"due_date"`
	Items        datatypes.JSON `gorm:"type:jsonb" json:"items"`
}
