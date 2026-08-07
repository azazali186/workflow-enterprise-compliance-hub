// Package models defines the full database schema for ComplianceHub.
//
// Every entity follows the README contract: UUID primary keys, timestamps and
// soft delete (gorm.DeletedAt). All persistence is performed through GORM —
// no raw SQL is used anywhere in this codebase.
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Base is embedded by every entity and provides UUID, timestamps and soft delete.
type Base struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// GetID returns the entity's UUID. A value receiver keeps the method in the
// value method set so models satisfy the Entity constraint.
func (b Base) GetID() uuid.UUID { return b.ID }

// BeforeCreate generates the UUID when not supplied by the caller.
func (b *Base) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

// Entity is the minimal contract satisfied by all persisted models.
type Entity interface {
	GetID() uuid.UUID
}

// --- Status / severity enums (kept as strings for GORM compatibility) ---

const (
	ComplianceStatusDraft        = "draft"
	ComplianceStatusActive       = "active"
	ComplianceStatusCompliant    = "compliant"
	ComplianceStatusNonCompliant = "non_compliant"
	ComplianceStatusArchived     = "archived"
)

const (
	AuditStatusScheduled  = "scheduled"
	AuditStatusInProgress = "in_progress"
	AuditStatusCompleted  = "completed"
	AuditStatusCancelled  = "cancelled"
)

const (
	ViolationStatusOpen     = "open"
	ViolationStatusInReview = "in_review"
	ViolationStatusResolved = "resolved"
	ViolationStatusClosed   = "closed"
)

const (
	AlertStatusOpen         = "open"
	AlertStatusAcknowledged = "acknowledged"
	AlertStatusResolved     = "resolved"
)

const (
	DeadlineStatusUpcoming  = "upcoming"
	DeadlineStatusDue       = "due"
	DeadlineStatusOverdue   = "overdue"
	DeadlineStatusCompleted = "completed"
)

const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
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

// Audit represents a scheduled or executed compliance audit.
type Audit struct {
	Base
	Title        string     `gorm:"size:255;not null;index" json:"title" vd:"$ != ''"`
	Description  string     `gorm:"type:text" json:"description"`
	Status       string     `gorm:"size:32;not null;default:scheduled;index" json:"status"`
	ComplianceID uuid.UUID  `gorm:"type:uuid;index" json:"compliance_id"`
	AuditorID    string     `gorm:"size:64;index" json:"auditor_id"`
	ScheduledAt  *time.Time `json:"scheduled_at"`
	StartedAt    *time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
}

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

// CorrectiveAction represents the remediation plan for a violation.
type CorrectiveAction struct {
	Base
	Title       string     `gorm:"size:255;not null;index" json:"title" vd:"$ != ''"`
	Description string     `gorm:"type:text" json:"description"`
	Status      string     `gorm:"size:32;not null;default:open;index" json:"status"`
	ViolationID uuid.UUID  `gorm:"type:uuid;index" json:"violation_id"`
	OwnerID     string     `gorm:"size:64;index" json:"owner_id"`
	DueDate     *time.Time `json:"due_date"`
	CompletedAt *time.Time `json:"completed_at"`
}

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

// AuditLog records who did what, when, for which entity, with full before/
// after snapshots and a field-level change diff. Every mutating operation
// (CRUD, lifecycle actions, logins) writes an entry, so the trail is a
// complete, queryable history.
type AuditLog struct {
	Base
	Action     string         `gorm:"size:64;not null;index" json:"action"`
	Status     string         `gorm:"size:16;not null;default:success;index" json:"status"`
	EntityType string         `gorm:"size:64;index" json:"entity_type"`
	EntityID   uuid.UUID      `gorm:"type:uuid;index" json:"entity_id"`
	ActorID    string         `gorm:"size:64;index" json:"actor_id"`
	IP         string         `gorm:"size:64" json:"ip"`
	UserAgent  string         `gorm:"size:255" json:"user_agent"`
	BeforeData datatypes.JSON `gorm:"type:jsonb" json:"before_data,omitempty"`
	AfterData  datatypes.JSON `gorm:"type:jsonb" json:"after_data,omitempty"`
	Changes    datatypes.JSON `gorm:"type:jsonb" json:"changes,omitempty"`
	Metadata   datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"`
}

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

// Role is an RBAC role. Permissions are linked through the role_permissions
// join table (many-to-many).
type Role struct {
	Base
	Name        string       `gorm:"size:64;not null" json:"name" vd:"$ != ''"`
	Code        string       `gorm:"size:64;not null;uniqueIndex" json:"code" vd:"$ != ''"`
	Description string       `gorm:"type:text" json:"description"`
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
}

// OutboxEvent is a reliably-queued domain event (transactional outbox). The
// background dispatcher publishes rows with published_at = NULL to the bus
// and marks them delivered, retrying with backoff when the bus is down.
type OutboxEvent struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Subject     string         `gorm:"size:128;not null;index" json:"subject"`
	EventType   string         `gorm:"size:64;not null" json:"event_type"`
	Payload     datatypes.JSON `gorm:"type:jsonb" json:"payload"`
	CreatedAt   time.Time      `json:"created_at"`
	PublishedAt *time.Time     `gorm:"index" json:"published_at,omitempty"`
	Attempts    int            `gorm:"not null;default:0" json:"attempts"`
	LastError   string         `gorm:"type:text" json:"last_error,omitempty"`
}

// User is an authenticated account bound to a Role.
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

// All returns every model so migrations can register them in one place.
func All() []any {
	return []any{
		&Regulation{},
		&Compliance{},
		&Audit{},
		&Checklist{},
		&Alert{},
		&Report{},
		&Violation{},
		&CorrectiveAction{},
		&Deadline{},
		&AuditLog{},
		&Permission{},
		&Role{},
		&User{},
		&OutboxEvent{},
	}
}
