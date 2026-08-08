// Status and severity enums. Kept as strings for GORM compatibility and
// readable JSON payloads.
package models

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
