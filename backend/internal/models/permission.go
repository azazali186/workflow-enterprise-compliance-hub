package models

// Permission is an API route permission entry, synced from the registered
// HTTP routes (see internal/permissions). Route is the unique "METHOD path"
// key consumed by the API gateway's authorization layer.
type Permission struct {
	Base
	Name    string `gorm:"size:255;not null" json:"name"`
	Route   string `gorm:"size:255;not null;uniqueIndex" json:"route"`
	Path    string `gorm:"size:255;not null;index" json:"path"`
	Service string `gorm:"size:64;not null;default:api-gateway;index" json:"service"`
	Method  string `gorm:"size:16;index" json:"method"`
}
