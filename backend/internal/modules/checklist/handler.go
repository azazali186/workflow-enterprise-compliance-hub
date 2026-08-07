// Package checklist implements ChecklistService. Body-only API.
package checklist

import (
	"github.com/cloudwego/hertz/pkg/route"

	"github.com/aeroxe/compliance-hub/backend/internal/crud"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/repository"
)

// RegisterRoutes mounts ChecklistService endpoints under /api/v1/checklists.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	repo := repository.New[models.Checklist](d.DB, d.Cache, "checklist")
	resource := &crud.Resource[models.Checklist]{
		Repo:   repo,
		Bus:    d.Bus,
		DB:     d.DB,
		Slug:   "checklist",
		Audit:  true,
		Events: map[string]string{"created": "checklist.created", "updated": "checklist.updated"},
		SearchColumns: map[string]string{
			"title":         "title",
			"status":        "status",
			"compliance_id": "compliance_id",
			"owner_id":      "owner_id",
			"due_date":      "due_date",
			"created_at":    "created_at",
			"updated_at":    "updated_at",
		},
		SummaryBy: "status",
	}
	h := resource.Handlers()

	rg := g.Group("/checklists")
	rg.POST("", h.Create)
	rg.POST("/search", h.Search)
	rg.POST("/get", h.GetByID)
	rg.PATCH("", h.Update)
	rg.DELETE("", h.Delete)
}
