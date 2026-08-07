// Package notification implements NotificationService: it converts outbound
// notifications into outbox-queued bus events (pushed to WebSocket clients)
// and records an audit-log entry. The README schema has no notifications
// table, so nothing is persisted here beyond the audit trail.
package notification

import (
	"context"
	"log/slog"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/auditlog"
	"github.com/aeroxe/compliance-hub/backend/internal/bus"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/events"
	"github.com/aeroxe/compliance-hub/backend/internal/outbox"
	"github.com/aeroxe/compliance-hub/backend/internal/respond"
)

// RegisterRoutes mounts NotificationService endpoints under /api/v1/notifications.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	h := &Handler{bus: d.Bus, db: d.DB}
	rg := g.Group("/notifications")
	rg.POST("/send", h.send)
	rg.POST("/events", h.recent)
}

// SendRequest is the body of POST /notifications/send.
type SendRequest struct {
	Type       string    `json:"type" vd:"$ != ''"`
	Title      string    `json:"title" vd:"$ != ''"`
	Message    string    `json:"message"`
	Severity   string    `json:"severity"`
	EntityType string    `json:"entity_type"`
	EntityID   uuid.UUID `json:"entity_id"`
}

// EventsRequest is the body of POST /notifications/events.
type EventsRequest struct {
	Limit int `json:"limit"`
}

// Handler serves notification endpoints.
type Handler struct {
	bus bus.Bus
	db  *gorm.DB
}

func (h *Handler) send(ctx context.Context, c *app.RequestContext) {
	var req SendRequest
	if err := c.BindAndValidate(&req); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}

	payload := map[string]any{
		"type":        req.Type,
		"title":       req.Title,
		"message":     req.Message,
		"severity":    req.Severity,
		"entity_type": req.EntityType,
		"entity_id":   req.EntityID,
		"sent_at":     time.Now().UTC(),
	}

	eventType := req.Type
	if eventType == "" {
		eventType = events.WSComplianceAlert
	}
	if err := outbox.Enqueue(ctx, h.db, h.bus, events.SubjectNotificationSent, eventType, payload); err != nil {
		respond.Internal(c, err)
		return
	}

	// Write an audit-log entry so notifications leave a trace.
	e := auditlog.Entry{
		Action:     "notify",
		EntityType: req.EntityType,
		EntityID:   req.EntityID,
		After:      payload,
	}
	e.ActorID, e.IP, e.UserAgent = auditlog.FromRequest(c)
	if err := auditlog.Record(ctx, h.db, e); err != nil {
		// The notification was already queued; audit failure must not turn a
		// successful send into an error response.
		slog.Warn("audit log write failed", "resource", "notification", "action", "notify", "error", err)
	}

	respond.Created(c, map[string]any{"sent": true, "event_type": eventType, "subject": events.SubjectNotificationSent})
}

func (h *Handler) recent(ctx context.Context, c *app.RequestContext) {
	limit := 50
	var req EventsRequest
	if err := c.BindAndValidate(&req); err == nil && req.Limit > 0 {
		if req.Limit > 200 {
			req.Limit = 200
		}
		limit = req.Limit
	}
	respond.OK(c, h.bus.Recent(limit))
}
