// Package outbox implements the transactional outbox pattern: domain events
// are queued in the outbox_events table (transactionally with the business
// write where possible) and a dispatcher worker publishes them to the bus,
// retrying with exponential backoff and re-delivering until acknowledged.
// A distributed lock ensures only one replica dispatches at a time.
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/bus"
	"github.com/aeroxe/compliance-hub/backend/internal/crypto"
	"github.com/aeroxe/compliance-hub/backend/internal/lock"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/resilience"
)

// Enqueue records an event in the outbox and makes a best-effort immediate
// publish so latency stays low; the dispatcher guarantees delivery.
//
// Semantics are at-least-once: a crash between the business write and this
// insert loses the event (the strict outbox guarantee requires InsertTx inside
// the business transaction). Once queued, delivery is retried until the
// dispatcher publishes or dead-letters the row.
func Enqueue(ctx context.Context, db *gorm.DB, b bus.Bus, subject, eventType string, payload any) error {
	id, err := Insert(ctx, db, subject, eventType, payload)
	if err != nil {
		return err
	}
	// Best-effort fast path (dispatcher covers failures). Duplicate deliveries
	// share a stable event ID so consumers can deduplicate.
	_ = b.PublishEvent(ctx, bus.Event{ID: id, Subject: subject, Type: eventType, Payload: payload, Timestamp: time.Now().UTC()})
	return nil
}

// Insert writes a queued event (with a stable UUID) and returns its ID.
func Insert(ctx context.Context, db *gorm.DB, subject, eventType string, payload any) (string, error) {
	row, err := newRow(subject, eventType, payload)
	if err != nil {
		return "", err
	}
	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		return "", err
	}
	return row.ID.String(), nil
}

// InsertTx writes the queued event inside an existing transaction so business
// writes and their events commit atomically.
func InsertTx(ctx context.Context, tx *gorm.DB, subject, eventType string, payload any) (string, error) {
	row, err := newRow(subject, eventType, payload)
	if err != nil {
		return "", err
	}
	if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
		return "", err
	}
	return row.ID.String(), nil
}

func newRow(subject, eventType string, payload any) (models.OutboxEvent, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return models.OutboxEvent{}, err
	}
	// Encrypt the payload before it touches the database: outbox rows can
	// contain full entity snapshots (sensitive business data) and live for
	// minutes to hours until the dispatcher drains them. The dispatcher
	// decrypts on delivery, so bus consumers are unaffected.
	enc, err := crypto.EncryptString(string(b))
	if err != nil {
		return models.OutboxEvent{}, err
	}
	// The payload column is jsonb: the ciphertext must be stored as a valid
	// JSON string (quoted), not a bare string, or Postgres rejects the INSERT.
	payloadJSON, err := json.Marshal(enc)
	if err != nil {
		return models.OutboxEvent{}, err
	}
	id, err := uuid.NewV7() // time-ordered: keeps the primary key index hot
	if err != nil {
		return models.OutboxEvent{}, err
	}
	return models.OutboxEvent{
		ID:        id,
		Subject:   subject,
		EventType: eventType,
		Payload:   datatypes.JSON(payloadJSON),
		CreatedAt: time.Now().UTC(),
	}, nil
}

// maxAttemptsDefault is the delivery retry cap; rows beyond it are dead-lettered
// so a poisoned event cannot stall the queue behind it.
const maxAttemptsDefault = 25

// deliveryRetry is the per-event retry policy. It is deliberately short: the
// immediate publish fast path already covers the happy case, so the dispatcher
// only retries briefly to keep batches small and lock hold times bounded.
var deliveryRetry = resilience.RetryOpts{
	MaxAttempts: 4,
	InitialWait: 100 * time.Millisecond,
	MaxWait:     1 * time.Second,
}

// Dispatcher polls the outbox and publishes pending events.
type Dispatcher struct {
	db          *gorm.DB
	bus         bus.Bus
	lock        lock.Lock
	batchSize   int
	pollEvery   time.Duration
	maxAttempts int
	breaker     *resilience.CircuitBreaker
	deliveries  int64
}

// NewDispatcher creates a dispatcher. pollEvery <= 0 defaults to 1s.
func NewDispatcher(db *gorm.DB, b bus.Bus, l lock.Lock, pollEvery time.Duration) *Dispatcher {
	if pollEvery <= 0 {
		pollEvery = time.Second
	}
	d := &Dispatcher{
		db:          db,
		bus:         b,
		lock:        l,
		batchSize:   100,
		pollEvery:   pollEvery,
		maxAttempts: maxAttemptsDefault,
		breaker:     resilience.NewCircuitBreaker(5, 30*time.Second),
	}
	return d
}

