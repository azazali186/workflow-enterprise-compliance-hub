// Package analytics implements AnalyticsService read-only endpoints (all
// POST, body-only per the API contract). Aggregations use structured GORM
// queries (Model/Where/Count) — no raw SQL.
package analytics

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/respond"
)

// RegisterRoutes mounts AnalyticsService endpoints under /api/v1/analytics.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	h := &Handler{db: d.DB}
	rg := g.Group("/analytics")
	rg.POST("/summary", h.summary)
	rg.POST("/compliances", h.compliances)
	rg.POST("/audits", h.audits)
	rg.POST("/violations", h.violations)
	rg.POST("/deadlines", h.deadlines)
}

// Handler serves analytics endpoints.
type Handler struct {
	db *gorm.DB
}

type countPair struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

func (h *Handler) summary(ctx context.Context, c *app.RequestContext) {
	entities := []struct {
		key string
		m   any
	}{
		{"regulations", &models.Regulation{}},
		{"compliances", &models.Compliance{}},
		{"audits", &models.Audit{}},
		{"checklists", &models.Checklist{}},
		{"alerts", &models.Alert{}},
		{"reports", &models.Report{}},
		{"violations", &models.Violation{}},
		{"corrective_actions", &models.CorrectiveAction{}},
		{"deadlines", &models.Deadline{}},
		{"audit_logs", &models.AuditLog{}},
	}
	out := make([]countPair, 0, len(entities))
	for _, e := range entities {
		var n int64
		if err := h.db.Model(e.m).Count(&n).Error; err != nil {
			respond.Internal(c, err)
			return
		}
		out = append(out, countPair{Key: e.key, Count: n})
	}

	var openViolations, openAlerts, overdueDeadlines int64
	h.db.Model(&models.Violation{}).Where("status IN ?", []string{models.ViolationStatusOpen, models.ViolationStatusInReview}).Count(&openViolations)
	h.db.Model(&models.Alert{}).Where("status = ?", models.AlertStatusOpen).Count(&openAlerts)
	h.db.Model(&models.Deadline{}).Where("status = ?", models.DeadlineStatusOverdue).Count(&overdueDeadlines)

	respond.OK(c, map[string]any{
		"totals":            out,
		"open_violations":   openViolations,
		"open_alerts":       openAlerts,
		"overdue_deadlines": overdueDeadlines,
	})
}

func (h *Handler) compliances(ctx context.Context, c *app.RequestContext) {
	statuses := []string{
		models.ComplianceStatusDraft,
		models.ComplianceStatusActive,
		models.ComplianceStatusCompliant,
		models.ComplianceStatusNonCompliant,
		models.ComplianceStatusArchived,
	}
	h.countBy(c, &models.Compliance{}, "status", statuses)
}

func (h *Handler) audits(ctx context.Context, c *app.RequestContext) {
	statuses := []string{
		models.AuditStatusScheduled,
		models.AuditStatusInProgress,
		models.AuditStatusCompleted,
		models.AuditStatusCancelled,
	}
	h.countBy(c, &models.Audit{}, "status", statuses)
}

func (h *Handler) violations(ctx context.Context, c *app.RequestContext) {
	severities := []string{models.SeverityLow, models.SeverityMedium, models.SeverityHigh, models.SeverityCritical}
	h.countBy(c, &models.Violation{}, "severity", severities)
}

func (h *Handler) deadlines(ctx context.Context, c *app.RequestContext) {
	statuses := []string{
		models.DeadlineStatusUpcoming,
		models.DeadlineStatusDue,
		models.DeadlineStatusOverdue,
		models.DeadlineStatusCompleted,
	}
	h.countBy(c, &models.Deadline{}, "status", statuses)
}

func (h *Handler) countBy(c *app.RequestContext, m any, column string, values []string) {
	out := make([]countPair, 0, len(values))
	for _, v := range values {
		var n int64
		if err := h.db.Model(m).Where(column+" = ?", v).Count(&n).Error; err != nil {
			respond.Internal(c, err)
			return
		}
		out = append(out, countPair{Key: v, Count: n})
	}
	respond.OK(c, out)
}
