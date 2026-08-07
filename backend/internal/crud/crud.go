// Package crud provides generic REST handlers on top of the typed repository.
// The API contract is strict: only POST / PATCH / DELETE verbs, no path
// variables, no query parameters — every payload travels in the request body
// (snake_case JSON). Lists use cursor-based pagination via internal/pagination.
package crud

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/auditlog"
	"github.com/aeroxe/compliance-hub/backend/internal/bus"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/outbox"
	"github.com/aeroxe/compliance-hub/backend/internal/pagination"
	"github.com/aeroxe/compliance-hub/backend/internal/repository"
	"github.com/aeroxe/compliance-hub/backend/internal/respond"
)

// IDRequest is the standard body for actions that reference a single entity:
// {"id": "<uuid>"}.
type IDRequest struct {
	ID uuid.UUID `json:"id"`
}

// systemColumns are never writable through update bodies.
var systemColumns = []string{"id", "created_at", "updated_at", "deleted_at"}

// BindID parses the standard {"id": "..."} body used by action endpoints
// (POST /<resource>/<action>).
func BindID(c *app.RequestContext) (uuid.UUID, bool) {
	var req IDRequest
	if err := c.BindAndValidate(&req); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return uuid.Nil, false
	}
	if req.ID == uuid.Nil {
		respond.BadRequest(c, "invalid_body", errors.New("field \"id\" is required"))
		return uuid.Nil, false
	}
	return req.ID, true
}

// Resource wires the standard handlers for a model.
type Resource[T models.Entity] struct {
	Repo *repository.Repository[T]
	Bus  bus.Bus
	DB   *gorm.DB
	Slug string

	// Audit enables audit-log writes for every mutating operation.
	Audit bool
	// Events maps an action ("created"|"updated"|"deleted") to a bus subject.
	Events map[string]string
	// Types optionally maps an action to the WebSocket event type sent to
	// clients (defaults to "<action>.<slug>"). Use this to emit the README
	// event types such as "violation_detected" or "audit_scheduled".
	Types map[string]string

	// BeforeCreate runs after binding, before persistence.
	BeforeCreate func(ctx context.Context, c *app.RequestContext, e *T) error
	// BeforeUpdate runs before applying a partial update.
	BeforeUpdate func(ctx context.Context, c *app.RequestContext, id uuid.UUID, updates map[string]any) error

	// SearchColumns maps logical (client) column names to DB columns for
	// sorting/filtering/date ranges. Defaults cover id/created_at/updated_at/status.
	SearchColumns map[string]string
	// SummaryBy is a logical column whose distinct values are grouped in the
	// pagination summary (e.g. "status" or "severity").
	SummaryBy string
}

// Handlers returns the standard handler set for this resource.
func (r *Resource[T]) Handlers() Handlers[T] { return Handlers[T]{r: r} }

// Handlers exposes the standard body-based endpoints.
type Handlers[T models.Entity] struct{ r *Resource[T] }

// Create handles POST /<resource> with the full entity in the body.
func (h *Handlers[T]) Create(ctx context.Context, c *app.RequestContext) {
	var e T
	if err := c.BindAndValidate(&e); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	if h.r.BeforeCreate != nil {
		if err := h.r.BeforeCreate(ctx, c, &e); err != nil {
			respond.BadRequest(c, "validation_failed", err)
			return
		}
	}
	if err := h.r.Repo.Create(ctx, &e); err != nil {
		respond.Internal(c, err)
		return
	}
	h.r.publish(ctx, "created", &e)
	h.r.audit(ctx, c, "create", nil, &e)
	respond.Created(c, e)
}

// Search handles POST /<resource>/search with a pagination.Query body.
func (h *Handlers[T]) Search(ctx context.Context, c *app.RequestContext) {
	var q pagination.Query
	if err := json.Unmarshal(c.Request.Body(), &q); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	result, err := pagination.Apply[T](h.r.Repo.Scope(), q, h.r.SearchColumns, h.r.SummaryBy)
	if err != nil {
		respond.BadRequest(c, "invalid_search", err)
		return
	}
	respond.OK(c, result)
}

// GetByID handles POST /<resource>/get with {"id": "..."}.
func (h *Handlers[T]) GetByID(ctx context.Context, c *app.RequestContext) {
	var req IDRequest
	if err := c.BindAndValidate(&req); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	if req.ID == uuid.Nil {
		respond.BadRequest(c, "invalid_body", errors.New("field \"id\" is required"))
		return
	}
	e, err := h.r.Repo.GetByID(ctx, req.ID)
	if errors.Is(err, repository.ErrNotFound) {
		respond.NotFound(c, "not_found", err)
		return
	}
	if err != nil {
		respond.Internal(c, err)
		return
	}
	respond.OK(c, *e)
}

