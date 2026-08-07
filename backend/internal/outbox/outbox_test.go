package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/aeroxe/compliance-hub/backend/internal/bus"
	"github.com/aeroxe/compliance-hub/backend/internal/lock"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
	"github.com/aeroxe/compliance-hub/backend/internal/resilience"
)

// fakeBus implements bus.Bus with controllable failures and recorded deliveries.
type fakeBus struct {
	mu         sync.Mutex
	failFirst  int // fail this many distinct PublishEvent calls
	attempts   map[string]int
	delivered  []bus.Event
	failAlways string // event ID that always fails (poisoned)
}

func (f *fakeBus) Publish(ctx context.Context, subject, eventType string, payload any) error {
	return f.PublishEvent(ctx, bus.Event{ID: "x", Subject: subject, Type: eventType, Payload: payload})
}

func (f *fakeBus) PublishEvent(ctx context.Context, e bus.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts[e.ID]++
	if e.ID == f.failAlways {
		return errors.New("poisoned event")
	}
	if f.failFirst > 0 {
		f.failFirst--
		return errors.New("transient bus failure")
	}
	f.delivered = append(f.delivered, e)
	return nil
}

func (f *fakeBus) Subscribe(subject string, handler bus.Handler) (func(), error) {
	return func() {}, nil
}

func (f *fakeBus) Recent(limit int) []bus.Event { return nil }

func (f *fakeBus) Close() error { return nil }

// pinSingleConnection keeps the shared :memory: sqlite database on one pooled
// connection — without it, each pooled connection gets its own empty DB and
// writes become invisible to later reads.
func pinSingleConnection(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
}

func newOutboxEnv(t *testing.T) (*gorm.DB, *fakeBus, *Dispatcher) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	pinSingleConnection(t, db)
	if err := db.AutoMigrate(&models.OutboxEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	fb := &fakeBus{attempts: map[string]int{}}
	// Note: real apps pass lock.New(redis); here the memory lock stands in.
	d := NewDispatcher(db, fb, lock.New(context.Background(), ""), time.Hour)
	return db, fb, d
}

func countPending(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&models.OutboxEvent{}).Where("published_at IS NULL").Count(&n).Error; err != nil {
		t.Fatalf("count pending: %v", err)
	}
	return n
}

func TestDispatchRetriesTransientFailures(t *testing.T) {
	db, fb, d := newOutboxEnv(t)
	ctx := context.Background()

	ids := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		id, err := Insert(ctx, db, "test.subject", "test.event", map[string]any{"n": i})
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
		ids = append(ids, id)
	}

	fb.failFirst = 2 // first two PublishEvent calls fail once
	if err := d.dispatchBatch(ctx); err != nil {
		t.Fatalf("dispatchBatch: %v", err)
	}

	if pending := countPending(t, db); pending != 0 {
		t.Fatalf("pending after dispatch = %d, want 0", pending)
	}
	fb.mu.Lock()
	defer fb.mu.Unlock()
	if len(fb.delivered) != 3 {
		t.Fatalf("delivered = %d, want 3", len(fb.delivered))
	}
	// The delivered IDs must match the queued stable IDs.
	got := map[string]bool{}
	for _, e := range fb.delivered {
		got[e.ID] = true
	}
	for _, id := range ids {
		if !got[id] {
			t.Errorf("event %s not delivered with its stable id", id)
		}
	}
}

func TestPoisonedEventDeadLettersWithoutBlocking(t *testing.T) {
	db, fb, d := newOutboxEnv(t)
	ctx := context.Background()

	poisonID, err := Insert(ctx, db, "poison.subject", "poison.event", map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("insert poison: %v", err)
	}
	fb.failAlways = poisonID

	healthyID, err := Insert(ctx, db, "healthy.subject", "healthy.event", map[string]any{"x": 2})
	if err != nil {
		t.Fatalf("insert healthy: %v", err)
	}

	// Lower the cap and make per-event retries instant so the attempt cap is
	// reached quickly: the poisoned row exhausts its attempts and is
	// dead-lettered while the healthy row still delivers.
	d.maxAttempts = 3
	origRetry := deliveryRetry
	deliveryRetry = resilience.RetryOpts{MaxAttempts: 1, InitialWait: time.Millisecond, MaxWait: time.Millisecond}
	defer func() { deliveryRetry = origRetry }()

	// Drain until the queue is empty (the poison needs 3 dispatch cycles).
	for i := 0; i < 10; i++ {
		if err := d.dispatchBatch(ctx); err != nil {
			t.Fatalf("dispatchBatch: %v", err)
		}
		if countPending(t, db) == 0 {
			break
		}
	}

	// The healthy event is gone from the queue; the poisoned one is retired.
	if pending := countPending(t, db); pending != 0 {
		t.Fatalf("pending after dispatch = %d, want 0 (poisoned dead-lettered, healthy delivered)", pending)
	}

	var poison models.OutboxEvent
	if err := db.Where("id = ?", poisonID).First(&poison).Error; err != nil {
		t.Fatalf("find poison row: %v", err)
	}
	if poison.PublishedAt == nil {
		t.Fatal("poisoned event should be retired (published_at set)")
	}
	if poison.Attempts != 3 {
		t.Errorf("poisoned attempts = %d, want 3", poison.Attempts)
	}

	var healthy models.OutboxEvent
	if err := db.Where("id = ?", healthyID).First(&healthy).Error; err != nil {
		t.Fatalf("find healthy row: %v", err)
	}
	if healthy.PublishedAt == nil {
		t.Fatal("healthy event should be published")
	}

	fb.mu.Lock()
	defer fb.mu.Unlock()
	found := false
	for _, e := range fb.delivered {
		if e.ID == healthyID {
			found = true
		}
	}
	if !found {
		t.Fatal("healthy event never delivered — poisoned head must not block the batch")
	}
}

func TestEnqueuePublishesImmediately(t *testing.T) {
	db, fb, _ := newOutboxEnv(t)
	ctx := context.Background()

	if err := Enqueue(ctx, db, fb, "fast.subject", "fast.event", map[string]any{"ok": true}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// Queued row exists…
	if pending := countPending(t, db); pending != 1 {
		t.Fatalf("queued rows = %d, want 1", pending)
	}
	// …and the fast path already published it.
	fb.mu.Lock()
	defer fb.mu.Unlock()
	if len(fb.delivered) != 1 {
		t.Fatalf("fast-path deliveries = %d, want 1", len(fb.delivered))
	}
}
