package saga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/bus"
	"github.com/aeroxe/compliance-hub/backend/internal/events"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/outbox"
	"github.com/aeroxe/compliance-hub/backend/internal/repository"
)

// The four business sagas from the README "Saga Orchestrator" table, each
// orchestrated via NATS events with state in Redis:
//
//	ComplianceCheck      monitors compliance status against regulations
//	AuditExecution       manages audit scheduling and execution
//	ViolationProcessing  violation detection + corrective action tracking
//	CorrectiveActionFlow remediation plan lifecycle
//
// Steps advance on the events the handlers already publish; a step that has
// not received its event yet keeps the saga active in Redis ("until
// completion").

// complianceCheck defines the ComplianceCheck saga. On create/update the
// engine automatically evaluates the compliance against its due date and last
// review (the same rule the manual /check endpoint applies), flips the status
// when needed, and drives a real-time compliance_alert event.
func (e *Engine) complianceCheck() Definition {
	repo := repository.New[models.Compliance](e.db, e.cache, "compliance")

	evaluate := Step{
		Name: "evaluate",
		Trigger: func(ev bus.Event) bool {
			return ev.Subject == events.SubjectComplianceCreated || ev.Subject == events.SubjectComplianceUpdated
		},
		Action: func(ctx context.Context, s *Saga, ev bus.Event) error {
			var compliance models.Compliance
			if err := e.firstEntity(ctx, &compliance, s.EntityID); err != nil {
				return fmt.Errorf("load compliance: %w", err)
			}
			if compliance.Status == models.ComplianceStatusDraft || compliance.Status == models.ComplianceStatusArchived {
				s.Payload["reason"] = "draft or archived — no evaluation"
				return nil
			}

			newStatus, reason := evaluateCompliance(&compliance, e.now())
			changed := newStatus != compliance.Status
			if changed {
				s.Payload["before_status"] = compliance.Status
				if err := repo.UpdatePartial(ctx, s.EntityID, map[string]any{"status": newStatus}); err != nil {
					return fmt.Errorf("update compliance status: %w", err)
				}
				compliance.Status = newStatus
			}
			s.Payload["reason"] = reason
			s.Payload["status"] = newStatus
			s.Payload["changed"] = changed

			// Notify on any change or on a freshly created compliance; the
			// outbox guarantees delivery and drives the alert step.
			if changed || ev.Subject == events.SubjectComplianceCreated {
				if err := outbox.Enqueue(ctx, e.db, e.bus, events.SubjectComplianceAlert, events.WSComplianceAlert, compliance); err != nil {
					return fmt.Errorf("queue compliance alert: %w", err)
				}
			}
			return nil
		},
		Compensate: func(ctx context.Context, s *Saga, _ bus.Event) error {
			before, _ := s.Payload["before_status"].(string)
			if before == "" || before == s.Payload["status"] {
				return nil
			}
			return repo.UpdatePartial(ctx, s.EntityID, map[string]any{"status": before})
		},
	}

	alert := Step{
		Name: "alert",
		Trigger: func(ev bus.Event) bool {
			return ev.Subject == events.SubjectComplianceAlert
		},
		Action: func(ctx context.Context, s *Saga, ev bus.Event) error {
			s.Payload["alerted_at"] = e.now().Format(time.RFC3339)
			return nil
		},
	}

	return Definition{
		Type:     TypeComplianceCheck,
		Subjects: []string{events.SubjectComplianceCreated, events.SubjectComplianceUpdated, events.SubjectComplianceAlert},
		Steps:    []Step{evaluate, alert},
	}
}

// evaluateCompliance is the shared status rule: a compliance whose due date
// passed without a later review is non_compliant; reviewed-after-due is
// compliant; otherwise compliant while within the due date. Kept identical to
// the manual check in the compliance module.
func evaluateCompliance(c *models.Compliance, now time.Time) (status, reason string) {
	switch {
	case c.DueDate != nil && now.After(*c.DueDate):
		if c.LastReviewedAt == nil || c.LastReviewedAt.Before(*c.DueDate) {
			return models.ComplianceStatusNonCompliant, "due date passed without a review"
		}
		return models.ComplianceStatusCompliant, "reviewed after due date"
	default:
		return models.ComplianceStatusCompliant, "within due date"
	}
}

