// Package rbac implements role-based access control: an authentication
// middleware (JWT + single-session cache check, mirroring the gateway pattern)
// and a permission middleware that verifies the caller's role is granted the
// exact "METHOD path" being requested. Roles and the bootstrap admin user are
// seeded at startup, after the route->permission sync.
package rbac

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/auth"
	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/config"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

// Role codes seeded at startup.
const (
	RoleAdmin             = "admin"
	RoleComplianceOfficer = "compliance_officer"
	RoleViewer            = "viewer"
)

// roleCacheTTL is how long a role's permission set is cached.
const roleCacheTTL = 15 * time.Minute

// roleCacheKey returns the cache key for a role's allowed routes.
func roleCacheKey(code string) string { return "rbac:role:" + code }

// PublicRoutes are exempt from authentication and permission checks.
func isPublic(method, path string) bool {
	switch {
	case method == "GET" && path == "/health":
		return true
	case method == "GET" && path == "/health/ready":
		return true
	case method == "GET" && path == "/metrics":
		return true
	case strings.HasPrefix(path, "/swagger"):
		return true
	case path == "/api/v1/auth/login":
		return true
	case path == "/ws":
		return true
	}
	return false
}

// Auth returns the authentication middleware. It skips public routes,
// validates the Bearer token, enforces single-session via the cached
// fingerprint and renews sessions that are close to expiry.
func Auth(cfg *config.Config, c cache.Cache) app.HandlerFunc {
	return func(ctx context.Context, rc *app.RequestContext) {
		if isPublic(string(rc.Method()), string(rc.Path())) {
			rc.Next(ctx)
			return
		}
		tokenStr, ok := bearerToken(rc)
		if !ok {
			unauthorized(rc)
			return
		}
		claims, err := auth.ParseToken(cfg.JWTSecret, tokenStr)
		if err != nil {
			unauthorized(rc)
			return
		}

		expected, ok := c.Get(ctx, auth.SessionKey(claims.UserID))
		if !ok || auth.SessionHash(tokenStr, claims.UserID) != expected {
			unauthorized(rc)
			return
		}

		auth.RenewIfNeeded(ctx, c, claims.UserID, expected, cfg.JWTExpiry)

		rc.Set("auth_user_id", claims.UserID)
		rc.Set("auth_username", claims.Username)
		rc.Set("auth_role_code", claims.RoleCode)
		rc.Next(ctx)
	}
}

// Permission returns the authorization middleware. The admin role bypasses the
// check; every other role must be granted the exact "METHOD path" route.
func Permission(db *gorm.DB, c cache.Cache) app.HandlerFunc {
	return func(ctx context.Context, rc *app.RequestContext) {
		if isPublic(string(rc.Method()), string(rc.Path())) {
			rc.Next(ctx)
			return
		}
		if rc.GetString("auth_role_code") == RoleAdmin {
			rc.Next(ctx)
			return
		}

		route := string(rc.Method()) + " " + string(rc.Path())
		roleCode := rc.GetString("auth_role_code")
		if roleCode == "" {
			unauthorized(rc)
			return
		}

		allowed, err := roleAllows(ctx, db, c, roleCode, route)
		if err != nil {
			slog.Error("rbac check failed", "role", roleCode, "route", route, "error", err)
			rc.SetStatusCode(consts.StatusInternalServerError)
			rc.Abort()
			return
		}
		if !allowed {
			forbidden(rc, route)
			return
		}
		rc.Next(ctx)
	}
}

// roleAllows loads a role's granted routes (cached) and checks membership.
func roleAllows(ctx context.Context, db *gorm.DB, c cache.Cache, roleCode, route string) (bool, error) {
	key := roleCacheKey(roleCode)
	if raw, ok := c.Get(ctx, key); ok {
		var routes []string
		if json.Unmarshal([]byte(raw), &routes) == nil {
			return contains(routes, route), nil
		}
	}

	var role models.Role
	if err := db.WithContext(ctx).Preload("Permissions").Where("code = ?", roleCode).First(&role).Error; err != nil {
		return false, err
	}
	routes := make([]string, 0, len(role.Permissions))
	for _, p := range role.Permissions {
		routes = append(routes, p.Route)
	}
	if b, err := json.Marshal(routes); err == nil {
		_ = c.Set(ctx, key, string(b), roleCacheTTL)
	}
	return contains(routes, route), nil
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// bearerToken extracts the token from the Authorization header.
func bearerToken(rc *app.RequestContext) (string, bool) {
	authorization := string(rc.GetHeader("Authorization"))
	if len(authorization) <= 7 || !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		return "", false
	}
	return strings.TrimSpace(authorization[7:]), true
}

