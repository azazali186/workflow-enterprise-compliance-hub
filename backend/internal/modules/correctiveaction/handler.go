// Package correctiveaction implements the CorrectiveActionFlow: body-only CRUD
// plus a complete action used to close a remediation plan.
package correctiveaction

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

// RegisterRoutes mounts corrective action endpoints under /api/v1/corrective-actions.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	repo := repository.New[models.CorrectiveAction](d.DB, d.Cache, "correctiveaction")
	svc := &Service{repo: repo, db: d.DB, bus: d.Bus}

	resource := &crud.Resource[models.CorrectiveAction]{
		Repo:   repo,
		Bus:    d.Bus,
		DB:     d.DB,
		Slug:   "correctiveaction",
		Audit:  true,
		Events: map[string]string{"created": "correctiveaction.created"},
		SearchColumns: map[string]string{
			"title":        "title",
			"status":       "status",
			"violation_id": "violation_id",
			"owner_id":     "owner_id",
			"due_date":     "due_date",
			"created_at":   "created_at",
			"updated_at":   "updated_at",
		},
		SummaryBy: "status",
		BeforeCreate: func(ctx context.Context, c *app.RequestContext, e *models.CorrectiveAction) error {
			if e.Status == "" {
				e.Status = "open"
			}
			return nil
		},
	}
	h := resource.Handlers()

	rg := g.Group("/corrective-actions")
	rg.POST("", h.Create)
	rg.POST("/search", h.Search)
	rg.POST("/get", h.GetByID)
	rg.PATCH("", h.Update)
	rg.DELETE("", h.Delete)
	rg.POST("/complete", svc.completeHandler)
}

// Service contains corrective action business logic.
type Service struct {
	repo *repository.Repository[models.CorrectiveAction]
	db   *gorm.DB
	bus  bus.Bus
}

func (s *Service) completeHandler(ctx context.Context, c *app.RequestContext) {
	id, ok := crud.BindID(c)
	if !ok {
		return
	}
	before, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.completeError(c, err)
		return
	}
	now := repository.Now()
	if err := s.repo.UpdatePartial(ctx, id, map[string]any{
		"status":       "completed",
		"completed_at": now,
	}); err != nil {
		s.completeError(c, err)
		return
	}
	e, err := s.repo.GetByID(ctx, id)
	if err != nil {
		respond.OK(c, nil)
		return
	}
	auditEntry := auditlog.Entry{
		Action: "complete", EntityType: "correctiveaction", EntityID: id,
		Before: before, After: e,
	}
	auditEntry.ActorID, auditEntry.IP, auditEntry.UserAgent = auditlog.FromRequest(c)
	if err := auditlog.Record(ctx, s.repo.DB(), auditEntry); err != nil {
		slog.Warn("audit log write failed", "resource", "correctiveaction", "action", "complete", "error", err)
	}
	// Drive the CorrectiveActionFlow saga to its completion step.
	_ = outbox.Enqueue(ctx, s.db, s.bus, events.SubjectCorrectiveActionCompleted, "correctiveaction.completed", e)
	respond.OK(c, *e)
}

func (s *Service) completeError(c *app.RequestContext, err error) {
	if errors.Is(err, repository.ErrNotFound) {
		respond.NotFound(c, "not_found", err)
		return
	}
	respond.Internal(c, err)
}
