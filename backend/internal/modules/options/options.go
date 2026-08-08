// Package options serves a single body-only endpoint (POST /api/v1/options)
// that returns lightweight {id, name[, sub]} pairs for every entity the UI
// needs in dropdowns. Clients ask for one or many entities in one request and
// can filter by a case-insensitive search term and/or by an explicit id set
// (used to resolve the stored value of an edit form). All queries are built
// with GORM — no raw SQL.
package options

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/respond"
)

// Item is a single dropdown option. Sub carries an optional secondary label
// (email for users, code for roles) rendered beneath the name in the UI.
type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Sub  string `json:"sub,omitempty"`
}

// source describes how to query one entity for options.
type source struct {
	model   any
	nameCol string
	subCol  string
}

// registry maps the public (snake_case) entity key to its query source. The
// column names are compile-time constants, so the SELECT/LIKE/ORDER strings
// below can never be attacker-controlled.
var registry = map[string]source{
	"users":              {model: &models.User{}, nameCol: "username", subCol: "email"},
	"roles":              {model: &models.Role{}, nameCol: "name", subCol: "code"},
	"compliances":        {model: &models.Compliance{}, nameCol: "name"},
	"regulations":        {model: &models.Regulation{}, nameCol: "title"},
	"violations":         {model: &models.Violation{}, nameCol: "title"},
	"checklists":         {model: &models.Checklist{}, nameCol: "title"},
	"deadlines":          {model: &models.Deadline{}, nameCol: "title"},
	"corrective_actions": {model: &models.CorrectiveAction{}, nameCol: "title"},
	"audits":             {model: &models.Audit{}, nameCol: "title"},
	"reports":            {model: &models.Report{}, nameCol: "title"},
}

// entityRoutePrefix maps an option entity key back to its API route prefix.
// The viewer role is deliberately excluded from identity-bearing options
// (users, roles), mirroring its lack of /api/v1/users access.
var restrictedForViewer = map[string]bool{
	"users": true,
	"roles": true,
}

// request is the body contract:
// {"entities": ["compliances", ...], "search": "iso", "limit": 50, "ids": {...}}.
type request struct {
	Entities []string            `json:"entities"`
	Search   string              `json:"search"`
	Limit    int                 `json:"limit"`
	IDs      map[string][]string `json:"ids"`
}

// RegisterRoutes mounts POST /api/v1/options.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	h := &Handler{db: d.DB}
	rg := g.Group("/options")
	rg.POST("", h.Options)
}

// Handler serves dropdown options.
type Handler struct {
	db *gorm.DB
}

// Options returns the requested entity options. Response shape:
// {"compliances": [{id, name}], "regulations": [...]}.
func (h *Handler) Options(ctx context.Context, c *app.RequestContext) {
	var req request
	if err := c.BindAndValidate(&req); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	// Identity-bearing entities stay out of the viewer role's reach.
	role := c.GetString("auth_role_code")
	allowed := func(key string) bool {
		return role != "viewer" || !restrictedForViewer[key]
	}

	out := make(map[string][]Item, len(req.Entities))
	seen := make(map[string]bool, len(req.Entities))
	for _, key := range req.Entities {
		if seen[key] || !allowed(key) {
			continue
		}
		seen[key] = true
		items, err := h.fetch(ctx, key, req.IDs[key], req.Search, limit)
		if err != nil {
			respond.Internal(c, err)
			return
		}
		if items != nil {
			out[key] = items
		}
	}
	respond.OK(c, out)
}

// fetch resolves options for one entity key: requested ids first (so edit
// forms always see their stored value), then a case-insensitive name search,
// deduplicated by id and capped at limit.
func (h *Handler) fetch(ctx context.Context, key string, ids []string, search string, limit int) ([]Item, error) {
	src, ok := registry[key]
	if !ok {
		return nil, nil
	}
	selectCols := "id AS id, " + src.nameCol + " AS name"
	if src.subCol != "" {
		selectCols += ", " + src.subCol + " AS sub"
	}

	seen := make(map[string]struct{}, limit)
	items := make([]Item, 0, limit)

	if len(ids) > 0 {
		var rows []Item
		if err := h.db.WithContext(ctx).Model(src.model).
			Select(selectCols).
			Where("id IN ?", ids).
			Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			if _, dup := seen[r.ID]; dup {
				continue
			}
			seen[r.ID] = struct{}{}
			items = append(items, r)
		}
	}

	// The list query returns the first `limit` rows by name — filtered by the
	// case-insensitive search when provided, otherwise the full head of the
	// list so an empty search still populates the dropdown. It is skipped
	// entirely for ids-only requests (edit-form value resolution), which must
	// return exactly the requested ids.
	if len(ids) == 0 || search != "" {
		q := h.db.WithContext(ctx).Model(src.model).Select(selectCols)
		if search != "" {
			q = q.Where("LOWER("+src.nameCol+") LIKE LOWER(?)", "%"+search+"%")
		}
		var rows []Item
		if err := q.Order(src.nameCol + " ASC").Limit(limit).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			if _, dup := seen[r.ID]; dup {
				continue
			}
			seen[r.ID] = struct{}{}
			items = append(items, r)
		}
	}

	// The limit caps the whole response, not just the search query — ids and
	// search results are merged, so trim the combined slice to the contract.
	if len(items) > limit {
		items = items[:limit]
	}
	if len(items) == 0 {
		return []Item{}, nil
	}
	return items, nil
}
