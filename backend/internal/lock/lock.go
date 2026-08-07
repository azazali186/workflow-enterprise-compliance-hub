// Package lock provides distributed mutual exclusion used by background
// workers (outbox dispatcher, deadline evaluator) so multiple replicas do not
// process the same work. Backends: Redis (SET NX EX) and an in-process mutex
// fallback for single-instance deployments and tests.
package lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrLocked is returned when the lock is held elsewhere.
var ErrLocked = errors.New("lock held by another holder")

// Lock is the distributed lock contract.
type Lock interface {
	// TryAcquire attempts to take the lock; it never blocks.
	TryAcquire(ctx context.Context, key string, ttl time.Duration) (Token, error)
	// Release releases a lock previously acquired with the same token.
	Release(ctx context.Context, key string, token Token) error
}

// Token identifies a lock holder (prevents releasing someone else's lock).
type Token string

// WithLock runs fn only if the lock can be acquired; otherwise returns ErrLocked.
func WithLock(ctx context.Context, l Lock, key string, ttl time.Duration, fn func() error) error {
	tok, err := l.TryAcquire(ctx, key, ttl)
	if err != nil {
		return err
	}
	defer l.Release(context.Background(), key, tok)
	return fn()
}

// New builds a lock. It prefers Redis; falls back to the in-process mutex.
func New(ctx context.Context, redisURL string) Lock {
	if redisURL != "" {
		if opts, err := redis.ParseURL(redisURL); err == nil {
			if r := newRedisLock(redis.NewClient(opts)); r != nil {
				// Verify connectivity; fall back to memory otherwise.
				if r.client.Ping(ctx).Err() == nil {
					return r
				}
			}
		}
	}
	return newMemoryLock()
}

// --- Redis backend (distributed) ---

type redisLock struct {
	client *redis.Client
}

func newRedisLock(client *redis.Client) *redisLock { return &redisLock{client: client} }

func (r *redisLock) TryAcquire(ctx context.Context, key string, ttl time.Duration) (Token, error) {
	token := randomToken()
	ok, err := r.client.SetNX(ctx, "lock:"+key, token, ttl).Result()
	if err != nil {
		return "", fmt.Errorf("acquire lock %s: %w", key, err)
	}
	if !ok {
		return "", ErrLocked
	}
	return Token(token), nil
}

func (r *redisLock) Release(ctx context.Context, key string, token Token) error {
	// Compare-and-delete so we only release our own lock (Lua, no raw SQL).
	const script = "if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return 0 end"
	_, err := r.client.Eval(ctx, script, []string{"lock:" + key}, string(token)).Result()
	return err
}

// --- In-process backend (single instance / tests) ---

type memoryLock struct {
	mu     sync.Mutex
	held   map[string]Token
	expiry map[string]time.Time
}

func newMemoryLock() *memoryLock {
	return &memoryLock{held: make(map[string]Token), expiry: make(map[string]time.Time)}
}

func (m *memoryLock) TryAcquire(ctx context.Context, key string, ttl time.Duration) (Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.held[key]; ok {
		if time.Now().Before(m.expiry[key]) {
			return "", ErrLocked
		}
		// expired: reclaim
		delete(m.held, key)
		delete(m.expiry, key)
	}
	token := Token(randomToken())
	m.held[key] = token
	m.expiry[key] = time.Now().Add(ttl)
	return token, nil
}

func (m *memoryLock) Release(ctx context.Context, key string, token Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if held, ok := m.held[key]; ok && held == token {
		delete(m.held, key)
		delete(m.expiry, key)
	}
	return nil
}

func randomToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
