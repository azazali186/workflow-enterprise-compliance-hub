package server

import (
	"context"
	"log/slog"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"

	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/config"
)

// recovery recovers panics and returns a 500 instead of crashing the process.
func recovery() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered", "path", string(c.Path()), "panic", r, "stack", string(debug.Stack()))
				c.AbortWithStatus(consts.StatusInternalServerError)
			}
		}()
		c.Next(ctx)
	}
}

// requestID assigns a UUID v7 to every request and echoes it back.
func requestID() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		rid := string(c.GetHeader("X-Request-ID"))
		if rid == "" {
			if id, err := uuid.NewV7(); err == nil {
				rid = id.String()
			} else {
				rid = uuid.NewString() // unreachable in practice; keeps the header set
			}
		}
		c.Header("X-Request-ID", rid)
		c.Next(ctx)
	}
}

// logging emits one structured log line per request.
func logging(logger *slog.Logger) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		c.Next(ctx)
		logger.Info("http",
			"method", string(c.Method()),
			"path", string(c.Path()),
			"status", c.Response.StatusCode(),
			"latency", time.Since(start).String(),
			"ip", c.ClientIP(),
			"request_id", string(c.GetHeader("X-Request-ID")),
		)
	}
}

// cors handles cross-origin browser clients behind an explicit allowlist.
//
// Security posture: when CORS_ALLOWED_ORIGINS is configured, only those
// origins are reflected and credentials (cookies / Authorization) are
// allowed. Without an allowlist the API answers "*" with credentials
// disabled — reflecting any origin with Allow-Credentials would let any site
// read authenticated responses, so that combination is never emitted.
func cors(cfg *config.Config) app.HandlerFunc {
	allowed := make(map[string]bool, len(cfg.CORSAllowedOrigins))
	for _, o := range cfg.CORSAllowedOrigins {
		allowed[o] = true
	}
	return func(ctx context.Context, c *app.RequestContext) {
		origin := string(c.GetHeader("Origin"))
		switch {
		case origin != "" && allowed[origin]:
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		case origin == "":
			// Non-browser clients (curl, services) need no CORS headers.
		default:
			// Disallowed origin: no CORS headers, so the browser blocks the
			// response from being read.
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID, X-Request-ID")
		c.Header("Access-Control-Max-Age", "86400")
		// The response varies by Origin: shared caches must not serve a
		// credentialed response to a disallowed origin.
		c.Header("Vary", "Origin")

		if string(c.Method()) == "OPTIONS" {
			c.AbortWithStatus(consts.StatusNoContent)
			return
		}
		c.Next(ctx)
	}
}

// rateLimit caps each IP at `max` requests per minute using the cache
// (README: `ratelimit:<ip>` with 1m TTL).
func rateLimit(cache cache.Cache, max int) app.HandlerFunc {
	if max <= 0 {
		max = 60
	}
	return func(ctx context.Context, c *app.RequestContext) {
		ip := c.ClientIP()
		key := "ratelimit:" + ip
		count := 1
		if v, ok := cache.Get(ctx, key); ok {
			if n, err := strconv.Atoi(v); err == nil {
				count = n + 1
			}
		}
		if count > max {
			c.AbortWithStatus(consts.StatusTooManyRequests)
			return
		}
		_ = cache.Set(ctx, key, strconv.Itoa(count), time.Minute)
		c.Next(ctx)
	}
}
