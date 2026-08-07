// Package auditlog implements the AuditLogService: query endpoints over the
// audit trail (login logs, CRUD history, lifecycle actions) with cursor
// pagination, filters, date ranges and an action summary.
package auditlog

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/google/uuid"

	"github.com/aeroxe/compliance-hub/backend/internal/crud"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/pagination"
	"github.com/aeroxe/compliance-hub/backend/internal/repository"
	"github.com/aeroxe/compliance-hub/backend/internal/respond"
)

// RegisterRoutes mounts audit-log endpoints under /api/v1/audit-logs.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	repo := repository.New[models.AuditLog](d.DB, d.Cache, "auditlog")
	h := &Handler{repo: repo}

	rg := g.Group("/audit-logs")
	rg.POST("/search", h.search)
	rg.POST("/get", h.get)
}

// Handler serves audit-log query endpoints.
type Handler struct {
	repo *repository.Repository[models.AuditLog]
}

// auditLogColumns exposes the queryable columns for the pagination engine.
var auditLogColumns = map[string]string{
	"action":      "action",
	"status":      "status",
	"entity_type": "entity_type",
	"entity_id":   "entity_id",
	"actor_id":    "actor_id",
	"ip":          "ip",
	"created_at":  "created_at",
	"updated_at":  "updated_at",
}

func (h *Handler) search(ctx context.Context, c *app.RequestContext) {
	var q pagination.Query
	if err := json.Unmarshal(c.Request.Body(), &q); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	result, err := pagination.Apply[models.AuditLog](h.repo.Scope(), q, auditLogColumns, "action")
	if err != nil {
		respond.BadRequest(c, "invalid_search", err)
		return
	}
	respond.OK(c, result)
}

func (h *Handler) get(ctx context.Context, c *app.RequestContext) {
	var req crud.IDRequest
	if err := c.BindAndValidate(&req); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	if req.ID == uuid.Nil {
		respond.BadRequest(c, "invalid_body", errors.New("field \"id\" is required"))
		return
	}
	e, err := h.repo.GetByID(ctx, req.ID)
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
