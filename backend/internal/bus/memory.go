package bus

import (
	"encoding/json"
	"strings"
	"sync"
)

// memoryBus is an in-process pub/sub used when NATS is not available.
// Subject patterns support a single "*" token and the NATS-style ">" wildcard.
type memoryBus struct {
	mu   sync.RWMutex
	subs map[string]map[int]func(Event)
	next int
}

func newMemory() *memoryBus {
	return &memoryBus{subs: make(map[string]map[int]func(Event))}
}

func (m *memoryBus) Publish(subject string, e Event) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for pattern, handlers := range m.subs {
		if match(pattern, subject) {
			for _, h := range handlers {
				go h(e)
			}
		}
	}
	return nil
}

func (m *memoryBus) Subscribe(subject string, handler func(data []byte)) (func(), error) {
	h := func(e Event) { handler(m.encode(e)) }

	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.next
	m.next++
	if m.subs[subject] == nil {
		m.subs[subject] = make(map[int]func(Event))
	}
	m.subs[subject][id] = h

	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.subs[subject], id)
	}, nil
}

func (m *memoryBus) Close() error { return nil }

// encode round-trips an Event through JSON so all backends deliver identical
// byte payloads to subscribers.
func (m *memoryBus) encode(e Event) []byte {
	b, _ := json.Marshal(e)
	return b
}

// match implements NATS-style subject matching: "*" matches exactly one token,
// ">" matches one or more trailing tokens.
func match(pattern, subject string) bool {
	if pattern == ">" || pattern == subject {
		return true
	}
	p := strings.Split(pattern, ".")
	s := strings.Split(subject, ".")
	for i, pt := range p {
		if pt == ">" {
			return len(s) >= i+1
		}
		if i >= len(s) || (pt != "*" && pt != s[i]) {
			return false
		}
	}
	return len(p) == len(s)
}
