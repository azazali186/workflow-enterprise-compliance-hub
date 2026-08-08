// Package server wires the Hertz HTTP server: middleware stack, module routes
// and the WebSocket hub.
package server

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/aeroxe/compliance-hub/backend/internal/auditlog"
	"github.com/aeroxe/compliance-hub/backend/internal/config"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/lock"
	"github.com/aeroxe/compliance-hub/backend/internal/metrics"
	"github.com/aeroxe/compliance-hub/backend/internal/modules/deadline"
	"github.com/aeroxe/compliance-hub/backend/internal/outbox"
)

// shutdownTimeout bounds how long graceful shutdown waits for background
// workers to drain after the HTTP server stops accepting requests.
const shutdownTimeout = 10 * time.Second

// httpDrainTimeout is the budget for stopping the HTTP server itself; the
// worker drain gets its own (larger) window so a slow request drain cannot
// starve in-flight outbox/saga batches.
const httpDrainTimeout = 5 * time.Second

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

// Run starts the server and its background workers: the outbox dispatcher,
// the saga engine, the deadline evaluator and the audit-retention job (all
// guarded by a distributed lock so replicas never double-process). It blocks
// until the process receives a termination signal, then drains workers within
// a bounded window so in-flight dispatch is not cut mid-batch.
func Run(ctx context.Context, cfg *config.Config, d deps.Deps, srv *server.Hertz) {
	l := lock.New(ctx, cfg.RedisURL)
	var wg sync.WaitGroup

	if cfg.DeadlineJobInterval > 0 {
		deadline.RunJob(ctx, d, l, cfg.DeadlineJobInterval, &wg)
	}
	auditlog.RunRetentionJob(ctx, d.DB, l, cfg.AuditRetentionInterval, cfg.AuditRetentionDays, &wg)

	dispatcher := outbox.NewDispatcher(d.DB, d.Bus, l, cfg.OutboxPollInterval)
	wg.Add(1)
	go func() {
		defer wg.Done()
		dispatcher.Run(ctx)
	}()

	// The saga engine orchestrates the four README business sagas from the
	// events flowing through the bus, with state in Redis.
	if d.Saga != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Saga.Run(ctx)
		}()
	}

	// Keep the outbox deliveries gauge fresh for Prometheus.
	wg.Add(1)
	go func() {
		defer wg.Done()
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

	// drained is closed only after the HTTP server has stopped, the workers
	// have been awaited (bounded), and the bus is closed — so Run cannot return
	// (and the process exit) before graceful shutdown actually completes.
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		<-ctx.Done()
		slog.Info("shutting down...")

		// Stop accepting new requests first. Separate budget from the worker
		// drain so a slow HTTP shutdown cannot starve in-flight batches.
		httpCtx, httpCancel := context.WithTimeout(context.Background(), httpDrainTimeout)
		_ = srv.Shutdown(httpCtx)
		httpCancel()

		// Give the workers a bounded window to finish their current batch.
		workersDone := make(chan struct{})
		go func() {
			wg.Wait()
			close(workersDone)
		}()
		select {
		case <-workersDone:
			slog.Info("background workers stopped")
		case <-time.After(shutdownTimeout):
			slog.Warn("timed out waiting for background workers", "timeout", shutdownTimeout.String())
		}

		_ = d.Bus.Close()
	}()

	slog.Info("compliance-hub server listening", "addr", cfg.ServerAddr)
	srv.Spin()

	// Block until the drain goroutine above finishes. Defensive timeout: the
	// only path out of Spin is ctx cancellation (which starts the drain), so
	// this normally completes immediately after Spin returns.
	select {
	case <-drained:
	case <-time.After(shutdownTimeout + httpDrainTimeout):
		slog.Warn("drain did not complete within the shutdown window")
	}
}
