package server

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/aeroxe/compliance-hub/backend/internal/config"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/metrics"
	"github.com/aeroxe/compliance-hub/backend/internal/modules/alert"
	"github.com/aeroxe/compliance-hub/backend/internal/modules/analytics"
	"github.com/aeroxe/compliance-hub/backend/internal/modules/audit"
	auditlogmodule "github.com/aeroxe/compliance-hub/backend/internal/modules/auditlog"
	authmodule "github.com/aeroxe/compliance-hub/backend/internal/modules/auth"
	"github.com/aeroxe/compliance-hub/backend/internal/modules/checklist"
	"github.com/aeroxe/compliance-hub/backend/internal/modules/compliance"
	"github.com/aeroxe/compliance-hub/backend/internal/modules/correctiveaction"
	"github.com/aeroxe/compliance-hub/backend/internal/modules/deadline"
	"github.com/aeroxe/compliance-hub/backend/internal/modules/notification"
	"github.com/aeroxe/compliance-hub/backend/internal/modules/options"
	"github.com/aeroxe/compliance-hub/backend/internal/modules/regulation"
	"github.com/aeroxe/compliance-hub/backend/internal/modules/reporting"
	rolemodule "github.com/aeroxe/compliance-hub/backend/internal/modules/role"
	sagamodule "github.com/aeroxe/compliance-hub/backend/internal/modules/saga"
	usermodule "github.com/aeroxe/compliance-hub/backend/internal/modules/user"
	"github.com/aeroxe/compliance-hub/backend/internal/modules/violation"
	"github.com/aeroxe/compliance-hub/backend/internal/permissions"
	"github.com/aeroxe/compliance-hub/backend/internal/rbac"
	"github.com/aeroxe/compliance-hub/backend/internal/respond"
	"github.com/aeroxe/compliance-hub/backend/internal/swagger"
)

// registerRoutes mounts every service module under /api/v1 plus the platform
// endpoints (/health, /metrics, /swagger, /events, /ws). Authentication and
// RBAC middleware run on every route; public routes (login, health, swagger,
// ws, metrics) are exempt inside the middleware, and every other route must be
// granted as an exact "METHOD path" permission to the caller's role.
func registerRoutes(h *server.Hertz, d deps.Deps, cfg *config.Config) {
	h.Use(rbac.Auth(cfg, d.Cache))
	h.Use(rbac.Permission(d.DB, d.Cache))

	api := h.Group("/api/v1")

	authmodule.RegisterRoutes(api, d, cfg)
	regulation.RegisterRoutes(api, d)
	compliance.RegisterRoutes(api, d)
	audit.RegisterRoutes(api, d)
	checklist.RegisterRoutes(api, d)
	auditlogmodule.RegisterRoutes(api, d)
	alert.RegisterRoutes(api, d)
	reporting.RegisterRoutes(api, d)
	analytics.RegisterRoutes(api, d)
	notification.RegisterRoutes(api, d)
	options.RegisterRoutes(api, d)
	violation.RegisterRoutes(api, d)
	correctiveaction.RegisterRoutes(api, d)
	deadline.RegisterRoutes(api, d)
	sagamodule.RegisterRoutes(api, d)
	usermodule.RegisterRoutes(api, d)
	rolemodule.RegisterRoutes(api, d)

	// Platform endpoints.
	h.POST("/events", eventsHandler(d))
	h.GET("/health", func(ctx context.Context, c *app.RequestContext) {
		respond.OK(c, map[string]any{"status": "ok", "service": "compliance-hub"})
	})
	h.GET("/metrics", metrics.Handler())

	// Swagger UI + runtime-generated OpenAPI spec.
	h.GET("/swagger/doc.json", func(ctx context.Context, c *app.RequestContext) {
		spec := swagger.GenerateSpec(permissions.Extract(h))
		c.Response.Header.SetContentType("application/json")
		c.Response.SetStatusCode(200)
		c.Response.SetBody(spec)
	})
	h.GET("/swagger/*any", swagger.Handler())

	// Readiness probe: real dependency checks (Postgres + cache), public so
	// orchestrators can gate traffic without credentials.
	h.GET("/health/ready", func(ctx context.Context, c *app.RequestContext) {
		checks := map[string]string{}
		ready := true
		sqlDB, err := d.DB.DB()
		if err != nil || sqlDB.PingContext(ctx) != nil {
			checks["database"] = "down"
			ready = false
		} else {
			checks["database"] = "ok"
		}
		if err := d.Cache.Ping(ctx); err != nil {
			checks["cache"] = "down"
			ready = false
		} else {
			checks["cache"] = "ok"
		}
		if ready {
			respond.OK(c, map[string]any{"status": "ready", "checks": checks})
			return
		}
		c.JSON(consts.StatusServiceUnavailable, map[string]any{
			"success": false, "status": "unavailable", "checks": checks,
		})
	})

	// The WebSocket handshake is public to rbac (browsers cannot send
	// Authorization headers), so the hub validates the bearer token itself
	// (Authorization header or Sec-WebSocket-Protocol: bearer.<token>) before
	// upgrading. The ?token= query parameter is not supported: tokens must
	// never appear in URLs where access logs would capture them.
	h.GET("/ws", func(ctx context.Context, c *app.RequestContext) {
		d.Hub.HandleHTTPAuth(ctx, c, cfg, d.Cache)
	})
}

// eventsHandler returns recent bus events (body-only: {"limit": n}).
func eventsHandler(d deps.Deps) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		limit := 50
		var req struct {
			Limit int `json:"limit"`
		}
		if err := c.BindAndValidate(&req); err == nil && req.Limit > 0 {
			if req.Limit > 200 {
				req.Limit = 200
			}
			limit = req.Limit
		}
		respond.OK(c, d.Bus.Recent(limit))
	}
}
