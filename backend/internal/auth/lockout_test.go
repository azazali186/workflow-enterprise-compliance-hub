package auth

import (
	"context"
	"testing"

	"github.com/aeroxe/compliance-hub/backend/internal/cache"
)

func TestLockoutLifecycle(t *testing.T) {
	ctx := context.Background()
	c := cache.New(ctx, "")

	if Locked(ctx, c, "alice") {
		t.Fatal("fresh account must not be locked")
	}

	// Failures below the threshold do not lock.
	for i := 0; i < LockoutMaxFailures-1; i++ {
		if RecordLockoutFailure(ctx, c, "alice") {
			t.Fatalf("locked at failure %d, threshold %d", i+1, LockoutMaxFailures)
		}
		if Locked(ctx, c, "alice") {
			t.Fatalf("account locked before threshold at failure %d", i+1)
		}
	}

	// The threshold failure locks the account.
	if !RecordLockoutFailure(ctx, c, "alice") {
		t.Fatal("threshold failure must report locked")
	}
	if !Locked(ctx, c, "alice") {
		t.Fatal("account must be locked after threshold")
	}

	// A successful login clears the counter.
	ClearLockout(ctx, c, "alice")
	if Locked(ctx, c, "alice") {
		t.Fatal("account must be unlocked after ClearLockout")
	}

	// Accounts are independent.
	if Locked(ctx, c, "bob") {
		t.Fatal("unrelated account must not be locked")
	}
}
