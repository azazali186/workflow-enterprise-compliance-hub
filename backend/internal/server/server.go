// Package server wires the Hertz HTTP server: middleware stack, module routes
// and the WebSocket hub.
package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/aeroxe/compliance-hub/backend/internal/config"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/lock"
	"github.com/aeroxe/compliance-hub/backend/internal/metrics"
	"github.com/aeroxe/compliance-hub/backend/internal/modules/deadline"
	"github.com/aeroxe/compliance-hub/backend/internal/outbox"
)

// New builds and configures the Hertz server with the full middleware stack
// and module routes. The returned server is not started until Run is called.
func New(cfg *config.Config, d deps.Deps) *server.Hertz {
	h := server.New(
		server.WithHostPorts(cfg.ServerAddr),
		server.WithMaxRequestBodySize(cfg.MaxBodySize),
	)

	h.Use(recovery())
	h.Use(requestID())
	h.Use(logging(d.Logger))
	h.Use(cors(cfg))
	h.Use(rateLimit(d.Cache, cfg.RateLimitPerMinute))
	h.Use(metrics.Middleware())

	registerRoutes(h, d, cfg)
	return h
}

// Run starts the server and its background workers: the outbox dispatcher and
// the deadline evaluator (both guarded by a distributed lock so replicas never
// double-process). It blocks until the process receives a termination signal.
func Run(ctx context.Context, cfg *config.Config, d deps.Deps, srv *server.Hertz) {
	l := lock.New(ctx, cfg.RedisURL)

	if cfg.DeadlineJobInterval > 0 {
		deadline.RunJob(ctx, d, l, cfg.DeadlineJobInterval)
	}

	dispatcher := outbox.NewDispatcher(d.DB, d.Bus, l, cfg.OutboxPollInterval)
	go dispatcher.Run(ctx)

	// The saga engine orchestrates the four README business sagas from the
	// events flowing through the bus, with state in Redis.
	if d.Saga != nil {
		go d.Saga.Run(ctx)
	}

	// Keep the outbox deliveries gauge fresh for Prometheus.
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				metrics.SetOutboxDeliveries(dispatcher.Deliveries())
			}
		}
	}()

	go func() {
		<-ctx.Done()
		slog.Info("shutting down...")
		_ = d.Bus.Close()
		_ = srv.Shutdown(context.Background())
	}()

	slog.Info("compliance-hub server listening", "addr", cfg.ServerAddr)
	srv.Spin()
}
