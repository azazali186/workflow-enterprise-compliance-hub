// Package usermodule implements user administration: create, update (including
// password reset and role assignment), soft delete, get and cursor-paginated
// search. All endpoints are body-only (no path variables, no query params)
// and every mutation writes an audit entry. Passwords are never returned or
// logged — they are bcrypt-hashed and the model carries json:"-".
//
// Route-level authorization comes from the auto-seeded permission table: the
// admin role has every route, the compliance officer all non-delete routes,
// and the viewer role is excluded from /api/v1/users entirely (see
// rbac.isReadRoute), so user data stays with administrators.
package usermodule

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/auditlog"
	"github.com/aeroxe/compliance-hub/backend/internal/auth"
	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/crud"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/pagination"
	"github.com/aeroxe/compliance-hub/backend/internal/repository"
	"github.com/aeroxe/compliance-hub/backend/internal/respond"
)

// RegisterRoutes mounts user endpoints under /api/v1/users.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	repo := repository.New[models.User](d.DB, d.Cache, "user")
	h := &Handler{db: d.DB, cache: d.Cache, repo: repo}

	rg := g.Group("/users")
	rg.POST("", h.create)
	rg.POST("/search", h.search)
	rg.POST("/get", h.get)
	rg.PATCH("", h.update)
	rg.DELETE("", h.delete)
}

// Handler serves user administration endpoints.
type Handler struct {
	db    *gorm.DB
	cache cache.Cache
	repo  *repository.Repository[models.User]
}

// userColumns exposes the queryable columns for the pagination engine.
var userColumns = map[string]string{
	"username":   "username",
	"email":      "email",
	"role_id":    "role_id",
	"status":     "status",
	"created_at": "created_at",
	"updated_at": "updated_at",
}

// UserCreateRequest is the body of POST /users.
type UserCreateRequest struct {
	Username string    `json:"username" vd:"$ != ''"`
	Email    string    `json:"email"`
	Password string    `json:"password" vd:"$ != ''"`
	RoleID   uuid.UUID `json:"role_id"`
	Status   string    `json:"status"`
}

func (h *Handler) create(ctx context.Context, c *app.RequestContext) {
	var req UserCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	if len(req.Password) < 8 {
		respond.BadRequest(c, "invalid_body", errors.New("password must be at least 8 characters"))
		return
	}
	if req.RoleID == uuid.Nil {
		respond.BadRequest(c, "invalid_body", errors.New("field \"role_id\" is required"))
		return
	}
	var role models.Role
	if err := h.db.WithContext(ctx).Where("id = ?", req.RoleID).First(&role).Error; err != nil {
		respond.BadRequest(c, "invalid_role", errors.New("role does not exist"))
		return
	}
	var existing models.User
	if err := h.db.WithContext(ctx).Where("username = ?", req.Username).First(&existing).Error; err == nil {
		respond.Conflict(c, "username_taken", errors.New("username already exists"))
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		respond.Internal(c, err)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respond.Internal(c, err)
		return
	}
	status := req.Status
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "disabled" {
		respond.BadRequest(c, "invalid_body", errors.New("status must be \"active\" or \"disabled\""))
		return
	}
	user := models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		RoleID:       req.RoleID,
		Status:       status,
	}
	if err := h.db.WithContext(ctx).Create(&user).Error; err != nil {
		// The pre-check above is best-effort; the unique index on username is
		// the authoritative guard, so a concurrent duplicate surfaces here as a
		// constraint violation — report it as a conflict, never a 500.
		if isDuplicateKey(err) {
			respond.Conflict(c, "username_taken", errors.New("username already exists"))
			return
		}
		respond.Internal(c, err)
		return
	}
	h.audit(ctx, c, "create", nil, &user)
	respond.Created(c, user)
}

// isDuplicateKey reports whether a GORM error is a unique-constraint
// violation, across the drivers in use: the postgres dialector translates
// SQLSTATE 23505 to gorm.ErrDuplicatedKey when TranslateError is enabled, and
// the pure-Go sqlite driver reports a raw message.
func isDuplicateKey(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "unique constraint failed")
}

// UserUpdateRequest is the body of PATCH /users (all fields optional except id).
type UserUpdateRequest struct {
	ID       uuid.UUID  `json:"id"`
	Username *string    `json:"username"`
	Email    *string    `json:"email"`
	Password *string    `json:"password"`
	RoleID   *uuid.UUID `json:"role_id"`
	Status   *string    `json:"status"`
}

