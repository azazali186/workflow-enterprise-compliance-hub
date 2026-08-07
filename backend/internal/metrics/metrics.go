// Package metrics exposes Prometheus instrumentation: HTTP request counters
// and latency histograms via middleware, plus a small set of application
// gauges (outbox deliveries, circuit states). The /metrics endpoint is mounted
// by the server.
package metrics

import (
	"context"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry is the application Prometheus registry (default collector included).
var Registry = prometheus.NewRegistry()

var (
	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "compliancehub",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total HTTP requests processed.",
	}, []string{"method", "path", "status"})

	httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "compliancehub",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request latency in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "path"})

	outboxDeliveries = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "compliancehub",
		Subsystem: "outbox",
		Name:      "deliveries_total",
		Help:      "Total outbox events delivered.",
	})

	sagaActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "compliancehub",
		Subsystem: "saga",
		Name:      "active",
		Help:      "Sagas currently in flight (started but not completed).",
	})

	sagaStarted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "compliancehub",
		Subsystem: "saga",
		Name:      "started_total",
		Help:      "Total saga instances started.",
	})

	sagaCompleted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "compliancehub",
		Subsystem: "saga",
		Name:      "completed_total",
		Help:      "Total sagas that ran every step to completion.",
	})

	sagaFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "compliancehub",
		Subsystem: "saga",
		Name:      "failed_total",
		Help:      "Total sagas that failed and were compensated.",
	})
)

func init() {
	Registry.MustRegister(prometheus.NewGoCollector())
	Registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	Registry.MustRegister(httpRequests, httpDuration, outboxDeliveries, sagaActive, sagaStarted, sagaCompleted, sagaFailed)
}

// Middleware records request counts and latency per route.
func Middleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		c.Next(ctx)
		path := string(c.Path())
		method := string(c.Method())
		httpRequests.WithLabelValues(method, path, strconv.Itoa(c.Response.StatusCode())).Inc()
		httpDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
	}
}

// SetOutboxDeliveries updates the outbox deliveries gauge.
func SetOutboxDeliveries(n int64) { outboxDeliveries.Set(float64(n)) }

// SagaStarted records a new saga instance.
func SagaStarted() { sagaStarted.Inc(); sagaActive.Inc() }

// SagaCompleted records a successfully finished saga.
func SagaCompleted() { sagaCompleted.Inc(); sagaActive.Dec() }

// SagaFailed records a failed (compensated) saga.
func SagaFailed() { sagaFailed.Inc(); sagaActive.Dec() }

// Handler returns the Prometheus scrape handler, bridged to hertz via the
// standard adaptor (same approach as the WebSocket upgrade).
func Handler() app.HandlerFunc {
	h := promhttp.HandlerFor(Registry, promhttp.HandlerOpts{})
	return func(ctx context.Context, c *app.RequestContext) {
		req, err := adaptor.GetCompatRequest(&c.Request)
		if err != nil {
			c.AbortWithStatus(400)
			return
		}
		w := adaptor.GetCompatResponseWriter(&c.Response)
		h.ServeHTTP(w, req)
	}
}
