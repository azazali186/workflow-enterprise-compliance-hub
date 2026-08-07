package cache

import (
	"context"
	"strings"
	"sync"
	"time"
)

type item struct {
	value   string
	expires time.Time
}

type memoryCache struct {
	mu    sync.RWMutex
	items map[string]item
}

func newMemory() *memoryCache {
	return &memoryCache{items: make(map[string]item)}
}

func (m *memoryCache) Get(_ context.Context, key string) (string, bool) {
	m.mu.RLock()
	it, ok := m.items[key]
	m.mu.RUnlock()
	if !ok {
		return "", false
	}
	// A zero expiry means the item never expires (e.g. route permissions).
	if !it.expires.IsZero() && time.Now().After(it.expires) {
		m.Del(context.Background(), key)
		return "", false
	}
	return it.value, true
}

func (m *memoryCache) Set(_ context.Context, key, value string, ttl time.Duration) error {
	m.mu.Lock()
	expires := time.Now().Add(ttl)
	if ttl <= 0 {
		expires = time.Time{} // no expiry, matching Redis semantics for ttl<=0
	}
	m.items[key] = item{value: value, expires: expires}
	m.mu.Unlock()
	return nil
}

func (m *memoryCache) SetNX(_ context.Context, key, value string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if it, ok := m.items[key]; ok && (it.expires.IsZero() || time.Now().Before(it.expires)) {
		return false, nil // already present and not expired
	}
	expires := time.Now().Add(ttl)
	if ttl <= 0 {
		expires = time.Time{}
	}
	m.items[key] = item{value: value, expires: expires}
	return true, nil
}

func (m *memoryCache) Del(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.items, key)
	m.mu.Unlock()
	return nil
}

func (m *memoryCache) TTL(_ context.Context, key string) (time.Duration, bool) {
	m.mu.RLock()
	it, ok := m.items[key]
	m.mu.RUnlock()
	if !ok {
		return 0, false
	}
	if it.expires.IsZero() {
		return 0, true // no expiry set
	}
	d := time.Until(it.expires)
	if d <= 0 {
		return 0, false
	}
	return d, true
}

// Keys returns keys matching a glob pattern ("*" wildcards). The in-memory
// backend supports the leading/trailing wildcard shapes the application uses.
func (m *memoryCache) Keys(_ context.Context, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []string
	for k := range m.items {
		if matchPattern(pattern, k) {
			out = append(out, k)
		}
	}
	return out, nil
}

func matchPattern(pattern, key string) bool {
	if pattern == "*" {
		return true
	}
	switch {
	case strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*"):
		return strings.Contains(key, strings.TrimSuffix(strings.TrimPrefix(pattern, "*"), "*"))
	case strings.HasPrefix(pattern, "*"):
		return strings.HasSuffix(key, strings.TrimPrefix(pattern, "*"))
	case strings.HasSuffix(pattern, "*"):
		return strings.HasPrefix(key, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == key
}
