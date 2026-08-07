// Package reporting implements ReportingService: body-only CRUD for reports
// plus a generate endpoint that assembles a summary for a compliance entity.
package reporting

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/auditlog"
	"github.com/aeroxe/compliance-hub/backend/internal/bus"
	"github.com/aeroxe/compliance-hub/backend/internal/crud"
	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/events"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/outbox"
	"github.com/aeroxe/compliance-hub/backend/internal/repository"
	"github.com/aeroxe/compliance-hub/backend/internal/respond"
)

// RegisterRoutes mounts ReportingService endpoints under /api/v1/reports.
func RegisterRoutes(g *route.RouterGroup, d deps.Deps) {
	repo := repository.New[models.Report](d.DB, d.Cache, "report")
	svc := &Service{repo: repo, bus: d.Bus, db: d.DB}

	resource := &crud.Resource[models.Report]{
		Repo:   repo,
		Bus:    d.Bus,
		DB:     d.DB,
		Slug:   "report",
		Audit:  true,
		Events: map[string]string{"created": events.SubjectReportGenerated},
		SearchColumns: map[string]string{
			"title":         "title",
			"type":          "type",
			"status":        "status",
			"compliance_id": "compliance_id",
			"generated_at":  "generated_at",
			"created_at":    "created_at",
			"updated_at":    "updated_at",
		},
		SummaryBy: "type",
	}
	h := resource.Handlers()

	rg := g.Group("/reports")
	rg.POST("", h.Create)
	rg.POST("/search", h.Search)
	rg.POST("/get", h.GetByID)
	rg.PATCH("", h.Update)
	rg.DELETE("", h.Delete)
	rg.POST("/generate", svc.generate)
}

// GenerateRequest is the body of POST /reports/generate.
type GenerateRequest struct {
	Title        string    `json:"title" vd:"$ != ''"`
	Type         string    `json:"type"`
	Description  string    `json:"description"`
	ComplianceID uuid.UUID `json:"compliance_id"`
}

// Service contains ReportingService business logic.
type Service struct {
	repo *repository.Repository[models.Report]
	bus  bus.Bus
	db   *gorm.DB
}

func (s *Service) generate(ctx context.Context, c *app.RequestContext) {
	var req GenerateRequest
	if err := c.BindAndValidate(&req); err != nil {
		respond.BadRequest(c, "invalid_body", err)
		return
	}
	if req.Type == "" {
		req.Type = "summary"
	}

	// Assemble the summary payload using structured GORM queries.
	summary := map[string]any{
		"compliance_id": req.ComplianceID,
		"generated_at":  repository.Now(),
	}
	if req.ComplianceID != uuid.Nil {
		var audits, violations, alerts, checklists int64
		s.db.Model(&models.Audit{}).Where("compliance_id = ?", req.ComplianceID).Count(&audits)
		s.db.Model(&models.Violation{}).Where("compliance_id = ?", req.ComplianceID).Count(&violations)
		s.db.Model(&models.Alert{}).Where("entity_type = ? AND entity_id = ?", "compliance", req.ComplianceID).Count(&alerts)
		s.db.Model(&models.Checklist{}).Where("compliance_id = ?", req.ComplianceID).Count(&checklists)
		summary["audits"] = audits
		summary["violations"] = violations
		summary["alerts"] = alerts
		summary["checklists"] = checklists
	}

	now := repository.Now()
	report := models.Report{
		Title:        req.Title,
		Type:         req.Type,
		Status:       "generated",
		Description:  req.Description,
		ComplianceID: req.ComplianceID,
		GeneratedAt:  &now,
	}
	if b, err := json.Marshal(summary); err == nil {
		report.Data = datatypes.JSON(b)
	}

	if err := s.repo.Create(ctx, &report); err != nil {
		respond.Internal(c, err)
		return
	}
	_ = outbox.Enqueue(ctx, s.db, s.bus, events.SubjectReportGenerated, "report_generated", report)

	e := auditlog.Entry{
		Action: "generate", EntityType: "report", EntityID: report.ID,
		After: report,
	}
	e.ActorID, e.IP, e.UserAgent = auditlog.FromRequest(c)
	if err := auditlog.Record(ctx, s.db, e); err != nil {
		slog.Warn("audit log write failed", "resource", "report", "action", "generate", "error", err)
	}

	respond.Created(c, report)
}
