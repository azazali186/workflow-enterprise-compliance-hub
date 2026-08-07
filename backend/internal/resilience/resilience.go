// Package resilience provides production-grade fault tolerance primitives:
// exponential backoff with jitter for retries, and a sliding circuit breaker
// for degrading gracefully when a dependency keeps failing.
package resilience

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"
)

// RetryOpts configures retry behaviour.
type RetryOpts struct {
	MaxAttempts int           // total attempts (1 = no retry)
	InitialWait time.Duration // base backoff
	MaxWait     time.Duration // backoff ceiling
	Jitter      float64       // 0..1 randomization factor
}

// DefaultRetry is a sensible production default (5 attempts, 100ms..2s, 20% jitter).
var DefaultRetry = RetryOpts{
	MaxAttempts: 5,
	InitialWait: 100 * time.Millisecond,
	MaxWait:     2 * time.Second,
	Jitter:      0.2,
}

// Do retries fn with exponential backoff and jitter until it succeeds, the
// context is cancelled, or MaxAttempts is exhausted.
func Do(ctx context.Context, fn func() error, opts RetryOpts) error {
	if opts.MaxAttempts < 1 {
		opts.MaxAttempts = 1
	}
	if opts.InitialWait <= 0 {
		opts.InitialWait = 100 * time.Millisecond
	}
	if opts.MaxWait <= 0 {
		opts.MaxWait = 2 * time.Second
	}
	var lastErr error
	wait := opts.InitialWait
	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt == opts.MaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jittered(wait, opts.Jitter)):
		}
		wait *= 2
		if wait > opts.MaxWait {
			wait = opts.MaxWait
		}
	}
	return lastErr
}

func jittered(wait time.Duration, factor float64) time.Duration {
	if factor <= 0 {
		return wait
	}
	delta := time.Duration(float64(wait) * factor)
	return wait - delta + time.Duration(rand.Int63n(int64(2*delta+1)))
}

// CircuitState of the breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // normal operation
	CircuitOpen                         // failing fast
	CircuitHalfOpen                     // probing after cooldown
)

// CircuitBreaker protects a dependency: after FailureThreshold consecutive
// failures it opens for Cooldown, then half-opens to probe with a single
// trial call before closing again.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitState
	failures         int
	openedAt         time.Time
	FailureThreshold int
	Cooldown         time.Duration
	OnStateChange    func(from, to CircuitState)
}

// NewCircuitBreaker creates a breaker with the given threshold and cooldown.
func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	if threshold < 1 {
		threshold = 3
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &CircuitBreaker{FailureThreshold: threshold, Cooldown: cooldown}
}

// ErrOpen is returned while the circuit is open.
var ErrOpen = errors.New("circuit open")

// Allow reports whether a call may proceed.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case CircuitClosed, CircuitHalfOpen:
		return true
	case CircuitOpen:
		if time.Since(cb.openedAt) >= cb.Cooldown {
			cb.transition(CircuitOpen, CircuitHalfOpen)
			return true
		}
		return false
	}
	return false
}

// Success records a successful call.
func (cb *CircuitBreaker) Success() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == CircuitHalfOpen {
		cb.transition(CircuitHalfOpen, CircuitClosed)
	}
	cb.failures = 0
}

// Failure records a failed call.
func (cb *CircuitBreaker) Failure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	switch cb.state {
	case CircuitHalfOpen:
		cb.transition(CircuitHalfOpen, CircuitOpen)
	case CircuitClosed:
		if cb.failures >= cb.FailureThreshold {
			cb.transition(CircuitClosed, CircuitOpen)
		}
	}
}

func (cb *CircuitBreaker) transition(from, to CircuitState) {
	cb.state = to
	if to == CircuitOpen {
		cb.openedAt = time.Now()
	}
	if cb.OnStateChange != nil {
		cb.OnStateChange(from, to)
	}
}

// State returns the current state (for metrics/observability).
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Execute runs fn guarded by the breaker with an optional retry policy.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error, opts *RetryOpts) error {
	if !cb.Allow() {
		return ErrOpen
	}
	run := func() error { return fn() }
	if opts != nil {
		run = func() error { return Do(ctx, fn, *opts) }
	}
	if err := run(); err != nil {
		cb.Failure()
		return err
	}
	cb.Success()
	return nil
}
