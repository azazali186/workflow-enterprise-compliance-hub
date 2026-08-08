package models

import (
	"time"

	"github.com/google/uuid"
)

// User is an authenticated account bound to a Role.
//
// Passwords are never stored in plaintext: PasswordHash is a bcrypt hash
// (json:"-" keeps it out of every API payload and audit snapshot), and the
// single-session token stored in Redis is an irreversible md5 fingerprint of
// the JWT, never the token itself.
type User struct {
	Base
	Username     string     `gorm:"size:64;not null;uniqueIndex" json:"username" vd:"$ != ''"`
	Email        string     `gorm:"size:128;index" json:"email"`
	PasswordHash string     `gorm:"size:255;not null" json:"-"`
	RoleID       uuid.UUID  `gorm:"type:uuid;index" json:"role_id"`
	Role         *Role      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"role,omitempty"`
	Status       string     `gorm:"size:32;not null;default:active" json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}