func (h *Handler) update(ctx context.Context, c *app.RequestContext) {
	var req UserUpdateRequest
	if err := c.BindAndValidate(&req); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	if req.ID == uuid.Nil {
		respond.BadRequest(c, "invalid_body", errors.New("field \"id\" is required"))
		return
	}
	before, err := h.repo.GetByID(ctx, req.ID)
	if errors.Is(err, repository.ErrNotFound) {
		respond.NotFound(c, "not_found", err)
		return
	}
	if err != nil {
		respond.Internal(c, err)
		return
	}

	user := *before
	if req.Username != nil && *req.Username != "" {
		var clash models.User
		if err := h.db.WithContext(ctx).Where("username = ? AND id <> ?", *req.Username, req.ID).First(&clash).Error; err == nil {
			respond.Conflict(c, "username_taken", errors.New("username already exists"))
			return
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			respond.Internal(c, err)
			return
		}
		user.Username = *req.Username
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Password != nil && *req.Password != "" {
		if len(*req.Password) < 8 {
			respond.BadRequest(c, "invalid_body", errors.New("password must be at least 8 characters"))
			return
		}
		hash, err := auth.HashPassword(*req.Password)
		if err != nil {
			respond.Internal(c, err)
			return
		}
		user.PasswordHash = hash
		// A password change invalidates the single active session.
		_ = h.cache.Del(ctx, auth.SessionKey(req.ID.String()))
	}
	if req.RoleID != nil && *req.RoleID != uuid.Nil {
		var role models.Role
		if err := h.db.WithContext(ctx).Where("id = ?", *req.RoleID).First(&role).Error; err != nil {
			respond.BadRequest(c, "invalid_role", errors.New("role does not exist"))
			return
		}
		user.RoleID = *req.RoleID
	}
	if req.Status != nil && *req.Status != "" {
		if *req.Status != "active" && *req.Status != "disabled" {
			respond.BadRequest(c, "invalid_body", errors.New("status must be \"active\" or \"disabled\""))
			return
		}
		user.Status = *req.Status
	}

	if err := h.db.WithContext(ctx).Save(&user).Error; err != nil {
		respond.Internal(c, err)
		return
	}
	after := user
	h.audit(ctx, c, "update", before, &after)
	respond.OK(c, user)
}

func (h *Handler) delete(ctx context.Context, c *app.RequestContext) {
	id, ok := crud.BindID(c)
	if !ok {
		return
	}
	// An admin must not be able to remove their own account mid-session.
	if c.GetString("auth_user_id") == id.String() {
		respond.BadRequest(c, "invalid_body", errors.New("cannot delete your own account"))
		return
	}
	before, err := h.repo.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		respond.NotFound(c, "not_found", err)
		return
	}
	if err != nil {
		respond.Internal(c, err)
		return
	}
	if err := h.repo.Delete(ctx, id); err != nil {
		respond.Internal(c, err)
		return
	}
	_ = h.cache.Del(ctx, auth.SessionKey(id.String()))
	h.audit(ctx, c, "delete", before, nil)
	respond.NoContent(c)
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
	var user models.User
	err := h.db.WithContext(ctx).Preload("Role").Where("id = ?", req.ID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respond.NotFound(c, "not_found", err)
		return
	}
	if err != nil {
		respond.Internal(c, err)
		return
	}
	respond.OK(c, user)
}

func (h *Handler) search(ctx context.Context, c *app.RequestContext) {
	var q pagination.Query
	if err := json.Unmarshal(c.Request.Body(), &q); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	result, err := pagination.Apply[models.User](h.repo.Scope(), q, userColumns, "status")
	if err != nil {
		respond.BadRequest(c, "invalid_search", err)
		return
	}
	respond.OK(c, result)
}

// audit writes a best-effort audit entry for a user mutation. Actor/request
// info comes from the auth middleware; the diff engine redacts password keys
// automatically.
func (h *Handler) audit(ctx context.Context, c *app.RequestContext, action string, before, after *models.User) {
	e := auditlog.Entry{
		Action:     action,
		EntityType: "user",
		Before:     before,
		After:      after,
	}
	e.ActorID, e.IP, e.UserAgent = auditlog.FromRequest(c)
	if after != nil {
		e.EntityID = after.ID
	} else if before != nil {
		e.EntityID = before.ID
	}
	if err := auditlog.Record(ctx, h.db, e); err != nil {
		slog.Warn("audit log write failed", "resource", "user", "action", action, "error", err)
	}
}
