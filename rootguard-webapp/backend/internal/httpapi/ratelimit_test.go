package httpapi

import (
	"testing"
	"time"
)

// recordFailureForTest seeds a window-tracked failure the same way
// production code does: beginAttempt+endAttempt(key, true), not a
// dedicated recordFailure() method - found in review, that method was an
// orphan only these two tests still called, unreachable from any real
// production code path since the TOCTOU fix moved every caller onto
// beginAttempt/endAttempt.
func recordFailureForTest(t *testing.T, rl *rateLimiter, key string) {
	t.Helper()
	if !rl.beginAttempt(key) {
		t.Fatalf("beginAttempt(%q): unexpectedly already at the limit", key)
	}
	rl.endAttempt(key, true)
}

func TestRateLimiterSweepRemovesFullyAgedOutKeys(t *testing.T) {
	rl := newRateLimiter(time.Millisecond, 5)
	recordFailureForTest(t, rl, "attacker-1")
	recordFailureForTest(t, rl, "attacker-2")

	time.Sleep(5 * time.Millisecond)
	rl.sweep()

	rl.mu.Lock()
	defer rl.mu.Unlock()
	if len(rl.failures) != 0 {
		t.Fatalf("expected sweep to remove every key whose failures aged out of the window, got %d keys left", len(rl.failures))
	}
}

func TestRateLimiterSweepKeepsKeysStillWithinWindow(t *testing.T) {
	rl := newRateLimiter(time.Hour, 5)
	recordFailureForTest(t, rl, "still-active")

	rl.sweep()

	rl.mu.Lock()
	defer rl.mu.Unlock()
	if len(rl.failures) != 1 {
		t.Fatalf("expected sweep to keep a key with failures still inside the window, got %d keys left", len(rl.failures))
	}
}
