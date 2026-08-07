package saga

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/bus"
	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/events"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

func newSagaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1) // :memory: sqlite is per-connection
	if err := db.AutoMigrate(models.All()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newTestEngine(t *testing.T, db *gorm.DB) (*Engine, cache.Cache, bus.Bus) {
	t.Helper()
	ctx := context.Background()
	c := cache.New(ctx, "")
	b := bus.New(ctx, "")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(db, c, b, logger), c, b
}

func ev(subject, eventType string, payload any) bus.Event {
	return bus.Event{ID: uuid.NewString(), Subject: subject, Type: eventType, Payload: payload, Timestamp: time.Now().UTC()}
}

func countRows(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var n int64
	if err := db.Model(model).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestComplianceCheckAutoEvaluatesAndCompletes(t *testing.T) {
	db := newSagaTestDB(t)
	ctx := context.Background()
	e, _, _ := newTestEngine(t, db)

	past := time.Now().UTC().Add(-24 * time.Hour)
	compliance := models.Compliance{
		Name: "GDPR", Status: models.ComplianceStatusActive, RiskLevel: "high",
		DueDate: &past, // due date passed, never reviewed
	}
	if err := db.Create(&compliance).Error; err != nil {
		t.Fatalf("create compliance: %v", err)
	}

	e.HandleEvent(ctx, ev(events.SubjectComplianceCreated, "compliance.created", compliance))

	// The saga flipped the status to non_compliant and queued the alert.
	var updated models.Compliance
	if err := db.First(&updated, "id = ?", compliance.ID).Error; err != nil {
		t.Fatalf("load compliance: %v", err)
	}
	if updated.Status != models.ComplianceStatusNonCompliant {
		t.Errorf("status = %q, want non_compliant (auto-evaluated by the saga)", updated.Status)
	}
	if n := countRows(t, db, &models.OutboxEvent{}); n != 1 {
		t.Errorf("outbox rows = %d, want 1 (compliance.alert queued)", n)
	}

	// Saga is active, one step in, waiting for the alert event.
	s, err := e.Get(ctx, TypeComplianceCheck, compliance.ID)
	if err != nil || s == nil {
		t.Fatalf("Get active saga = %v/%v, want non-nil", s, err)
	}
	if s.Status != StatusActive || s.CurrentStep != 1 {
		t.Errorf("saga = status %q step %d, want active step 1", s.Status, s.CurrentStep)
	}

	// The alert event completes the flow and clears Redis state.
	e.HandleEvent(ctx, ev(events.SubjectComplianceAlert, events.WSComplianceAlert, updated))
	s, _ = e.Get(ctx, TypeComplianceCheck, compliance.ID)
	if s != nil {
		t.Errorf("completed saga still present in Redis: %+v (state must live only until completion)", s)
	}
	started, completed, failed := e.Stats()
	if started != 1 || completed != 1 || failed != 0 {
		t.Errorf("stats = started %d completed %d failed %d, want 1/1/0", started, completed, failed)
	}
}

func TestComplianceCheckSkipsDraftAndDoesNotAlertOnNoopUpdate(t *testing.T) {
	db := newSagaTestDB(t)
	ctx := context.Background()
	e, _, _ := newTestEngine(t, db)

	compliant := models.Compliance{Name: "SOX", Status: models.ComplianceStatusCompliant}
	if err := db.Create(&compliant).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	e.HandleEvent(ctx, ev(events.SubjectComplianceUpdated, "compliance.updated", compliant))

	// Compliant within due date: no status change, no alert queued.
	var got models.Compliance
	_ = db.First(&got, "id = ?", compliant.ID).Error
	if got.Status != models.ComplianceStatusCompliant {
		t.Errorf("status = %q, want compliant unchanged", got.Status)
	}
	if n := countRows(t, db, &models.OutboxEvent{}); n != 0 {
		t.Errorf("outbox rows = %d, want 0 (no change on update must not alert)", n)
	}

	draft := models.Compliance{Name: "HIPAA", Status: models.ComplianceStatusDraft, DueDate: &[]time.Time{time.Now().Add(-time.Hour)}[0]}
	if err := db.Create(&draft).Error; err != nil {
		t.Fatalf("create draft: %v", err)
	}
	e.HandleEvent(ctx, ev(events.SubjectComplianceCreated, "compliance.created", draft))
	if n := countRows(t, db, &models.OutboxEvent{}); n != 0 {
		t.Errorf("outbox rows = %d, want 0 (draft is not monitored)", n)
	}
}

func TestComplianceCheckRedeliveryIsIdempotent(t *testing.T) {
	db := newSagaTestDB(t)
	ctx := context.Background()
	e, _, _ := newTestEngine(t, db)

	past := time.Now().UTC().Add(-time.Hour)
	c := models.Compliance{Name: "PCI", Status: models.ComplianceStatusActive, DueDate: &past}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	e.HandleEvent(ctx, ev(events.SubjectComplianceCreated, "compliance.created", c))
	e.HandleEvent(ctx, ev(events.SubjectComplianceCreated, "compliance.created", c)) // at-least-once re-delivery

	if n := countRows(t, db, &models.OutboxEvent{}); n != 1 {
		t.Errorf("outbox rows = %d, want 1 (re-delivery must not enqueue duplicates)", n)
	}
}

func TestViolationProcessingAutoCreatesAndTracksCorrectiveAction(t *testing.T) {
	db := newSagaTestDB(t)
	ctx := context.Background()
	e, _, _ := newTestEngine(t, db)

	v := models.Violation{Title: "Missing SOC2 control", Severity: "high", Status: models.ViolationStatusOpen}
	if err := db.Create(&v).Error; err != nil {
		t.Fatalf("create violation: %v", err)
	}
	e.HandleEvent(ctx, ev(events.SubjectViolationDetected, events.WSViolationDetected, v))
	e.HandleEvent(ctx, ev(events.SubjectViolationDetected, events.WSViolationDetected, v)) // re-delivery

	var cas []models.CorrectiveAction
	if err := db.Where("violation_id = ?", v.ID).Find(&cas).Error; err != nil {
		t.Fatalf("find corrective actions: %v", err)
	}
	if len(cas) != 1 {
		t.Fatalf("corrective actions = %d, want 1 (auto-created, deduplicated)", len(cas))
	}
	if cas[0].Status != "open" {
		t.Errorf("corrective action status = %q, want open", cas[0].Status)
	}

	// Resolving the violation closes the open corrective action.
	resolved := v
	resolved.Status = models.ViolationStatusResolved
	e.HandleEvent(ctx, ev(events.SubjectViolationResolved, "violation.resolved", resolved))

	if err := db.First(&cas[0], "id = ?", cas[0].ID).Error; err != nil {
		t.Fatalf("reload CA: %v", err)
	}
	if cas[0].Status != "completed" {
		t.Errorf("corrective action status after resolve = %q, want completed", cas[0].Status)
	}
	// Two events by now: correctiveaction.created (auto-create kickoff) and
	// correctiveaction.completed (closed by the resolve step).
	if n := countRows(t, db, &models.OutboxEvent{}); n != 2 {
		t.Errorf("outbox rows = %d, want 2 (created + completed)", n)
	}
	s, _ := e.Get(ctx, TypeViolationProcessing, v.ID)
	if s != nil {
		t.Errorf("completed violation saga still in Redis")
	}
	// The auto-created corrective action runs its own CorrectiveActionFlow
	// saga, which must have completed after the resolve closed the action.
	flow, _ := e.Get(ctx, TypeCorrectiveActionFlow, cas[0].ID)
	if flow != nil {
		t.Errorf("completed corrective action flow saga still in Redis")
	}
}

func TestAuditExecutionAdvancesThroughLifecycle(t *testing.T) {
	db := newSagaTestDB(t)
	ctx := context.Background()
	e, _, _ := newTestEngine(t, db)

	scheduled := models.Audit{Title: "Annual ISO audit", Status: models.AuditStatusScheduled}
	if err := db.Create(&scheduled).Error; err != nil {
		t.Fatalf("create audit: %v", err)
	}
	e.HandleEvent(ctx, ev(events.SubjectAuditScheduled, events.WSAuditScheduled, scheduled))

	s, _ := e.Get(ctx, TypeAuditExecution, scheduled.ID)
	if s == nil || s.CurrentStep != 1 {
		t.Fatalf("after schedule: saga = %+v, want step 1", s)
	}

	// The handler persists the transition before emitting the event; the saga
	// reads authoritative DB state, so persist like the audit module does.
	now := time.Now().UTC()
	if err := db.Model(&models.Audit{}).Where("id = ?", scheduled.ID).Updates(map[string]any{
		"status": models.AuditStatusInProgress, "started_at": now,
	}).Error; err != nil {
		t.Fatalf("persist start: %v", err)
	}
	e.HandleEvent(ctx, ev(events.SubjectAuditStarted, events.WSAuditScheduled, scheduled))
	s, _ = e.Get(ctx, TypeAuditExecution, scheduled.ID)
	if s == nil || s.CurrentStep != 2 {
		t.Fatalf("after start: saga = %+v, want step 2", s)
	}
	if s.Payload["started_at"] == "" {
		t.Error("start step did not record started_at")
	}

	if err := db.Model(&models.Audit{}).Where("id = ?", scheduled.ID).Updates(map[string]any{
		"status": models.AuditStatusCompleted, "completed_at": now,
	}).Error; err != nil {
		t.Fatalf("persist complete: %v", err)
	}
	e.HandleEvent(ctx, ev(events.SubjectAuditCompleted, events.WSAuditScheduled, scheduled))
	s, _ = e.Get(ctx, TypeAuditExecution, scheduled.ID)
	if s != nil {
		t.Errorf("completed audit saga still in Redis")
	}
}

func TestCompensationRunsOnStepFailure(t *testing.T) {
	db := newSagaTestDB(t)
	ctx := context.Background()
	c := cache.New(ctx, "")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := &Engine{
		db: db, cache: c, bus: bus.New(ctx, ""), logger: logger,
		now:  func() time.Time { return time.Now().UTC() },
		seen: make(map[string]time.Time),
	}

	var compensated bool
	entityID := uuid.New()
	e.defs = []Definition{{
		Type:     "test_saga",
		Subjects: []string{"test.step"},
		Steps: []Step{
			{Name: "one", Trigger: func(bus.Event) bool { return true },
				Action: func(ctx context.Context, s *Saga, ev bus.Event) error {
					s.Payload["one"] = "done"
					return nil
				},
				Compensate: func(ctx context.Context, s *Saga, ev bus.Event) error {
					compensated = true
					return nil
				}},
			{Name: "two", Trigger: func(bus.Event) bool { return true },
				Action: func(ctx context.Context, s *Saga, ev bus.Event) error {
					return errors.New("boom")
				}},
		},
	}}

	e.HandleEvent(ctx, ev("test.step", "test", map[string]any{"id": entityID.String()}))

	if !compensated {
		t.Error("compensation did not run for the completed step")
	}
	s, _ := e.Get(ctx, "test_saga", entityID)
	if s != nil {
		t.Errorf("failed saga still in Redis: %+v", s)
	}
	started, completed, failed := e.Stats()
	if started != 1 || completed != 0 || failed != 1 {
		t.Errorf("stats = started %d completed %d failed %d, want 1/0/1", started, completed, failed)
	}
}

func TestDuplicateEventIdIsDropped(t *testing.T) {
	db := newSagaTestDB(t)
	ctx := context.Background()
	e, _, _ := newTestEngine(t, db)

	c := models.Compliance{Name: "GLBA", Status: models.ComplianceStatusActive,
		DueDate: &[]time.Time{time.Now().UTC().Add(-time.Hour)}[0]}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	// The outbox publishes every event twice with the SAME stable ID
	// (immediate + dispatcher). The engine must drop the duplicate entirely.
	delivery := ev(events.SubjectComplianceCreated, "compliance.created", c)
	e.HandleEvent(ctx, delivery)
	e.HandleEvent(ctx, delivery) // duplicate ID

	if n := countRows(t, db, &models.OutboxEvent{}); n != 1 {
		t.Errorf("outbox rows = %d, want 1 (duplicate delivery must not re-run the step)", n)
	}
	started, _, _ := e.Stats()
	if started != 1 {
		t.Errorf("started = %d, want 1 (duplicate must not spawn a second saga)", started)
	}
}

func TestFailingStepItselfIsCompensated(t *testing.T) {
	db := newSagaTestDB(t)
	ctx := context.Background()
	c := cache.New(ctx, "")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := &Engine{
		db: db, cache: c, bus: bus.New(ctx, ""), logger: logger,
		now:  func() time.Time { return time.Now().UTC() },
		seen: make(map[string]time.Time),
	}

	// Step 0 mutates state (payload) and then fails; its own compensator must
	// roll the mutation back — the engine compensates up to AND including the
	// failing step.
	var compensated bool
	entityID := uuid.New()
	e.defs = []Definition{{
		Type:     "test_saga",
		Subjects: []string{"test.step"},
		Steps: []Step{
			{Name: "one", Trigger: func(bus.Event) bool { return true },
				Action: func(ctx context.Context, s *Saga, ev bus.Event) error {
					s.Payload["mutated"] = true
					return errors.New("boom")
				},
				Compensate: func(ctx context.Context, s *Saga, ev bus.Event) error {
					compensated = true
					delete(s.Payload, "mutated")
					return nil
				}},
		},
	}}

	e.HandleEvent(ctx, ev("test.step", "test", map[string]any{"id": entityID.String()}))

	if !compensated {
		t.Error("failing step's own compensator did not run")
	}
	_, _, failed := e.Stats()
	if failed != 1 {
		t.Errorf("failed = %d, want 1", failed)
	}
}

func TestTimeoutReapsStuckSaga(t *testing.T) {
	db := newSagaTestDB(t)
	ctx := context.Background()
	e, _, _ := newTestEngine(t, db)
	e.timeoutAfter = 0 // any active saga is immediately stale

	// A scheduled audit whose start event never arrives: the saga sits at
	// step 1 with no further event — the sweeper must reap it.
	scheduled := models.Audit{Title: "Ghost audit", Status: models.AuditStatusScheduled}
	if err := db.Create(&scheduled).Error; err != nil {
		t.Fatalf("create audit: %v", err)
	}
	e.HandleEvent(ctx, ev(events.SubjectAuditScheduled, events.WSAuditScheduled, scheduled))

	s, _ := e.Get(ctx, TypeAuditExecution, scheduled.ID)
	if s == nil || s.Status != StatusActive {
		t.Fatalf("expected an active saga, got %+v", s)
	}

	e.sweepOnce(ctx)

	s, _ = e.Get(ctx, TypeAuditExecution, scheduled.ID)
	if s != nil {
		t.Errorf("stuck saga still in Redis after sweep")
	}
	_, _, failed := e.Stats()
	if failed != 1 {
		t.Errorf("failed = %d, want 1 (stuck saga reaped)", failed)
	}
	// The sweep records a saga.timeout audit entry and surfaces in search.
	var auditCount int64
	if err := db.Model(&models.AuditLog{}).Where("action = ?", "saga.timeout").Count(&auditCount).Error; err != nil {
		t.Fatalf("count timeout audits: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("saga.timeout audit entries = %d, want 1", auditCount)
	}
}

func TestSearchReturnsRecentSagas(t *testing.T) {
	db := newSagaTestDB(t)
	ctx := context.Background()
	e, _, _ := newTestEngine(t, db)

	c := models.Compliance{Name: "NIST", Status: models.ComplianceStatusCompliant}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	e.HandleEvent(ctx, ev(events.SubjectComplianceCreated, "compliance.created", c))

	all := e.Search("", "", 10)
	if len(all) != 1 {
		t.Fatalf("search all = %d, want 1", len(all))
	}
	byType := e.Search(TypeComplianceCheck, "", 10)
	if len(byType) != 1 {
		t.Errorf("search by type = %d, want 1", len(byType))
	}
	byStatus := e.Search("", string(StatusActive), 10)
	if len(byStatus) != 1 {
		t.Errorf("search active = %d, want 1", len(byStatus))
	}
	missing := e.Search(TypeAuditExecution, "", 10)
	if len(missing) != 0 {
		t.Errorf("search wrong type = %d, want 0", len(missing))
	}
}
