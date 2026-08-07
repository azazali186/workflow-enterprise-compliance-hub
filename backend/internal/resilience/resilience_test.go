package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreakerOpensAndRecovers(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)

	// Closed: calls allowed until threshold of consecutive failures.
	for i := 0; i < 2; i++ {
		if !cb.Allow() {
			t.Fatalf("allow %d: circuit should be closed", i)
		}
		cb.Failure()
	}
	if cb.State() != CircuitClosed {
		t.Fatal("circuit should still be closed below threshold")
	}
	cb.Failure() // 3rd failure
	if cb.State() != CircuitOpen {
		t.Fatal("circuit should be open after threshold")
	}
	if cb.Allow() {
		t.Fatal("open circuit must fail fast")
	}

	// After cooldown it half-opens and allows a single probe.
	time.Sleep(60 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("half-open circuit should allow a probe")
	}
	if cb.State() != CircuitHalfOpen {
		t.Fatal("state should be half-open after cooldown")
	}

	// Successful probe closes the circuit.
	cb.Success()
	if cb.State() != CircuitClosed {
		t.Fatal("successful probe should close the circuit")
	}
	if !cb.Allow() {
		t.Fatal("closed circuit should allow calls")
	}
}

func TestCircuitBreakerHalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(2, 20*time.Millisecond)
	cb.Failure()
	cb.Failure()
	if cb.State() != CircuitOpen {
		t.Fatal("circuit should be open")
	}
	time.Sleep(30 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("half-open probe expected")
	}
	cb.Failure() // probe fails -> back to open
	if cb.State() != CircuitOpen {
		t.Fatal("failed probe should reopen the circuit")
	}
	if cb.Allow() {
		t.Fatal("reopened circuit must fail fast")
	}
}

func TestExecuteWithRetrySucceeds(t *testing.T) {
	cb := NewCircuitBreaker(5, time.Minute)
	attempts := 0
	err := cb.Execute(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return errors.New("transient failure")
		}
		return nil
	}, &RetryOpts{MaxAttempts: 5, InitialWait: time.Millisecond, MaxWait: 5 * time.Millisecond})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
	if cb.State() != CircuitClosed {
		t.Fatal("circuit should be closed after success")
	}
}

func TestExecuteRetryExhausted(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute)
	fail := func() error { return errors.New("always fails") }
	opts := &RetryOpts{MaxAttempts: 3, InitialWait: time.Millisecond, MaxWait: time.Millisecond}

	// Execute records one failure per logical call (retries absorb transient
	// failures inside), so three failing calls trip the threshold.
	for i := 0; i < 3; i++ {
		if err := cb.Execute(context.Background(), fail, opts); err == nil {
			t.Fatal("expected error after retries exhausted")
		}
	}
	if cb.State() != CircuitOpen {
		t.Fatal("circuit should be open after sustained failures")
	}
}

func TestExecuteOpenCircuitFailsFast(t *testing.T) {
	cb := NewCircuitBreaker(1, time.Hour)
	cb.Failure()
	if err := cb.Execute(context.Background(), func() error { return nil }, nil); err != ErrOpen {
		t.Fatalf("Execute on open circuit = %v, want ErrOpen", err)
	}
}

func TestRetryRespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Do(ctx, func() error { return errors.New("nope") }, DefaultRetry)
	if err == nil {
		t.Fatal("expected error")
	}
}
