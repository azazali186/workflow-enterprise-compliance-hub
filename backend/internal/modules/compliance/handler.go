// Package compliance implements ComplianceService: body-only CRUD plus the
// status check action driving the ComplianceCheck saga.
package compliance

import (
	"context"
	"errors"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/auditlog"
	"github.com/aeroxe/compliance-hub/backend/internal/bus"
	"github.com/aeroxe/compliance-hub/backend/internal/crud"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/events"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/outbox"
	"github.com/aeroxe/compliance-hub/backend/internal/repository"
	"github.com/aeroxe/compliance-hub/backend/internal/respond"
)

// RegisterRoutes mounts ComplianceService endpoints under /api/v1/compliances.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	repo := repository.New[models.Compliance](d.DB, d.Cache, "compliance")
	svc := &Service{repo: repo, db: d.DB, bus: d.Bus, now: repository.Now}

	resource := &crud.Resource[models.Compliance]{
		Repo:   repo,
		Bus:    d.Bus,
		DB:     d.DB,
		Slug:   "compliance",
		Audit:  true,
		Events: map[string]string{"created": events.SubjectComplianceCreated, "updated": events.SubjectComplianceUpdated},
		Types:  map[string]string{"created": "compliance.created", "updated": "compliance.updated"},
		SearchColumns: map[string]string{
			"name":          "name",
			"status":        "status",
			"risk_level":    "risk_level",
			"owner_id":      "owner_id",
			"regulation_id": "regulation_id",
			"due_date":      "due_date",
			"created_at":    "created_at",
			"updated_at":    "updated_at",
		},
		SummaryBy: "status",
		BeforeCreate: func(ctx context.Context, c *app.RequestContext, e *models.Compliance) error {
			if e.Status == "" {
				e.Status = models.ComplianceStatusDraft
			}
			if e.RiskLevel == "" {
				e.RiskLevel = "medium"
			}
			return nil
		},
	}
	h := resource.Handlers()

	rg := g.Group("/compliances")
	rg.POST("", h.Create)
	rg.POST("/search", h.Search)
	rg.POST("/get", h.GetByID)
	rg.PATCH("", h.Update)
	rg.DELETE("", h.Delete)
	rg.POST("/check", func(ctx context.Context, c *app.RequestContext) {
		id, ok := crud.BindID(c)
		if !ok {
			return
		}
		result, err := svc.Check(ctx, c, id)
		if errors.Is(err, repository.ErrNotFound) {
			respond.NotFound(c, "not_found", err)
			return
		}
		if err != nil {
			respond.Internal(c, err)
			return
		}
		respond.OK(c, result)
	})
}

// CheckResult describes the outcome of a compliance status evaluation.
type CheckResult struct {
	Compliance models.Compliance `json:"compliance"`
	Changed    bool              `json:"changed"`
	Reason     string            `json:"reason"`
}

// Service contains ComplianceService business logic.
type Service struct {
	repo *repository.Repository[models.Compliance]
	db   *gorm.DB
	bus  bus.Bus
	now  func() time.Time
}

// Check evaluates a compliance against its due date and last review. If the
// due date has passed without a review the compliance becomes non_compliant,
// otherwise it is compliant (draft/archived records are left untouched).
// Every evaluation is written to the audit trail with before/after snapshots.
func (s *Service) Check(ctx context.Context, c *app.RequestContext, id uuid.UUID) (*CheckResult, error) {
	compliance, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	before := *compliance // pre-change snapshot for the audit trail

	if compliance.Status == models.ComplianceStatusDraft || compliance.Status == models.ComplianceStatusArchived {
		s.auditCheck(ctx, c, id, &before, compliance, "draft or archived — no evaluation")
		return &CheckResult{Compliance: *compliance, Changed: false, Reason: "draft or archived — no evaluation"}, nil
	}

	now := s.now()
	changed := false
	reason := ""
	var newStatus string

	switch {
	case compliance.DueDate != nil && now.After(*compliance.DueDate):
		if compliance.LastReviewedAt == nil || compliance.LastReviewedAt.Before(*compliance.DueDate) {
			newStatus = models.ComplianceStatusNonCompliant
			reason = "due date passed without a review"
		} else {
			newStatus = models.ComplianceStatusCompliant
			reason = "reviewed after due date"
		}
	default:
		newStatus = models.ComplianceStatusCompliant
		reason = "within due date"
	}

	if newStatus != compliance.Status {
		changed = true
		if err := s.repo.UpdatePartial(ctx, id, map[string]any{"status": newStatus}); err != nil {
			return nil, err
		}
		compliance.Status = newStatus
		_ = outbox.Enqueue(ctx, s.db, s.bus, events.SubjectComplianceAlert, events.WSComplianceAlert, compliance)
	}

	// Audit after the possible status flip so the diff is accurate.
	s.auditCheck(ctx, c, id, &before, compliance, reason)

	return &CheckResult{Compliance: *compliance, Changed: changed, Reason: reason}, nil
}

// auditCheck records a check evaluation with the pre/post status snapshots.
func (s *Service) auditCheck(ctx context.Context, c *app.RequestContext, id uuid.UUID, before, after *models.Compliance, reason string) {
	e := auditlog.Entry{
		Action:     "check",
		EntityType: "compliance",
		EntityID:   id,
		Before:     before,
		After:      after,
		Metadata:   map[string]any{"reason": reason},
	}
	e.ActorID, e.IP, e.UserAgent = auditlog.FromRequest(c)
	if err := auditlog.Record(ctx, s.db, e); err != nil {
		return // audit must never break the check
	}
}
