// Package events defines the canonical event subjects and WebSocket event
// types used across the platform.
package events

// WebSocket event types (mirrors the README "WebSocket Events" table).
const (
	WSComplianceAlert     = "compliance_alert"
	WSAuditScheduled      = "audit_scheduled"
	WSViolationDetected   = "violation_detected"
	WSDeadlineApproaching = "deadline_approaching"
)

// Event subjects (NATS subject namespacing).
const (
	SubjectComplianceAlert     = "compliance.alert"
	SubjectAuditScheduled      = "audit.scheduled"
	SubjectAuditStarted        = "audit.started"
	SubjectAuditCompleted      = "audit.completed"
	SubjectViolationDetected   = "violation.detected"
	SubjectViolationResolved   = "violation.resolved"
	SubjectDeadlineApproaching = "deadline.approaching"
	SubjectReportGenerated     = "report.generated"
	SubjectNotificationSent    = "notification.sent"
)

// Generic entity lifecycle subjects.
const (
	SubjectComplianceCreated         = "compliance.created"
	SubjectComplianceUpdated         = "compliance.updated"
	SubjectComplianceDeleted         = "compliance.deleted"
	SubjectCorrectiveActionCreated   = "correctiveaction.created"
	SubjectCorrectiveActionCompleted = "correctiveaction.completed"
)