// Deliveries returns the total number of events dispatched (metrics).
func (d *Dispatcher) Deliveries() int64 { return d.deliveries }

// Run blocks, dispatching outbox events until the context is cancelled.
func (d *Dispatcher) Run(ctx context.Context) {
	slog.Info("outbox dispatcher started", "poll_every", d.pollEvery.String())
	ticker := time.NewTicker(d.pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.dispatchOnce(ctx)
		}
	}
}

// dispatchOnce dispatches one batch under the distributed lock. The lock TTL
// is the crash-detection window; batches stay short thanks to the bounded
// per-event retry, so normal runs release it well before expiry.
func (d *Dispatcher) dispatchOnce(ctx context.Context) {
	err := lock.WithLock(ctx, d.lock, "outbox:dispatcher", 30*time.Second, func() error {
		if !d.breaker.Allow() {
			return nil // open: skip this cycle, avoid hammering the bus
		}
		return d.dispatchBatch(ctx)
	})
	if err != nil && !errors.Is(err, lock.ErrLocked) {
		slog.Error("outbox dispatch failed", "error", err)
	}
}

func (d *Dispatcher) dispatchBatch(ctx context.Context) error {
	var pending []models.OutboxEvent
	// Structured GORM query — no raw SQL.
	if err := d.db.WithContext(ctx).
		Where("published_at IS NULL").
		Order("created_at ASC").
		Limit(d.batchSize).
		Find(&pending).Error; err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	for i := range pending {
		row := &pending[i]
		if err := d.deliver(ctx, row); err != nil {
			d.breaker.Failure()
			d.recordFailure(ctx, row, err)
			// A permanently-failing event must not stall the queue behind it:
			// dead-letter it after the attempt cap and keep going. The breaker
			// still limits overall cycles when the bus is down.
			continue
		}
		d.breaker.Success()
		d.markPublished(ctx, row)
		d.deliveries++
	}
	return nil
}

func (d *Dispatcher) deliver(ctx context.Context, row *models.OutboxEvent) error {
	// Payloads are encrypted at rest and stored as a JSON-quoted string.
	// Unquote it first; legacy plaintext rows (raw JSON objects) fall back to
	// the raw bytes and pass through DecryptString unchanged via the enc:v1
	// prefix check.
	var enc string
	if err := json.Unmarshal(row.Payload, &enc); err != nil {
		enc = string(row.Payload)
	}
	plain, err := crypto.DecryptString(enc)
	if err != nil {
		return fmt.Errorf("decrypt outbox payload: %w", err)
	}
	payload := json.RawMessage(plain)
	e := bus.Event{
		ID:        row.ID.String(),
		Subject:   row.Subject,
		Type:      row.EventType,
		Payload:   payload,
		Timestamp: row.CreatedAt,
	}
	return resilience.Do(ctx, func() error { return d.bus.PublishEvent(ctx, e) }, deliveryRetry)
}

func (d *Dispatcher) markPublished(ctx context.Context, row *models.OutboxEvent) {
	_ = d.db.WithContext(ctx).Model(row).Update("published_at", time.Now().UTC())
}

func (d *Dispatcher) recordFailure(ctx context.Context, row *models.OutboxEvent, err error) {
	row.Attempts++
	_ = d.db.WithContext(ctx).Model(row).
		Updates(map[string]any{"attempts": row.Attempts, "last_error": err.Error()})
	if row.Attempts >= d.maxAttempts {
		d.deadLetter(ctx, row, err)
	}
}

// deadLetter permanently retires an event that exhausted its attempts so the
// queue keeps flowing; the error is preserved for inspection.
func (d *Dispatcher) deadLetter(ctx context.Context, row *models.OutboxEvent, err error) {
	_ = d.db.WithContext(ctx).Model(row).Updates(map[string]any{
		"published_at": time.Now().UTC(),
		"last_error":   fmt.Sprintf("dead_lettered after %d attempts: %v", row.Attempts, err),
	})
	slog.Error("outbox event dead-lettered", "event_id", row.ID, "type", row.EventType, "attempts", row.Attempts, "error", err)
}
