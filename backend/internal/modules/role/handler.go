// Package role implements the read-only role query endpoints used by the
// admin UI (role dropdowns for user management). Roles themselves are seeded
// and managed by rbac.Seed — there is intentionally no create/update/delete
// here, so a misconfiguration can never alter the role hierarchy at runtime.
package role

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

// RegisterRoutes mounts role query endpoints under /api/v1/roles.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	repo := repository.New[models.Role](d.DB, d.Cache, "role")
	h := &Handler{repo: repo}

	rg := g.Group("/roles")
	rg.POST("/search", h.search)
	rg.POST("/get", h.get)
}

// Handler serves role query endpoints.
type Handler struct {
	repo *repository.Repository[models.Role]
}

// roleColumns exposes the queryable columns for the pagination engine.
var roleColumns = map[string]string{
	"name":        "name",
	"code":        "code",
	"description": "description",
	"created_at":  "created_at",
	"updated_at":  "updated_at",
}

func (h *Handler) search(ctx context.Context, c *app.RequestContext) {
	var q pagination.Query
	if err := json.Unmarshal(c.Request.Body(), &q); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	result, err := pagination.Apply[models.Role](h.repo.Scope(), q, roleColumns, "code")
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
	r, err := h.repo.GetByID(ctx, req.ID)
	if errors.Is(err, repository.ErrNotFound) {
		respond.NotFound(c, "not_found", err)
		return
	}
	if err != nil {
		respond.Internal(c, err)
		return
	}
	respond.OK(c, *r)
}
