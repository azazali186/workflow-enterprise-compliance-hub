// Package cache provides a small caching abstraction with two backends:
// Redis (preferred) and an in-process TTL map (fallback). Cache keys follow
// the README convention `cache:<slug>:<id>` with a 15 minute TTL.
package cache

import (
	"context"
	"log/slog"
	"time"
)

// TTL is the default cache lifetime for entity reads (README: 15m).
const TTL = 15 * time.Minute

// Cache is the minimal interface used by the rest of the application.
type Cache interface {
	Get(ctx context.Context, key string) (string, bool)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	// SetNX atomically sets a key only if it does not already exist and
	// reports whether the caller won the race. Used for distributed claims
	// (e.g. saga instance creation across replicas).
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	Del(ctx context.Context, key string) error
	// TTL returns the remaining lifetime of a key, if present.
	TTL(ctx context.Context, key string) (time.Duration, bool)
	// Keys lists keys matching a glob pattern (used by the saga timeout
	// sweeper to find in-flight state).
	Keys(ctx context.Context, pattern string) ([]string, error)
	// Ping verifies the backend is reachable (readiness probe).
	Ping(ctx context.Context) error
}

// New creates the configured cache backend. It never returns nil: when Redis
// is unavailable or not configured it silently falls back to the in-memory
// implementation so the server can still boot.
func New(ctx context.Context, redisURL string) Cache {
	if redisURL != "" {
		if r, err := newRedis(redisURL); err == nil {
			if err := r.Ping(ctx); err == nil {
				slog.Info("cache backend: redis", "url", redisURL)
				return r
			}
			slog.Warn("redis unavailable, falling back to in-memory cache", "error", err)
		} else {
			slog.Warn("redis unavailable, falling back to in-memory cache", "error", err)
		}
	}
	slog.Info("cache backend: in-memory")
	return newMemory()
}

// EntityKey builds a `cache:<slug>:<id>` key.
func EntityKey(slug, id string) string {
	return "cache:" + slug + ":" + id
}
