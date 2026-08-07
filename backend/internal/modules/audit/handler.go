// Package audit implements AuditService: body-only CRUD plus lifecycle actions
// (start / complete) driving the AuditExecution saga.
package audit

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

// RegisterRoutes mounts AuditService endpoints under /api/v1/audits.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	repo := repository.New[models.Audit](d.DB, d.Cache, "audit")
	svc := &Service{repo: repo, db: d.DB, bus: d.Bus, now: repository.Now}

	resource := &crud.Resource[models.Audit]{
		Repo:   repo,
		Bus:    d.Bus,
		DB:     d.DB,
		Slug:   "audit",
		Audit:  true,
		Events: map[string]string{"created": events.SubjectAuditScheduled},
		Types:  map[string]string{"created": events.WSAuditScheduled},
		SearchColumns: map[string]string{
			"title":         "title",
			"status":        "status",
			"compliance_id": "compliance_id",
			"auditor_id":    "auditor_id",
			"scheduled_at":  "scheduled_at",
			"started_at":    "started_at",
			"completed_at":  "completed_at",
			"created_at":    "created_at",
			"updated_at":    "updated_at",
		},
		SummaryBy: "status",
		BeforeCreate: func(ctx context.Context, c *app.RequestContext, e *models.Audit) error {
			if e.Status == "" {
				e.Status = models.AuditStatusScheduled
			}
			return nil
		},
	}
	h := resource.Handlers()

	rg := g.Group("/audits")
	rg.POST("", h.Create)
	rg.POST("/search", h.Search)
	rg.POST("/get", h.GetByID)
	rg.PATCH("", h.Update)
	rg.DELETE("", h.Delete)
	rg.POST("/start", svc.startHandler)
	rg.POST("/complete", svc.completeHandler)
}

// Service contains AuditService business logic.
type Service struct {
	repo *repository.Repository[models.Audit]
	db   *gorm.DB
	bus  bus.Bus
	now  func() time.Time
}

func (s *Service) startHandler(ctx context.Context, c *app.RequestContext) {
	id, ok := crud.BindID(c)
	if !ok {
		return
	}
	audit, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		respond.NotFound(c, "not_found", err)
		return
	}
	if err != nil {
		respond.Internal(c, err)
		return
	}
	now := s.now()
	before := *audit
	if err := s.repo.UpdatePartial(ctx, id, map[string]any{
		"status":     models.AuditStatusInProgress,
		"started_at": now,
	}); err != nil {
		respond.Internal(c, err)
		return
	}
	audit.Status = models.AuditStatusInProgress
	audit.StartedAt = &now
	_ = outbox.Enqueue(ctx, s.db, s.bus, events.SubjectAuditStarted, events.WSAuditScheduled, audit)
	s.auditAction(ctx, c, id, "start", &before, audit)
	respond.OK(c, audit)
}

func (s *Service) completeHandler(ctx context.Context, c *app.RequestContext) {
	id, ok := crud.BindID(c)
	if !ok {
		return
	}
	audit, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		respond.NotFound(c, "not_found", err)
		return
	}
	if err != nil {
		respond.Internal(c, err)
		return
	}
	now := s.now()
	before := *audit
	if err := s.repo.UpdatePartial(ctx, id, map[string]any{
		"status":       models.AuditStatusCompleted,
		"completed_at": now,
	}); err != nil {
		respond.Internal(c, err)
		return
	}
	audit.Status = models.AuditStatusCompleted
	audit.CompletedAt = &now
	_ = outbox.Enqueue(ctx, s.db, s.bus, events.SubjectAuditCompleted, events.WSAuditScheduled, audit)
	s.auditAction(ctx, c, id, "complete", &before, audit)
	respond.OK(c, audit)
}

// auditAction records a lifecycle action (start/complete) with before/after.
func (s *Service) auditAction(ctx context.Context, c *app.RequestContext, id uuid.UUID, action string, before, after *models.Audit) {
	e := auditlog.Entry{
		Action:     action,
		EntityType: "audit",
		EntityID:   id,
		Before:     before,
		After:      after,
	}
	e.ActorID, e.IP, e.UserAgent = auditlog.FromRequest(c)
	if err := auditlog.Record(ctx, s.db, e); err != nil {
		return // audit must never break the action
	}
}