// Update handles PATCH /<resource> with {"id": "...", ...fields}.
func (h *Handlers[T]) Update(ctx context.Context, c *app.RequestContext) {
	var updates map[string]any
	if err := json.Unmarshal(c.Request.Body(), &updates); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	idRaw, ok := updates["id"].(string)
	if !ok || idRaw == "" {
		respond.BadRequest(c, "invalid_body", errors.New("field \"id\" is required"))
		return
	}
	id, err := uuid.Parse(idRaw)
	if err != nil {
		respond.BadRequest(c, "invalid_id", err)
		return
	}
	for _, key := range systemColumns {
		delete(updates, key)
	}
	// datatypes.JSON columns (metadata, items, data...) arrive as raw maps or
	// arrays from the JSON body; GORM cannot serialize those into jsonb, so
	// marshal nested values to JSON before handing them to Updates.
	for k, v := range updates {
		switch v.(type) {
		case map[string]any, []any:
			if b, err := json.Marshal(v); err == nil {
				updates[k] = json.RawMessage(b)
			}
		}
	}
	if h.r.BeforeUpdate != nil {
		if err := h.r.BeforeUpdate(ctx, c, id, updates); err != nil {
			respond.BadRequest(c, "validation_failed", err)
			return
		}
	}
	// Capture the pre-change snapshot for the audit trail before applying.
	before, err := h.r.Repo.GetByID(ctx, id)
	if err != nil {
		h.r.updateError(c, err)
		return
	}
	if err := h.r.Repo.UpdatePartial(ctx, id, updates); err != nil {
		h.r.updateError(c, err)
		return
	}
	saved, err := h.r.Repo.GetByID(ctx, id)
	if err != nil {
		respond.Internal(c, err)
		return
	}
	h.r.publish(ctx, "updated", saved)
	h.r.audit(ctx, c, "update", before, saved)
	respond.OK(c, *saved)
}

// Delete handles DELETE /<resource> with {"id": "..."} (soft delete).
func (h *Handlers[T]) Delete(ctx context.Context, c *app.RequestContext) {
	var req IDRequest
	if err := c.BindAndValidate(&req); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	if req.ID == uuid.Nil {
		respond.BadRequest(c, "invalid_body", errors.New("field \"id\" is required"))
		return
	}
	// Snapshot the entity before the (soft) delete so the audit trail keeps it.
	before, err := h.r.Repo.GetByID(ctx, req.ID)
	if err != nil {
		h.r.updateError(c, err)
		return
	}
	if err := h.r.Repo.Delete(ctx, req.ID); err != nil {
		h.r.updateError(c, err)
		return
	}
	if h.r.Bus != nil && h.r.Events != nil && h.r.Events["deleted"] != "" {
		if err := outbox.Enqueue(context.Background(), h.r.DB, h.r.Bus, h.r.Events["deleted"], h.r.eventType("deleted"),
			map[string]any{"action": "deleted", "id": req.ID}); err != nil {
			slog.Error("outbox enqueue failed", "resource", h.r.Slug, "action", "deleted", "error", err)
		}
	}
	h.r.audit(ctx, c, "delete", before, nil)
	respond.NoContent(c)
}

func (r *Resource[T]) updateError(c *app.RequestContext, err error) {
	if errors.Is(err, repository.ErrNotFound) {
		respond.NotFound(c, "not_found", err)
		return
	}
	respond.Internal(c, err)
}

func (r *Resource[T]) publish(_ context.Context, action string, e *T) {
	if r.Bus == nil || r.Events == nil {
		return
	}
	subject, ok := r.Events[action]
	if !ok || subject == "" {
		return
	}
	var payload any
	if e != nil {
		payload = *e
	} else {
		payload = map[string]any{"action": action}
	}
	// Reliable delivery: best-effort immediate publish + outbox queue. A
	// failed queue insert is loud — silently dropping the event would defeat
	// the whole point of the outbox.
	if err := outbox.Enqueue(context.Background(), r.DB, r.Bus, subject, r.eventType(action), payload); err != nil {
		slog.Error("outbox enqueue failed", "resource", r.Slug, "action", action, "error", err)
	}
}

// eventType resolves the WebSocket event type for an action, preferring the
// module-declared type and falling back to "<action>.<slug>".
func (r *Resource[T]) eventType(action string) string {
	if r.Types != nil {
		if t, ok := r.Types[action]; ok && t != "" {
			return t
		}
	}
	return action + "." + r.Slug
}

// audit writes an audit-trail entry with before/after snapshots and the field
// diff. before/after are *T or nil (create has no before, delete no after).
// Audit writes are best-effort: a failure must never break the primary op.
func (r *Resource[T]) audit(ctx context.Context, c *app.RequestContext, action string, before, after *T) {
	if !r.Audit || r.DB == nil {
		return
	}
	e := auditlog.Entry{
		Action:     action,
		EntityType: r.Slug,
		Before:     before,
		After:      after,
	}
	e.ActorID, e.IP, e.UserAgent = auditlog.FromRequest(c)
	if after != nil {
		e.EntityID = (*after).GetID()
	} else if before != nil {
		e.EntityID = (*before).GetID()
	}
	if err := auditlog.Record(ctx, r.DB, e); err != nil {
		slog.Warn("audit log write failed", "resource", r.Slug, "action", action, "error", err)
	}
}
