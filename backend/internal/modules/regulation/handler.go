// Package regulation implements RegulationService. Body-only API: POST
// create, POST /search (cursor pagination), POST /get, PATCH update,
// DELETE remove. No path variables, no query params.
package regulation

import (
	"github.com/cloudwego/hertz/pkg/route"

	"github.com/aeroxe/compliance-hub/backend/internal/crud"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/repository"
)

// RegisterRoutes mounts RegulationService endpoints under /api/v1/regulations.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	repo := repository.New[models.Regulation](d.DB, d.Cache, "regulation")
	resource := &crud.Resource[models.Regulation]{
		Repo:   repo,
		Bus:    d.Bus,
		DB:     d.DB,
		Slug:   "regulation",
		Audit:  true,
		Events: map[string]string{"created": "regulation.created", "updated": "regulation.updated"},
		SearchColumns: map[string]string{
			"title":          "title",
			"code":           "code",
			"jurisdiction":   "jurisdiction",
			"status":         "status",
			"effective_date": "effective_date",
			"expiry_date":    "expiry_date",
			"created_at":     "created_at",
			"updated_at":     "updated_at",
		},
		SummaryBy: "status",
	}
	h := resource.Handlers()

	rg := g.Group("/regulations")
	rg.POST("", h.Create)
	rg.POST("/search", h.Search)
	rg.POST("/get", h.GetByID)
	rg.PATCH("", h.Update)
	rg.DELETE("", h.Delete)
}
