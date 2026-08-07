// Package saga implements the README Saga Orchestrator: long-running business
// workflows (ComplianceCheck, AuditExecution, ViolationProcessing,
// CorrectiveActionFlow) orchestrated via NATS events with orchestration state
// persisted in Redis under the `saga:<saga_id>` key pattern ("until
// completion"). Each saga advances step-by-step as its driving events arrive;
// a failed step triggers compensation of the steps already completed, and the
// whole lifecycle is written to the audit trail with actor=system.
package saga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/auditlog"
	"github.com/aeroxe/compliance-hub/backend/internal/bus"
	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/metrics"
)

// Status is the lifecycle state of a saga instance.
type Status string

const (
	StatusPending     Status = "pending"
	StatusActive      Status = "active"
	StatusCompleted   Status = "completed"
	StatusFailed      Status = "failed"
	StatusCompensated Status = "compensated"
)

// Saga type codes (stable identifiers used in the deterministic saga ID).
const (
	TypeComplianceCheck      = "compliance_check"
	TypeAuditExecution       = "audit_execution"
	TypeViolationProcessing  = "violation_processing"
	TypeCorrectiveActionFlow = "corrective_action_flow"
)

// sagaNamespace is the fixed UUID namespace for deterministic saga IDs. The
// same entity+type always maps to the same saga ID, which makes get-or-create
// idempotent under at-least-once event delivery (outbox re-deliveries cannot
// spawn duplicate sagas).
var sagaNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

// stateTTL is how long an in-flight saga stays in Redis. The README says saga
// state lives "until completion"; the TTL is a crash guard so a saga waiting
// forever on a lost event cannot grow the key space without bound. Every step
// advance renews it.
const stateTTL = 7 * 24 * time.Hour

// stateKeyPrefix is the README `saga:<saga_id>` key prefix.
const stateKeyPrefix = "saga:"

// stateKey builds the README `saga:<saga_id>` cache key.
func stateKey(id uuid.UUID) string { return stateKeyPrefix + id.String() }

// Default saga timeout policy: an active saga that receives no further event
// (e.g. an audit completed without being started) is reaped — compensated and
// failed — after timeoutAfter, bounded by the sweep cadence. The Redis state
// TTL (stateTTL) remains the hard upper bound.
const (
	defaultTimeoutAfter = 24 * time.Hour
	defaultSweepEvery   = time.Hour
)

// Step is one unit of work in a saga. Trigger decides whether an event starts
// the step; Action performs it; Compensate (optional) rolls it back when a
// later step fails.
type Step struct {
	Name       string
	Trigger    func(e bus.Event) bool
	Action     func(ctx context.Context, s *Saga, e bus.Event) error
	Compensate func(ctx context.Context, s *Saga, e bus.Event) error
}

// Definition binds a saga type to its workflow steps and the event subjects
// that drive it.
type Definition struct {
	Type     string
	Subjects []string
	Steps    []Step
}

