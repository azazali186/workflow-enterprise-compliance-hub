// Package bus abstracts async event delivery. The primary backend is NATS
// (JetStream-capable server) matching the README; when NATS is unreachable the
// bus transparently falls back to an in-process pub/sub so local development
// and tests work without infrastructure.
package bus

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Event is the canonical message envelope published on the bus and pushed
// to WebSocket clients.
type Event struct {
	ID        string    `json:"id"`
	Subject   string    `json:"subject"`
	Type      string    `json:"type"`
	Payload   any       `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
}

// Handler receives a published event. Implementations must not block.
type Handler func(Event)

// Bus is the event bus contract.
type Bus interface {
	Publish(ctx context.Context, subject, eventType string, payload any) error
	// PublishEvent publishes a fully-formed event, preserving its ID (used by
	// the outbox dispatcher so re-deliveries keep a stable identity).
	PublishEvent(ctx context.Context, e Event) error
	Subscribe(subject string, handler Handler) (unsubscribe func(), err error)
	Recent(limit int) []Event
	Close() error
}

// impl is the transport-specific part of a bus.
type impl interface {
	Publish(subject string, e Event) error
	Subscribe(subject string, handler func(data []byte)) (func(), error)
	Close() error
}

type bus struct {
	impl impl
	mu   sync.Mutex
	ring []Event // recent events, newest last
	max  int
}

const recentLimit = 100

// New creates a bus. It never returns nil: NATS is preferred, in-memory is
// the fallback when the connection fails.
func New(ctx context.Context, natsURL string) Bus {
	if natsURL != "" {
		if n, err := newNATS(natsURL); err == nil {
			slog.Info("event bus backend: nats", "url", natsURL)
			return newBus(n)
		} else {
			slog.Warn("nats unavailable, falling back to in-process bus", "error", err)
		}
	}
	slog.Info("event bus backend: in-memory")
	return newBus(newMemory())
}

func newBus(b impl) *bus {
	return &bus{impl: b, max: recentLimit}
}

func (b *bus) Publish(_ context.Context, subject, eventType string, payload any) error {
	e := Event{
		ID:        uuid.NewString(),
		Subject:   subject,
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}
	return b.PublishEvent(context.Background(), e)
}

func (b *bus) PublishEvent(_ context.Context, e Event) error {
	b.mu.Lock()
	b.ring = append(b.ring, e)
	if len(b.ring) > b.max {
		b.ring = b.ring[len(b.ring)-b.max:]
	}
	b.mu.Unlock()

	return b.impl.Publish(e.Subject, e)
}

func (b *bus) Subscribe(subject string, handler Handler) (func(), error) {
	return b.impl.Subscribe(subject, func(data []byte) {
		handler(decode(data))
	})
}

// decode parses a raw subscription payload back into an Event. Because every
// backend round-trips through JSON, payloads arrive as json.RawMessage and are
// decoded to their concrete type here.
func decode(data []byte) Event {
	var e Event
	if err := json.Unmarshal(data, &e); err != nil {
		return Event{
			ID:        uuid.NewString(),
			Subject:   "unknown",
			Type:      "unknown",
			Payload:   string(data),
			Timestamp: time.Now().UTC(),
		}
	}
	return e
}

func (b *bus) Recent(limit int) []Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	if limit <= 0 || limit > len(b.ring) {
		limit = len(b.ring)
	}
	out := make([]Event, limit)
	copy(out, b.ring[len(b.ring)-limit:])
	return out
}

func (b *bus) Close() error { return b.impl.Close() }
