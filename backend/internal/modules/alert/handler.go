// Package alert implements AlertService: body-only CRUD plus acknowledge /
// resolve actions.
package alert

import (
	"context"
	"errors"

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

// RegisterRoutes mounts AlertService endpoints under /api/v1/alerts.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	repo := repository.New[models.Alert](d.DB, d.Cache, "alert")
	svc := &Service{repo: repo, db: d.DB, bus: d.Bus}

	resource := &crud.Resource[models.Alert]{
		Repo:   repo,
		Bus:    d.Bus,
		DB:     d.DB,
		Slug:   "alert",
		Audit:  true,
		Events: map[string]string{"created": events.SubjectComplianceAlert},
		Types:  map[string]string{"created": events.WSComplianceAlert},
		SearchColumns: map[string]string{
			"type":        "type",
			"status":      "status",
			"severity":    "severity",
			"entity_type": "entity_type",
			"entity_id":   "entity_id",
			"created_at":  "created_at",
			"updated_at":  "updated_at",
		},
		SummaryBy: "severity",
		BeforeCreate: func(ctx context.Context, c *app.RequestContext, e *models.Alert) error {
			if e.Status == "" {
				e.Status = models.AlertStatusOpen
			}
			if e.Severity == "" {
				e.Severity = "medium"
			}
			return nil
		},
	}
	h := resource.Handlers()

	rg := g.Group("/alerts")
	rg.POST("", h.Create)
	rg.POST("/search", h.Search)
	rg.POST("/get", h.GetByID)
	rg.PATCH("", h.Update)
	rg.DELETE("", h.Delete)
	rg.POST("/acknowledge", svc.acknowledgeHandler)
	rg.POST("/resolve", svc.resolveHandler)
}

// Service contains AlertService business logic.
type Service struct {
	repo *repository.Repository[models.Alert]
	db   *gorm.DB
	bus  bus.Bus
}

func (s *Service) acknowledgeHandler(ctx context.Context, c *app.RequestContext) {
	id, ok := crud.BindID(c)
	if !ok {
		return
	}
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.alertError(c, err)
		return
	}
	actor := c.GetString("auth_user_id")
	if err := s.repo.UpdatePartial(ctx, id, map[string]any{
		"status":          models.AlertStatusAcknowledged,
		"acknowledged_by": actor,
	}); err != nil {
		s.alertError(c, err)
		return
	}
	e, err := s.repo.GetByID(ctx, id)
	if err != nil {
		respond.OK(c, nil)
		return
	}
	_ = outbox.Enqueue(ctx, s.db, s.bus, events.SubjectComplianceAlert, events.WSComplianceAlert, *e)
	s.auditAction(ctx, c, id, "acknowledge", before, e)
	respond.OK(c, *e)
}

func (s *Service) resolveHandler(ctx context.Context, c *app.RequestContext) {
	id, ok := crud.BindID(c)
	if !ok {
		return
	}
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.alertError(c, err)
		return
	}
	now := repository.Now()
	if err := s.repo.UpdatePartial(ctx, id, map[string]any{
		"status":      models.AlertStatusResolved,
		"resolved_at": now,
	}); err != nil {
		s.alertError(c, err)
		return
	}
	e, err := s.repo.GetByID(ctx, id)
	if err != nil {
		respond.OK(c, nil)
		return
	}
	s.auditAction(ctx, c, id, "resolve", before, e)
	respond.OK(c, *e)
}

// auditAction records a lifecycle action (acknowledge/resolve) with snapshots.
func (s *Service) auditAction(ctx context.Context, c *app.RequestContext, id uuid.UUID, action string, before, after *models.Alert) {
	e := auditlog.Entry{
		Action:     action,
		EntityType: "alert",
		EntityID:   id,
		Before:     before,
		After:      after,
	}
	e.ActorID, e.IP, e.UserAgent = auditlog.FromRequest(c)
	if err := auditlog.Record(ctx, s.db, e); err != nil {
		return // audit must never break the action
	}
}

func (s *Service) alertError(c *app.RequestContext, err error) {
	if errors.Is(err, repository.ErrNotFound) {
		respond.NotFound(c, "not_found", err)
		return
	}
	respond.Internal(c, err)
}
