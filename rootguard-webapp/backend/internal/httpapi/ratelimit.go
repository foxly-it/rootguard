package httpapi

import (
	"sync"
	"time"
)

// rateLimiter is a simple sliding-window counter, keyed by an arbitrary
// string. The login/recovery limiters only ever call recordFailure for
// actual failures, so a legitimate operator typing their own password
// correctly is never affected; the destructive-action limiter (see
// destructive.go) instead calls recordFailure for every attempt regardless
// of outcome, since the thing being bounded there is request volume itself,
// not repeated wrong guesses.
type rateLimiter struct {
	mu         sync.Mutex
	window     time.Duration
	maxFailure int
	failures   map[string][]time.Time
}

func newRateLimiter(window time.Duration, maxFailure int) *rateLimiter {
	return &rateLimiter{
		window:     window,
		maxFailure: maxFailure,
		failures:   make(map[string][]time.Time),
	}
}

// blocked reports whether key has hit the failure limit within the current
// window, and prunes any failures that have aged out while it's here.
func (rl *rateLimiter) blocked(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.pruneLocked(key, time.Now())
	return len(rl.failures[key]) >= rl.maxFailure
}

func (rl *rateLimiter) recordFailure(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	rl.pruneLocked(key, now)
	rl.failures[key] = append(rl.failures[key], now)
}

// reset clears a key's failure history - called on a successful
// authentication so a since-resolved typo doesn't count against a later,
// unrelated attempt once the window would otherwise have expired anyway.
func (rl *rateLimiter) reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.failures, key)
}

func (rl *rateLimiter) pruneLocked(key string, now time.Time) {
	cutoff := now.Add(-rl.window)
	kept := rl.failures[key][:0]
	for _, at := range rl.failures[key] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		delete(rl.failures, key)
		return
	}
	rl.failures[key] = kept
}
