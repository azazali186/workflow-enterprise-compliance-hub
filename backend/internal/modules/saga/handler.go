// Package saga exposes the Saga Engine's observability endpoints: recent saga
// summaries and live orchestration state (the README Saga Orchestrator, state
// kept in Redis until completion). Body-only, consistent with the API contract.
package saga

import (
	"context"
	"errors"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/google/uuid"

	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/respond"
	sagacore "github.com/aeroxe/compliance-hub/backend/internal/saga"
)

// RegisterRoutes mounts saga endpoints under /api/v1/sagas.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	h := &Handler{engine: d.Saga}
	rg := g.Group("/sagas")
	rg.POST("/search", h.search)
	rg.POST("/get", h.get)
}

// Handler serves saga observability endpoints.
type Handler struct {
	engine *sagacore.Engine
}

// search lists recent sagas with optional type/status filters and a limit.
func (h *Handler) search(ctx context.Context, c *app.RequestContext) {
	var req struct {
		Type   string `json:"type"`
		Status string `json:"status"`
		Limit  int    `json:"limit"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	items := h.engine.Search(req.Type, req.Status, req.Limit)
	respond.OK(c, map[string]any{"items": items, "count": len(items)})
}

// get returns the live state of one saga identified by type + entity_id.
func (h *Handler) get(ctx context.Context, c *app.RequestContext) {
	var req struct {
		Type     string    `json:"type" vd:"$ != ''"`
		EntityID uuid.UUID `json:"entity_id" vd:"$ != uuid.Nil"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	s, err := h.engine.Get(ctx, req.Type, req.EntityID)
	if err != nil {
		respond.Internal(c, err)
		return
	}
	if s == nil {
		respond.NotFound(c, "not_found", errors.New("no saga found for the given type and entity"))
		return
	}
	respond.OK(c, s)
}
