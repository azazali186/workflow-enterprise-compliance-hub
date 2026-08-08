// Package deadline implements deadline tracking: body-only CRUD, a complete
// action, and a background job (guarded by a distributed lock) that marks
// deadlines due/overdue and emits deadline_approaching events.
package deadline

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"

	"github.com/aeroxe/compliance-hub/backend/internal/auditlog"
	"github.com/aeroxe/compliance-hub/backend/internal/crud"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/events"
	"github.com/aeroxe/compliance-hub/backend/internal/lock"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/outbox"
	"github.com/aeroxe/compliance-hub/backend/internal/repository"
	"github.com/aeroxe/compliance-hub/backend/internal/respond"
)

// RegisterRoutes mounts deadline endpoints under /api/v1/deadlines.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	repo := repository.New[models.Deadline](d.DB, d.Cache, "deadline")
	svc := &Service{repo: repo}

	resource := &crud.Resource[models.Deadline]{
		Repo:   repo,
		Bus:    d.Bus,
		DB:     d.DB,
		Slug:   "deadline",
		Audit:  true,
		Events: map[string]string{"created": "deadline.created"},
		SearchColumns: map[string]string{
			"title":       "title",
			"status":      "status",
			"entity_type": "entity_type",
			"entity_id":   "entity_id",
			"deadline_at": "deadline_at",
			"created_at":  "created_at",
			"updated_at":  "updated_at",
		},
		SummaryBy: "status",
		BeforeCreate: func(ctx context.Context, c *app.RequestContext, e *models.Deadline) error {
			if e.Status == "" {
				e.Status = models.DeadlineStatusUpcoming
			}
			return nil
		},
	}
	h := resource.Handlers()

	rg := g.Group("/deadlines")
	rg.POST("", h.Create)
	rg.POST("/search", h.Search)
	rg.POST("/get", h.GetByID)
	rg.PATCH("", h.Update)
	rg.DELETE("", h.Delete)
	rg.POST("/complete", svc.completeHandler)
}

// Service contains deadline business logic.
type Service struct {
	repo *repository.Repository[models.Deadline]
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
		"status":       models.DeadlineStatusCompleted,
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
		Action: "complete", EntityType: "deadline", EntityID: id,
		Before: before, After: e,
	}
	auditEntry.ActorID, auditEntry.IP, auditEntry.UserAgent = auditlog.FromRequest(c)
	if err := auditlog.Record(ctx, s.repo.DB(), auditEntry); err != nil {
		slog.Warn("audit log write failed", "resource", "deadline", "action", "complete", "error", err)
	}
	respond.OK(c, *e)
}

func (s *Service) completeError(c *app.RequestContext, err error) {
	if errors.Is(err, repository.ErrNotFound) {
		respond.NotFound(c, "not_found", err)
		return
	}
	respond.Internal(c, err)
}

// RunJob periodically evaluates open deadlines and publishes approaching /
// overdue events (README WebSocket event: deadline_approaching). A distributed
// lock prevents duplicate processing across replicas. The supplied WaitGroup
// is tracked so graceful shutdown can await the job's final tick.
func RunJob(ctx context.Context, d deps.Deps, l lock.Lock, interval time.Duration, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("deadline job started", "interval", interval.String())
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = lock.WithLock(ctx, l, "jobs:deadlines", 30*time.Second, func() error {
					evaluate(ctx, d)
					return nil
				})
			}
		}
	}()
}

func evaluate(ctx context.Context, d deps.Deps) {
	now := repository.Now()
	horizon := now.Add(24 * time.Hour)

	var pending []models.Deadline
	// Structured GORM query — no raw SQL.
	err := d.DB.WithContext(ctx).
		Where("status IN ?", []string{models.DeadlineStatusUpcoming, models.DeadlineStatusDue}).
		Where("deadline_at <= ?", horizon).
		Find(&pending).Error
	if err != nil {
		slog.Error("deadline job query failed", "error", err)
		return
	}

	for i := range pending {
		dline := &pending[i]
		var newStatus, eventType string
		switch {
		case now.After(dline.DeadlineAt):
			newStatus, eventType = models.DeadlineStatusOverdue, events.WSDeadlineApproaching
		case dline.Status == models.DeadlineStatusUpcoming:
			newStatus, eventType = models.DeadlineStatusDue, events.WSDeadlineApproaching
		default:
			continue
		}
		before := *dline
		if err := d.DB.WithContext(ctx).Model(dline).Update("status", newStatus).Error; err != nil {
			slog.Error("deadline job update failed", "id", dline.ID, "error", err)
			continue
		}
		dline.Status = newStatus
		_ = outbox.Enqueue(ctx, d.DB, d.Bus, events.SubjectDeadlineApproaching, eventType, dline)

		// System-driven mutation: keep the trail complete.
		if err := auditlog.Record(ctx, d.DB, auditlog.Entry{
			Action: "evaluate", EntityType: "deadline", EntityID: dline.ID,
			ActorID:  "system",
			Before:   &before,
			After:    dline,
			Metadata: map[string]any{"event_type": eventType},
		}); err != nil {
			slog.Warn("audit log write failed", "resource", "deadline", "action", "evaluate", "error", err)
		}
	}
}
