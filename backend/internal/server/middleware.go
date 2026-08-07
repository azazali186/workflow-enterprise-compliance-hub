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

// requestID assigns a UUID to every request and echoes it back.
func requestID() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		rid := string(c.GetHeader("X-Request-ID"))
		if rid == "" {
			rid = uuid.NewString()
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

// cors allows cross-origin browser clients (React dashboard, etc).
func cors() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		origin := string(c.GetHeader("Origin"))
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		} else {
			c.Header("Access-Control-Allow-Origin", "*")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID, X-Request-ID")
		c.Header("Access-Control-Max-Age", "86400")

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
