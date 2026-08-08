package models

// Role is an RBAC role. Permissions are linked through the role_permissions
// join table (many-to-many).
type Role struct {
	Base
	Name        string       `gorm:"size:64;not null" json:"name" vd:"$ != ''"`
	Code        string       `gorm:"size:64;not null;uniqueIndex" json:"code" vd:"$ != ''"`
	Description string       `gorm:"type:text" json:"description"`
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
}