func unauthorized(rc *app.RequestContext) {
	rc.JSON(consts.StatusUnauthorized, map[string]any{
		"success": false, "error": "unauthorized", "code": "unauthorized",
		"message": "missing or invalid token",
	})
	rc.Abort()
}

func forbidden(rc *app.RequestContext, route string) {
	rc.JSON(consts.StatusForbidden, map[string]any{
		"success": false, "error": "forbidden", "code": "forbidden",
		"message": "role does not have permission for " + route,
	})
	rc.Abort()
}

// Seed ensures the default roles exist, assigns their route-level permissions
// from the current permission table, and creates the bootstrap admin user.
func Seed(ctx context.Context, d deps.Deps, adminUsername, adminPassword string) error {
	perms := []models.Permission{}
	if err := d.DB.WithContext(ctx).Find(&perms).Error; err != nil {
		return err
	}

	admin, err := ensureRole(ctx, d.DB, "Administrator", RoleAdmin, "Full access to every API route", perms)
	if err != nil {
		return err
	}
	officerPerms := filterPermissions(perms, func(p models.Permission) bool {
		return p.Method != "DELETE"
	})
	officer, err := ensureRole(ctx, d.DB, "Compliance Officer", RoleComplianceOfficer, "Manage compliance data (no deletions)", officerPerms)
	if err != nil {
		return err
	}
	viewerPerms := filterPermissions(perms, isReadRoute)
	if _, err := ensureRole(ctx, d.DB, "Viewer", RoleViewer, "Read-only lists, analytics and events", viewerPerms); err != nil {
		return err
	}

	if err := ensureAdminUser(ctx, d.DB, d.Cache, adminUsername, adminPassword, admin.ID); err != nil {
		return err
	}
	_ = officer // assigned above; kept for clarity

	// Drop cached role permission sets so the seeded data is authoritative.
	_ = d.Cache.Del(ctx, roleCacheKey(RoleAdmin))
	_ = d.Cache.Del(ctx, roleCacheKey(RoleComplianceOfficer))
	_ = d.Cache.Del(ctx, roleCacheKey(RoleViewer))
	return nil
}

// ensureRole upserts a role and replaces its permission association.
func ensureRole(ctx context.Context, db *gorm.DB, name, code, description string, perms []models.Permission) (*models.Role, error) {
	var role models.Role
	err := db.WithContext(ctx).Where("code = ?", code).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		role = models.Role{Name: name, Code: code, Description: description}
		if err := db.WithContext(ctx).Create(&role).Error; err != nil {
			return nil, err
		}
		slog.Info("role created", "code", code)
	} else if err != nil {
		return nil, err
	} else {
		role.Name = name
		role.Description = description
		if err := db.WithContext(ctx).Save(&role).Error; err != nil {
			return nil, err
		}
	}
	if err := db.WithContext(ctx).Model(&role).Association("Permissions").Replace(perms); err != nil {
		return nil, err
	}
	return &role, nil
}

// ensureAdminUser creates the bootstrap admin when it does not exist.
func ensureAdminUser(ctx context.Context, db *gorm.DB, c cache.Cache, username, password string, roleID uuid.UUID) error {
	var existing models.User
	err := db.WithContext(ctx).Where("username = ?", username).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	user := models.User{Username: username, PasswordHash: hash, RoleID: roleID, Status: "active"}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		return err
	}
	slog.Info("admin user created", "username", username)
	_ = c.Del(ctx, auth.SessionKey(user.ID.String()))
	return nil
}

// filterPermissions selects permissions matching a predicate.
func filterPermissions(perms []models.Permission, keep func(models.Permission) bool) []models.Permission {
	out := make([]models.Permission, 0, len(perms))
	for _, p := range perms {
		if keep(p) {
			out = append(out, p)
		}
	}
	return out
}

// isReadRoute reports whether a permission is read-only (search/get/analytics/events).
// The viewer role is granted exactly these; own-profile and logout are included
// so a read-only user can still manage their own session.
func isReadRoute(p models.Permission) bool {
	switch {
	case strings.HasPrefix(p.Path, "/api/v1/users"):
		// User administration (profiles, roles, emails) stays with admins and
		// officers; the read-only viewer role never sees /api/v1/users.
		return false
	case strings.HasSuffix(p.Path, "/search"), strings.HasSuffix(p.Path, "/get"):
		return true
	case strings.HasPrefix(p.Path, "/api/v1/analytics"):
		return true
	case p.Path == "/events" || p.Path == "/api/v1/notifications/events":
		return true
	case p.Path == "/api/v1/auth/me" || p.Path == "/api/v1/auth/logout":
		return true
	}
	return false
}
