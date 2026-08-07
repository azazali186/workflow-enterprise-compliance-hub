// Command seed inserts a small set of sample records into every table so the
// API and dashboard have data to work with during development.
package main

import (
	"log/slog"
	"os"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/config"
	"github.com/aeroxe/compliance-hub/backend/internal/database"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	db, err := database.Connect(cfg.DatabaseURL, cfg.LogLevel)
	if err != nil {
		slog.Error("database", "error", err)
		os.Exit(1)
	}
	if err := database.Migrate(db); err != nil {
		slog.Error("migrate", "error", err)
		os.Exit(1)
	}

	seed(db)
	slog.Info("seed data inserted")
}

func seed(db *gorm.DB) {
	now := time.Now().UTC()
	inWeek := now.Add(7 * 24 * time.Hour)
	past := now.Add(-3 * 24 * time.Hour)

	reg := models.Regulation{
		Title: "GDPR - General Data Protection Regulation",
		Code:  "GDPR-2016/679", Jurisdiction: "EU",
		Status: "active", EffectiveDate: &past,
	}
	mustCreate(db, &reg)

	compliance := models.Compliance{
		Name:        "Annual Data Processing Review",
		Description: "Verify all data processing activities against GDPR obligations.",
		Status:      models.ComplianceStatusActive, RiskLevel: "high",
		RegulationID: reg.ID, DueDate: &inWeek, LastReviewedAt: &past,
	}
	mustCreate(db, &compliance)

	audit := models.Audit{
		Title:       "Q3 Internal Compliance Audit",
		Description: "Quarterly audit of processing records.",
		Status:      models.AuditStatusScheduled, ComplianceID: compliance.ID,
		AuditorID: "user-001", ScheduledAt: &inWeek,
	}
	mustCreate(db, &audit)

	checklist := models.Checklist{
		Title:       "Data Processing Register",
		Description: "Verify the register of processing activities is current.",
		Status:      "open", ComplianceID: compliance.ID, OwnerID: "user-002",
		DueDate: &inWeek,
		Items:   datatypes.JSON(`[{"text":"Review processing purposes","done":true},{"text":"Confirm retention periods","done":false}]`),
	}
	mustCreate(db, &checklist)

	alert := models.Alert{
		Type: "review_required", Title: "Compliance review overdue",
		Message:  "The annual review is due within 7 days.",
		Severity: "medium", Status: models.AlertStatusOpen,
		EntityType: "compliance", EntityID: compliance.ID,
	}
	mustCreate(db, &alert)

	violation := models.Violation{
		Title:       "Missing data retention policy",
		Description: "No documented retention period for customer records.",
		Severity:    "high", Status: models.ViolationStatusOpen,
		ComplianceID: compliance.ID, RegulationID: reg.ID,
		DetectedAt: &past,
	}
	mustCreate(db, &violation)

	action := models.CorrectiveAction{
		Title:       "Draft data retention policy",
		Description: "Create and publish the retention policy document.",
		Status:      "open", ViolationID: violation.ID,
		OwnerID: "user-003", DueDate: &inWeek,
	}
	mustCreate(db, &action)

	deadline := models.Deadline{
		Title:       "Annual compliance review",
		Description: "Complete the annual compliance review.",
		Status:      models.DeadlineStatusUpcoming,
		EntityType:  "compliance", EntityID: compliance.ID,
		DeadlineAt: inWeek,
	}
	mustCreate(db, &deadline)

	report := models.Report{
		Title: "Compliance summary report",
		Type:  "summary", Status: "generated",
		Description:  "Overview of the compliance program.",
		ComplianceID: compliance.ID, GeneratedAt: &now,
		Data: datatypes.JSON(`{"audits":1,"violations":1,"alerts":1}`),
	}
	mustCreate(db, &report)

	log := models.AuditLog{
		Action: "seed", EntityType: "system",
		ActorID: "seed-cli", IP: "127.0.0.1",
		Metadata: datatypes.JSON(`{"note":"initial sample data"}`),
	}
	mustCreate(db, &log)
}

// mustCreate persists a record and panics on failure — seed is a dev tool.
func mustCreate(db *gorm.DB, v any) {
	if err := db.Create(v).Error; err != nil {
		panic(err)
	}
}
