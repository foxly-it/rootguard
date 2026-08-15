package httpapi

import (
	"testing"
	"time"
)

func TestRateLimiterSweepRemovesFullyAgedOutKeys(t *testing.T) {
	rl := newRateLimiter(time.Millisecond, 5)
	rl.recordFailure("attacker-1")
	rl.recordFailure("attacker-2")

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
	rl.recordFailure("still-active")

	rl.sweep()

	rl.mu.Lock()
	defer rl.mu.Unlock()
	if len(rl.failures) != 1 {
		t.Fatalf("expected sweep to keep a key with failures still inside the window, got %d keys left", len(rl.failures))
	}
}
