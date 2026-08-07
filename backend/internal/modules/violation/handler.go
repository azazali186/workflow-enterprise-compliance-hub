// Package violation implements the ViolationProcessing saga entry points:
// body-only CRUD plus resolve.
package violation

import (
	"context"
	"errors"
	"log/slog"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
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

// RegisterRoutes mounts violation endpoints under /api/v1/violations.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	repo := repository.New[models.Violation](d.DB, d.Cache, "violation")
	svc := &Service{repo: repo, db: d.DB, bus: d.Bus}

	resource := &crud.Resource[models.Violation]{
		Repo:   repo,
		Bus:    d.Bus,
		DB:     d.DB,
		Slug:   "violation",
		Audit:  true,
		Events: map[string]string{"created": events.SubjectViolationDetected},
		Types:  map[string]string{"created": events.WSViolationDetected},
		SearchColumns: map[string]string{
			"title":         "title",
			"status":        "status",
			"severity":      "severity",
			"compliance_id": "compliance_id",
			"regulation_id": "regulation_id",
			"detected_at":   "detected_at",
			"resolved_at":   "resolved_at",
			"created_at":    "created_at",
			"updated_at":    "updated_at",
		},
		SummaryBy: "severity",
		BeforeCreate: func(ctx context.Context, c *app.RequestContext, e *models.Violation) error {
			if e.Status == "" {
				e.Status = models.ViolationStatusOpen
			}
			if e.Severity == "" {
				e.Severity = "medium"
			}
			now := repository.Now()
			if e.DetectedAt == nil {
				e.DetectedAt = &now
			}
			return nil
		},
	}
	h := resource.Handlers()

	rg := g.Group("/violations")
	rg.POST("", h.Create)
	rg.POST("/search", h.Search)
	rg.POST("/get", h.GetByID)
	rg.PATCH("", h.Update)
	rg.DELETE("", h.Delete)
	rg.POST("/resolve", svc.resolveHandler)
}

// Service contains violation business logic.
type Service struct {
	repo *repository.Repository[models.Violation]
	db   *gorm.DB
	bus  bus.Bus
}

func (s *Service) resolveHandler(ctx context.Context, c *app.RequestContext) {
	id, ok := crud.BindID(c)
	if !ok {
		return
	}
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.resolveError(c, err)
		return
	}
	now := repository.Now()
	if err := s.repo.UpdatePartial(ctx, id, map[string]any{
		"status":      models.ViolationStatusResolved,
		"resolved_at": now,
	}); err != nil {
		s.resolveError(c, err)
		return
	}
	e, err := s.repo.GetByID(ctx, id)
	if err != nil {
		respond.OK(c, nil)
		return
	}
	auditEntry := auditlog.Entry{
		Action: "resolve", EntityType: "violation", EntityID: id,
		Before: before, After: e,
	}
	auditEntry.ActorID, auditEntry.IP, auditEntry.UserAgent = auditlog.FromRequest(c)
	if err := auditlog.Record(ctx, s.repo.DB(), auditEntry); err != nil {
		slog.Warn("audit log write failed", "resource", "violation", "action", "resolve", "error", err)
	}
	// Drive the ViolationProcessing saga to its resolve step.
	_ = outbox.Enqueue(ctx, s.db, s.bus, events.SubjectViolationResolved, "violation.resolved", e)
	respond.OK(c, *e)
}

func (s *Service) resolveError(c *app.RequestContext, err error) {
	if errors.Is(err, repository.ErrNotFound) {
		respond.NotFound(c, "not_found", err)
		return
	}
	respond.Internal(c, err)
}
