package models

import "time"

// Regulation represents a regulatory requirement (law, standard, policy).
type Regulation struct {
	Base
	Title         string     `gorm:"size:255;not null;index" json:"title" vd:"$ != ''"`
	Code          string     `gorm:"size:64;not null;uniqueIndex" json:"code" vd:"$ != ''"`
	Description   string     `gorm:"type:text" json:"description"`
	Jurisdiction  string     `gorm:"size:128;index" json:"jurisdiction"`
	Status        string     `gorm:"size:32;not null;default:active;index" json:"status"`
	EffectiveDate *time.Time `json:"effective_date"`
	ExpiryDate    *time.Time `json:"expiry_date"`
}