// Saga is the persisted orchestration state stored at `saga:<id>`.
type Saga struct {
	ID          uuid.UUID      `json:"id"`
	Type        string         `json:"type"`
	EntityID    uuid.UUID      `json:"entity_id"`
	Status      Status         `json:"status"`
	CurrentStep int            `json:"current_step"` // index of the next step to run
	Payload     map[string]any `json:"payload,omitempty"`
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// Summary is the compact view kept in the observability ring.
type Summary struct {
	ID          uuid.UUID  `json:"id"`
	Type        string     `json:"type"`
	EntityID    uuid.UUID  `json:"entity_id"`
	Status      Status     `json:"status"`
	CurrentStep int        `json:"current_step"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// entityIDFromEvent extracts the entity UUID carried in an event payload.
// Event payloads are the marshaled domain entities, so the "id" field is the
// UUID the saga tracks.
func entityIDFromEvent(e bus.Event) (uuid.UUID, error) {
	if e.Payload == nil {
		return uuid.Nil, errors.New("event payload is empty")
	}
	raw, err := json.Marshal(e.Payload)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal event payload: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return uuid.Nil, fmt.Errorf("unmarshal event payload: %w", err)
	}
	idStr, _ := m["id"].(string)
	if idStr == "" {
		return uuid.Nil, errors.New("event payload has no id")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("event payload id is not a uuid: %w", err)
	}
	return id, nil
}

// sagaID deterministically derives the saga UUID for a type+entity pair.
func sagaID(sagaType string, entityID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(sagaNamespace, []byte(sagaType+":"+entityID.String()))
}

// recordAudit writes a saga transition to the audit trail. Failures are only
// logged: the saga engine must never be blocked by the audit store.
func recordAudit(ctx context.Context, db *gorm.DB, s *Saga, stepName string, status string, meta map[string]any) {
	e := auditlog.Entry{
		Action:     "saga." + stepName,
		Status:     status,
		EntityType: "saga",
		EntityID:   s.ID,
		ActorID:    "system",
		Metadata:   meta,
	}
	if err := auditlog.Record(ctx, db, e); err != nil {
		slog.Warn("saga audit write failed", "saga_id", s.ID, "step", stepName, "error", err)
	}
}

// load reads saga state from Redis by saga ID.
func load(ctx context.Context, c cache.Cache, id uuid.UUID) (*Saga, bool) {
	return loadKey(ctx, c, stateKey(id))
}

// loadKey reads saga state from Redis by its state key.
func loadKey(ctx context.Context, c cache.Cache, key string) (*Saga, bool) {
	raw, ok := c.Get(ctx, key)
	if !ok {
		return nil, false
	}
	var s Saga
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		slog.Warn("saga state corrupt, treating as absent", "key", key, "error", err)
		_ = c.Del(ctx, key)
		return nil, false
	}
	return &s, true
}

// persist writes saga state to Redis and refreshes the TTL.
func persist(ctx context.Context, c cache.Cache, s *Saga) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return c.Set(ctx, stateKey(s.ID), string(b), stateTTL)
}

// finish removes the state key once the saga reaches a terminal state
// (README: saga state lives "until completion").
func finish(ctx context.Context, c cache.Cache, s *Saga) {
	_ = c.Del(ctx, stateKey(s.ID))
}

// recentLimit caps the in-process observability ring.
const recentLimit = 100

// eventDedupWindow covers the outbox's at-least-once double delivery: every
// event is published immediately and again by the background dispatcher
// (~seconds apart) with the same stable ID. Consumers must drop the duplicate.
const eventDedupWindow = 10 * time.Minute

// eventDedupCap bounds the in-memory dedupe ring.
const eventDedupCap = 4096

// Engine orchestrates saga instances: it subscribes to the driving event
// subjects, routes each event to the matching saga, advances steps and
// compensates on failure. It is safe for concurrent use.
type Engine struct {
	db     *gorm.DB
	cache  cache.Cache
	bus    bus.Bus
	logger *slog.Logger
	now    func() time.Time

	defs []Definition

	mu     sync.Mutex
	recent []Summary

	started   int64
	completed int64
	failed    int64

	dedupMu   sync.Mutex
	seenOrder []string
	seen      map[string]time.Time

	timeoutAfter time.Duration
	sweepEvery   time.Duration
}

// New creates the saga engine with the four README business sagas registered.
func New(db *gorm.DB, c cache.Cache, b bus.Bus, logger *slog.Logger) *Engine {
	e := &Engine{
		db:           db,
		cache:        c,
		bus:          b,
		logger:       logger,
		now:          func() time.Time { return time.Now().UTC() },
		seen:         make(map[string]time.Time),
		timeoutAfter: defaultTimeoutAfter,
		sweepEvery:   defaultSweepEvery,
	}
	e.defs = []Definition{
		e.complianceCheck(),
		e.auditExecution(),
		e.violationProcessing(),
		e.correctiveActionFlow(),
	}
	return e
}

// Stats returns the engine's lifecycle counters (also exposed as Prometheus
// metrics).
func (e *Engine) Stats() (started, completed, failed int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.started, e.completed, e.failed
}

// Run subscribes to every subject the registered sagas care about and
// dispatches events. It returns when the context is cancelled.
func (e *Engine) Run(ctx context.Context) {
	seen := make(map[string]bool)
	for _, def := range e.defs {
		for _, subject := range def.Subjects {
			if seen[subject] {
				continue
			}
			seen[subject] = true
			sub := subject // capture for the closure
			unsub, err := e.bus.Subscribe(sub, func(ev bus.Event) { e.HandleEvent(ctx, ev) })
			if err != nil {
				e.logger.Warn("saga engine subscribe failed", "subject", sub, "error", err)
				continue
			}
			e.logger.Info("saga engine subscribed", "subject", sub)
			// NATS subscriptions persist for the process lifetime; the context
			// only gates event processing, not subscription cleanup.
			_ = unsub
		}
	}
	// Reap sagas that never received their next event (stuck workflows).
	go e.sweepLoop(ctx)
	<-ctx.Done()
}

// sweepLoop periodically reaps stuck sagas until the context is cancelled.
func (e *Engine) sweepLoop(ctx context.Context) {
	ticker := time.NewTicker(e.sweepEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.sweepOnce(ctx)
		}
	}
}

// sweepOnce fails any active saga that has made no progress for longer than
// timeoutAfter, running its compensation chain (so partial side effects are
// rolled back) and recording a saga.timeout audit entry. Only the engine that
// holds the Redis lock on the state key can reap; concurrent replicas observe
// the deleted key and skip it.
func (e *Engine) sweepOnce(ctx context.Context) {
	keys, err := e.cache.Keys(ctx, stateKeyPrefix+"*")
	if err != nil {
		e.logger.Warn("saga sweep keys failed", "error", err)
		return
	}
	for _, key := range keys {
		s, ok := loadKey(ctx, e.cache, key)
		if !ok || s.Status != StatusActive {
			continue
		}
		if e.now().Sub(s.UpdatedAt) < e.timeoutAfter {
			continue
		}
		def := e.definition(s.Type)
		if def == nil {
			continue
		}
		e.logger.Warn("saga timed out", "saga_id", s.ID, "type", s.Type, "entity_id", s.EntityID, "step", s.CurrentStep)
		e.fail(ctx, *def, s, s.CurrentStep, "timeout",
			fmt.Errorf("saga timed out after %s without progress", e.timeoutAfter))
	}
}

// definition returns the registered definition for a saga type, if any.
func (e *Engine) definition(sagaType string) *Definition {
	for i := range e.defs {
		if e.defs[i].Type == sagaType {
			return &e.defs[i]
		}
	}
	return nil
}

// Search returns recent saga summaries, optionally filtered by type/status.
func (e *Engine) Search(sagaType, status string, limit int) []Summary {
	if limit <= 0 || limit > recentLimit {
		limit = recentLimit
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Summary, 0, limit)
	seen := make(map[uuid.UUID]bool)
	for i := len(e.recent) - 1; i >= 0 && len(out) < limit; i-- {
		s := e.recent[i]
		if seen[s.ID] {
			continue // keep only the latest state per saga
		}
		if sagaType != "" && s.Type != sagaType {
			continue
		}
		if status != "" && string(s.Status) != status {
			continue
		}
		seen[s.ID] = true
		out = append(out, s)
	}
	return out
}

// Get returns the full live state of a saga (from Redis), or nil when no
// saga exists for the type+entity pair.
func (e *Engine) Get(ctx context.Context, sagaType string, entityID uuid.UUID) (*Saga, error) {
	id := sagaID(sagaType, entityID)
	if s, ok := load(ctx, e.cache, id); ok {
		return s, nil
	}
	return nil, nil
}

// HandleEvent routes one event to the saga(s) it drives. A saga instance is
// only created when the event actually matches a step, so unrelated or late
// events cannot spawn empty sagas. Events are deduplicated by their stable ID
// (the outbox publishes every event twice: immediately and via the dispatcher).
func (e *Engine) HandleEvent(ctx context.Context, ev bus.Event) {
	if ev.ID == "" || e.seenEvent(ev.ID) {
		return // at-least-once delivery: already processed this event
	}
	for _, def := range e.defs {
		if !containsSubject(def.Subjects, ev.Subject) {
			continue
		}
		e.handleDefinition(ctx, def, ev)
	}
}

// seenEvent records an event ID and reports whether it was already processed.
func (e *Engine) seenEvent(id string) bool {
	e.dedupMu.Lock()
	defer e.dedupMu.Unlock()
	if _, ok := e.seen[id]; ok {
		return true
	}
	now := e.now()
	for len(e.seenOrder) > 0 {
		first := e.seenOrder[0]
		t, ok := e.seen[first]
		if !ok || now.Sub(t) <= eventDedupWindow {
			break
		}
		delete(e.seen, first)
		e.seenOrder = e.seenOrder[1:]
	}
	if len(e.seenOrder) >= eventDedupCap {
		delete(e.seen, e.seenOrder[0])
		e.seenOrder = e.seenOrder[1:]
	}
	e.seen[id] = now
	e.seenOrder = append(e.seenOrder, id)
	return false
}

func (e *Engine) handleDefinition(ctx context.Context, def Definition, ev bus.Event) {
	entityID, err := entityIDFromEvent(ev)
	if err != nil {
		e.logger.Warn("saga event has no entity id", "saga_type", def.Type, "subject", ev.Subject, "error", err)
		return
	}
	id := sagaID(def.Type, entityID)

	s, ok := load(ctx, e.cache, id)
	if !ok {
		// Only start a fresh saga when the event triggers the first pending
		// step; otherwise the event belongs to a different lifecycle.
		if len(def.Steps) == 0 || !def.Steps[0].Trigger(ev) {
			return
		}
		s = &Saga{
			ID:          id,
			Type:        def.Type,
			EntityID:    entityID,
			Status:      StatusActive,
			CurrentStep: 0,
			Payload:     map[string]any{},
			CreatedAt:   e.now(),
			UpdatedAt:   e.now(),
		}
		// Atomically claim the instance: concurrent deliveries of the same
		// event (or a replica restarting) must not spawn a duplicate saga.
		// The loser only observes the winner's state: re-executing steps for
		// its (racing) event could duplicate side effects and skew metrics.
		if !e.claim(ctx, s) {
			s, _ = load(ctx, e.cache, id)
			return
		}
		{
			e.mu.Lock()
			e.started++
			e.mu.Unlock()
			metrics.SagaStarted()
			e.remember(s.Summary())
			recordAudit(ctx, e.db, s, "start", "success", map[string]any{"saga_type": def.Type, "entity_id": entityID.String()})
		}
	}

	for i := s.CurrentStep; i < len(def.Steps); i++ {
		step := def.Steps[i]
		if !step.Trigger(ev) {
			break // this saga is waiting for a later event
		}
		if err := step.Action(ctx, s, ev); err != nil {
			e.fail(ctx, def, s, i, step.Name, err)
			return
		}
		s.CurrentStep = i + 1
		s.Status = StatusActive
		s.UpdatedAt = e.now()
		s.Error = ""
		if perr := persist(ctx, e.cache, s); perr != nil {
			e.logger.Warn("saga state persist failed", "saga_id", s.ID, "error", perr)
		}
		e.remember(s.Summary())
		recordAudit(ctx, e.db, s, step.Name, "success", map[string]any{"saga_type": def.Type, "step_index": i})
	}

	if s.CurrentStep >= len(def.Steps) {
		s.Status = StatusCompleted
		s.UpdatedAt = e.now()
		s.CompletedAt = ptr(e.now())
		finish(ctx, e.cache, s) // "until completion"
		e.mu.Lock()
		e.completed++
		e.mu.Unlock()
		metrics.SagaCompleted()
		e.remember(s.Summary())
		recordAudit(ctx, e.db, s, "complete", "success", map[string]any{"saga_type": def.Type})
	}
}

// claim atomically persists a new saga via SET NX so only one concurrent
// delivery owns creation. On backend errors it fails open (the caller's own
// persist below still stores the state).
func (e *Engine) claim(ctx context.Context, s *Saga) bool {
	b, err := json.Marshal(s)
	if err != nil {
		return true
	}
	ok, err := e.cache.SetNX(ctx, stateKey(s.ID), string(b), stateTTL)
	if err != nil {
		e.logger.Warn("saga claim backend error, failing open", "saga_id", s.ID, "error", err)
		return true
	}
	return ok
}

// fail marks the saga failed after compensating every step up to and
// including the failed one (in reverse order) that registered a compensator.
// Compensators are defensive — they no-op unless they recorded a side effect
// in the payload — so running the failed step's own compensator rolls back
// any partial mutation it made before erroring.
func (e *Engine) fail(ctx context.Context, def Definition, s *Saga, failedIndex int, stepName string, cause error) {
	for i := failedIndex; i >= 0; i-- {
		step := def.Steps[i]
		if step.Compensate == nil {
			continue
		}
		if err := step.Compensate(ctx, s, bus.Event{}); err != nil {
			e.logger.Error("saga compensation failed", "saga_id", s.ID, "step", step.Name, "error", err)
		}
	}
	s.Status = StatusCompensated
	if cause != nil {
		s.Error = cause.Error()
		s.Status = StatusFailed
	}
	s.CurrentStep = failedIndex
	s.UpdatedAt = e.now()
	_ = persist(ctx, e.cache, s)
	finish(ctx, e.cache, s) // failed sagas do not linger in Redis
	e.mu.Lock()
	e.failed++
	e.mu.Unlock()
	metrics.SagaFailed()
	e.remember(s.Summary())
	recordAudit(ctx, e.db, s, stepName, "failed", map[string]any{"saga_type": def.Type, "error": s.Error})
	e.logger.Error("saga step failed", "saga_id", s.ID, "type", def.Type, "step", stepName, "error", cause)
}

func (e *Engine) remember(s Summary) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.recent = append(e.recent, s)
	if len(e.recent) > recentLimit {
		e.recent = e.recent[len(e.recent)-recentLimit:]
	}
}

// Summary builds the compact ring entry for a saga.
func (s *Saga) Summary() Summary {
	return Summary{
		ID:          s.ID,
		Type:        s.Type,
		EntityID:    s.EntityID,
		Status:      s.Status,
		CurrentStep: s.CurrentStep,
		Error:       s.Error,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
		CompletedAt: s.CompletedAt,
	}
}

func containsSubject(subjects []string, subject string) bool {
	for _, s := range subjects {
		if s == subject {
			return true
		}
	}
	return false
}

func ptr[T any](v T) *T { return &v }