// auditExecution defines the AuditExecution saga: scheduled -> started ->
// completed, tracking each lifecycle milestone from the events the audit
// module already emits.
func (e *Engine) auditExecution() Definition {
	return Definition{
		Type: TypeAuditExecution,
		Subjects: []string{
			events.SubjectAuditScheduled, events.SubjectAuditStarted, events.SubjectAuditCompleted,
		},
		Steps: []Step{
			{
				Name: "schedule",
				Trigger: func(ev bus.Event) bool {
					return ev.Subject == events.SubjectAuditScheduled && payloadStatus(ev) == models.AuditStatusScheduled
				},
				Action: func(ctx context.Context, s *Saga, ev bus.Event) error {
					var a models.Audit
					if err := e.firstEntity(ctx, &a, s.EntityID); err != nil {
						return fmt.Errorf("load audit: %w", err)
					}
					s.Payload["scheduled_at"] = timeField(a.ScheduledAt)
					return nil
				},
			},
			{
				Name: "start",
				Trigger: func(ev bus.Event) bool {
					return ev.Subject == events.SubjectAuditStarted
				},
				Action: func(ctx context.Context, s *Saga, ev bus.Event) error {
					var a models.Audit
					if err := e.firstEntity(ctx, &a, s.EntityID); err != nil {
						return fmt.Errorf("load audit: %w", err)
					}
					s.Payload["started_at"] = timeField(a.StartedAt)
					return nil
				},
			},
			{
				Name: "complete",
				Trigger: func(ev bus.Event) bool {
					return ev.Subject == events.SubjectAuditCompleted
				},
				Action: func(ctx context.Context, s *Saga, ev bus.Event) error {
					var a models.Audit
					if err := e.firstEntity(ctx, &a, s.EntityID); err != nil {
						return fmt.Errorf("load audit: %w", err)
					}
					s.Payload["completed_at"] = timeField(a.CompletedAt)
					return nil
				},
			},
		},
	}
}

