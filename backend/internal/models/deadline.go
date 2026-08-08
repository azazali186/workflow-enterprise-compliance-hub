package models

import (
	"time"

	"github.com/google/uuid"
)

// Deadline represents an approaching or missed compliance deadline.
type Deadline struct {
	Base
	Title       string     `gorm:"size:255;not null;index" json:"title" vd:"$ != ''"`
	Description string     `gorm:"type:text" json:"description"`
	Status      string     `gorm:"size:32;not null;default:upcoming;index" json:"status"`
	EntityType  string     `gorm:"size:64;index" json:"entity_type"`
	EntityID    uuid.UUID  `gorm:"type:uuid;index" json:"entity_id"`
	DeadlineAt  time.Time  `gorm:"not null;index" json:"deadline_at"`
	CompletedAt *time.Time `json:"completed_at"`
}
