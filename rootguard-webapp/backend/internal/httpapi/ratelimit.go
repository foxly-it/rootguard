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
	inFlight   map[string]int
}

func newRateLimiter(window time.Duration, maxFailure int) *rateLimiter {
	return &rateLimiter{
		window:     window,
		maxFailure: maxFailure,
		failures:   make(map[string][]time.Time),
		inFlight:   make(map[string]int),
	}
}

// beginAttempt atomically combines a failure-limit check with reserving a
// slot for one verification attempt - closing a timing gap a plain
// separate-check-then-recordFailure() pattern has: many concurrent
// requests could all observe zero recorded failures, all start their own
// (for login/account, PBKDF2-based and genuinely expensive) verification in
// parallel, and only get counted afterward once that work is already done,
// so the limit never actually bounds concurrent attempts, only sequential
// ones. Counting an in-flight attempt as if it already were a failure for
// admission purposes (without permanently recording it as one) closes that
// gap: a request arriving while others are still being verified sees them
// too and is turned away before doing any expensive work itself.
//
// Returns false (and reserves nothing) once recorded failures plus
// attempts already in flight reach the limit; the caller must not call
// endAttempt in that case. On true, the caller must call endAttempt exactly
// once, however the attempt turns out - a deferred call is the simplest way
// to guarantee that across every return path.
func (rl *rateLimiter) beginAttempt(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.pruneLocked(key, time.Now())
	if len(rl.failures[key])+rl.inFlight[key] >= rl.maxFailure {
		return false
	}
	rl.inFlight[key]++
	return true
}

// endAttempt releases the slot a prior successful beginAttempt call
// reserved. Pass failed=true to convert the reservation into a real,
// window-tracked failure (the same effect recordFailure has); pass false to
// simply release it without counting against the key.
func (rl *rateLimiter) endAttempt(key string, failed bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.inFlight[key] > 0 {
		rl.inFlight[key]--
	}
	if rl.inFlight[key] == 0 {
		delete(rl.inFlight, key)
	}
	if failed {
		now := time.Now()
		rl.pruneLocked(key, now)
		rl.failures[key] = append(rl.failures[key], now)
	}
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

// sweep prunes every key, not just one that's currently being queried -
// blocked/recordFailure only ever prune the single key they're called with,
// so a key that's queried exactly once (e.g. an attacker source IP that
// never returns) would otherwise sit in the map forever. Called
// periodically from a background goroutine (see auth.go) rather than on
// every request, since it's O(distinct keys) and doesn't need per-request
// freshness.
func (rl *rateLimiter) sweep() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	for key := range rl.failures {
		rl.pruneLocked(key, now)
	}
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
