package models

import (
	"time"

	"github.com/google/uuid"
)

// Alert represents a compliance alert raised for an entity.
type Alert struct {
	Base
	Type           string     `gorm:"size:64;not null;index" json:"type" vd:"$ != ''"`
	Title          string     `gorm:"size:255;not null" json:"title" vd:"$ != ''"`
	Message        string     `gorm:"type:text" json:"message"`
	Severity       string     `gorm:"size:16;not null;default:medium;index" json:"severity"`
	Status         string     `gorm:"size:32;not null;default:open;index" json:"status"`
	EntityType     string     `gorm:"size:64;index" json:"entity_type"`
	EntityID       uuid.UUID  `gorm:"type:uuid;index" json:"entity_id"`
	AcknowledgedBy string     `gorm:"size:64" json:"acknowledged_by"`
	ResolvedAt     *time.Time `json:"resolved_at"`
}
