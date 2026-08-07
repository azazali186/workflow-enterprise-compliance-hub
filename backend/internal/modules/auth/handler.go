// Package authmodule implements the authentication endpoints: login (public),
// me and logout (authenticated). Token issuance, single-session cache checks
// and password hashing live in internal/auth.
package authmodule

import (
	"context"
	"errors"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/auditlog"
	"github.com/aeroxe/compliance-hub/backend/internal/auth"
	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/config"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/respond"
)

// RegisterRoutes mounts auth endpoints under /api/v1/auth.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps, cfg *config.Config) {
	h := &Handler{db: d.DB, cache: d.Cache, cfg: cfg}
	rg := g.Group("/auth")
	rg.POST("/login", h.login)
	rg.POST("/me", h.me)
	rg.POST("/logout", h.logout)
}

// Handler serves auth endpoints.
type Handler struct {
	db    *gorm.DB
	cache cache.Cache
	cfg   *config.Config
}

// LoginRequest is the body of POST /auth/login.
type LoginRequest struct {
	Username string `json:"username" vd:"$ != ''"`
	Password string `json:"password" vd:"$ != ''"`
}

func (h *Handler) login(ctx context.Context, c *app.RequestContext) {
	var req LoginRequest
	if err := c.BindAndValidate(&req); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	_, ip, userAgent := auditlog.FromRequest(c) // login is public: no pre-auth actor

	var user models.User
	err := h.db.WithContext(ctx).Preload("Role").Where("username = ? AND status = ?", req.Username, "active").First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		h.logLogin(ctx, c, auditlog.Entry{
			Action: "login", Status: "failure", EntityType: "user",
			ActorID: req.Username, IP: ip, UserAgent: userAgent,
			Metadata: map[string]any{"username": req.Username, "reason": "unknown or disabled user"},
		})
		respond.Unauthorized(c)
		return
	}
	if err != nil {
		respond.Internal(c, err)
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		h.logLogin(ctx, c, auditlog.Entry{
			Action: "login", Status: "failure", EntityType: "user",
			EntityID: user.ID, ActorID: user.ID.String(), IP: ip, UserAgent: userAgent,
			Metadata: map[string]any{"username": req.Username, "reason": "invalid password"},
		})
		respond.Unauthorized(c)
		return
	}

	roleCode := ""
	if user.Role != nil {
		roleCode = user.Role.Code
	}
	token, err := auth.IssueToken(h.cfg.JWTSecret, h.cfg.JWTExpiry, user.ID.String(), user.Username, roleCode)
	if err != nil {
		respond.Internal(c, err)
		return
	}
	if err := auth.StoreSession(ctx, h.cache, user.ID.String(), token, h.cfg.JWTExpiry); err != nil {
		respond.Internal(c, err)
		return
	}

	now := time.Now()
	_ = h.db.WithContext(ctx).Model(&user).Update("last_login_at", now)

	h.logLogin(ctx, c, auditlog.Entry{
		Action: "login", Status: "success", EntityType: "user",
		EntityID: user.ID, ActorID: user.ID.String(), IP: ip, UserAgent: userAgent,
		Metadata: map[string]any{"username": req.Username, "role_code": roleCode},
	})

	respond.OK(c, map[string]any{
		"access_token": token,
		"token_type":   "bearer",
		"expires_in":   int(h.cfg.JWTExpiry.Seconds()),
		"user": map[string]any{
			"id":         user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"role_id":    user.RoleID,
			"role_code":  roleCode,
			"role_name":  roleName(user.Role),
			"last_login": user.LastLoginAt,
		},
	})
}

// me returns the current user with their granted permissions.
func (h *Handler) me(ctx context.Context, c *app.RequestContext) {
	userID := c.GetString("auth_user_id")
	var user models.User
	err := h.db.WithContext(ctx).Preload("Role.Permissions").Where("id = ?", userID).First(&user).Error
	if err != nil {
		respond.NotFound(c, "not_found", err)
		return
	}
	permissions := make([]map[string]any, 0, len(user.Role.Permissions))
	for _, p := range user.Role.Permissions {
		permissions = append(permissions, map[string]any{"name": p.Name, "route": p.Route})
	}
	respond.OK(c, map[string]any{
		"user":        user,
		"role":        user.Role,
		"permissions": permissions,
	})
}

// logout invalidates the single-session token.
func (h *Handler) logout(ctx context.Context, c *app.RequestContext) {
	userID := c.GetString("auth_user_id")
	_ = h.cache.Del(ctx, auth.SessionKey(userID))
	respond.OK(c, map[string]any{"logged_out": true})
}

func roleName(r *models.Role) string {
	if r == nil {
		return ""
	}
	return r.Name
}

// logLogin writes a login audit entry; failures must not disturb the response.
func (h *Handler) logLogin(ctx context.Context, c *app.RequestContext, e auditlog.Entry) {
	if err := auditlog.Record(ctx, h.db, e); err != nil {
		// keep the login flow unbroken; the entry is best-effort
	}
}