// violationProcessing defines the ViolationProcessing saga: on detection the
// engine opens a corrective action for the violation (deduplicated), and on
// resolution it closes the still-open action — the README's "violation
// detection and corrective action tracking".
func (e *Engine) violationProcessing() Definition {
	caRepo := repository.New[models.CorrectiveAction](e.db, e.cache, "correctiveaction")

	detect := Step{
		Name: "detect",
		Trigger: func(ev bus.Event) bool {
			return ev.Subject == events.SubjectViolationDetected
		},
		Action: func(ctx context.Context, s *Saga, ev bus.Event) error {
			var v models.Violation
			if err := e.firstEntity(ctx, &v, s.EntityID); err != nil {
				return fmt.Errorf("load violation: %w", err)
			}
			s.Payload["severity"] = v.Severity
			s.Payload["detected_at"] = timeField(v.DetectedAt)

			// Already tracked? Never create a duplicate corrective action.
			var existing models.CorrectiveAction
			err := e.db.WithContext(ctx).Where("violation_id = ?", s.EntityID).First(&existing).Error
			if err == nil {
				s.Payload["corrective_action_id"] = existing.ID.String()
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("find corrective action: %w", err)
			}

			ca := models.CorrectiveAction{
				Title:       "Corrective action for " + v.Title,
				Status:      "open",
				ViolationID: s.EntityID,
			}
			if err := caRepo.Create(ctx, &ca); err != nil {
				return fmt.Errorf("create corrective action: %w", err)
			}
			s.Payload["corrective_action_id"] = ca.ID.String()
			// Kick off the CorrectiveActionFlow saga for the auto-created
			// action, matching the manual-creation path.
			_ = outbox.Enqueue(ctx, e.db, e.bus, events.SubjectCorrectiveActionCreated, "correctiveaction.created", ca)
			return nil
		},
		Compensate: func(ctx context.Context, s *Saga, _ bus.Event) error {
			idStr, _ := s.Payload["corrective_action_id"].(string)
			if idStr == "" {
				return nil
			}
			id, err := uuid.Parse(idStr)
			if err != nil {
				return nil
			}
			ca, err := caRepo.GetByID(ctx, id)
			if err != nil {
				return nil
			}
			if ca.Status == "open" { // only roll back what we created
				return caRepo.Delete(ctx, id)
			}
			return nil
		},
	}

	resolve := Step{
		Name: "resolve",
		Trigger: func(ev bus.Event) bool {
			return ev.Subject == events.SubjectViolationResolved
		},
		Action: func(ctx context.Context, s *Saga, ev bus.Event) error {
			s.Payload["resolved_at"] = e.now().Format(time.RFC3339)
			// Close the still-open corrective action and let the
			// CorrectiveActionFlow saga observe the completion event.
			var ca models.CorrectiveAction
			err := e.db.WithContext(ctx).Where("violation_id = ? AND status = ?", s.EntityID, "open").First(&ca).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("find open corrective action: %w", err)
			}
			now := e.now()
			if err := caRepo.UpdatePartial(ctx, ca.ID, map[string]any{"status": "completed", "completed_at": now}); err != nil {
				return fmt.Errorf("complete corrective action: %w", err)
			}
			_ = outbox.Enqueue(ctx, e.db, e.bus, events.SubjectCorrectiveActionCompleted, "correctiveaction.completed", ca)
			return nil
		},
	}

	return Definition{
		Type:     TypeViolationProcessing,
		Subjects: []string{events.SubjectViolationDetected, events.SubjectViolationResolved},
		Steps:    []Step{detect, resolve},
	}
}

// correctiveActionFlow defines the CorrectiveActionFlow saga: assigned on
// creation, completed when the action is closed (by an officer or
// automatically by the ViolationProcessing saga).
func (e *Engine) correctiveActionFlow() Definition {
	return Definition{
		Type: TypeCorrectiveActionFlow,
		Subjects: []string{
			events.SubjectCorrectiveActionCreated, events.SubjectCorrectiveActionCompleted,
		},
		Steps: []Step{
			{
				Name: "assign",
				Trigger: func(ev bus.Event) bool {
					return ev.Subject == events.SubjectCorrectiveActionCreated
				},
				Action: func(ctx context.Context, s *Saga, ev bus.Event) error {
					var ca models.CorrectiveAction
					if err := e.firstEntity(ctx, &ca, s.EntityID); err != nil {
						return fmt.Errorf("load corrective action: %w", err)
					}
					s.Payload["owner_id"] = ca.OwnerID
					s.Payload["due_date"] = timeField(ca.DueDate)
					return nil
				},
			},
			{
				Name: "complete",
				Trigger: func(ev bus.Event) bool {
					return ev.Subject == events.SubjectCorrectiveActionCompleted
				},
				Action: func(ctx context.Context, s *Saga, ev bus.Event) error {
					s.Payload["completed_at"] = e.now().Format(time.RFC3339)
					return nil
				},
			},
		},
	}
}

// firstEntity reads authoritative entity state directly from the database.
// The orchestration layer must not act on stale cache entries, so saga steps
// bypass the caching repository for reads (writes still invalidate the cache).
func (e *Engine) firstEntity(ctx context.Context, out any, id uuid.UUID) error {
	return e.db.WithContext(ctx).First(out, "id = ?", id).Error
}

// payloadStatus reads the "status" field of an event payload (the marshaled
// entity), used to distinguish create from start events sharing a subject.
func payloadStatus(ev bus.Event) string {
	raw, err := json.Marshal(ev.Payload)
	if err != nil {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	s, _ := m["status"].(string)
	return s
}

// timeField renders an optional time as RFC3339 (or an empty string).
func timeField(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
