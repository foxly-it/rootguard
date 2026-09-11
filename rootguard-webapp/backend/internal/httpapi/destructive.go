package httpapi

import (
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/api"
)

// statusRecorder captures the status code an inner handler writes so
// guardDestructive can tell success from failure after the fact, without
// every wrapped handler having to report its own outcome explicitly.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

// destructiveLimiterKey resolves the key every destructive-action limiter
// (destructiveLimiter, restoreLimiter) is keyed by - shared by
// guardDestructive and guardRestoreUpload, found in a second-pass review
// duplicated between them. Keyed by session, not by account - found in
// review: this used to key by username, so every session the same admin
// account happens to have open (the session-inventory feature explicitly
// allows more than one) shared a single combined budget, directly
// contradicting these limiters' own documented purpose ("bound how much
// a single... session can do", see NewSessionAuth). Falls back to the
// IP-based key only when there's genuinely no session, which shouldn't
// happen given both callers' own precondition (already behind
// SessionAuth.Handler's session check) - defensive, not the normal path.
func (a *SessionAuth) destructiveLimiterKey(r *http.Request) string {
	if key, ok := a.authenticatedSessionID(r); ok {
		return key
	}
	return rateLimitKey(r)
}

// guardDestructive wraps a mutating route handler with the same
// rate-limit-then-audit shape the login/recovery handlers already use
// inline (see auth.go), generalized into one wrapper since destructive
// routes span many otherwise-unrelated handlers across the app rather than
// two closely related ones. The caller must already sit behind
// SessionAuth.Handler's session check, so authenticatedUser/
// authenticatedSessionID here are expected to succeed; they're re-read
// anyway since guardDestructive has no other way to learn which session
// is acting.
// destructiveRateLimitGate applies the shared destructiveLimiter budget -
// the same TOCTOU-safe beginAttempt/endAttempt gate guardDestructive already
// used inline - factored out so a handler that already records its own,
// more specific audit entry (handleRevokeSession's "session_revoked", see
// auth.go) can get the same rate-limit protection without also picking up
// guardDestructive's generic post-response success/failure audit on top of
// its own.
//
// beginAttempt/endAttempt, not blocked()/recordFailure() - found in review,
// the same TOCTOU gap already fixed for login/recovery (see ratelimit.go's
// own doc comment): many concurrent requests could all observe zero
// recorded uses and all be admitted before any of them got counted, so the
// limit never actually bounded concurrent volume, only sequential. Every
// attempt counts here (endAttempt(key, true) unconditionally below), not
// just failures - the thing being bounded is request volume itself,
// matching this limiter's pre-existing recordFailure-on-every-call
// behavior.
func (a *SessionAuth) destructiveRateLimitGate(event string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := a.destructiveLimiterKey(r)
		if !a.destructiveLimiter.beginAttempt(key) {
			username, _ := a.authenticatedUser(r)
			a.recordAuditDetail(event+"_rate_limited", username, clientAddress(r), r.Method+" "+r.URL.Path)
			api.WriteJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
			return
		}
		defer a.destructiveLimiter.endAttempt(key, true)
		next(w, r)
	}
}

func (a *SessionAuth) guardDestructive(event string, next http.HandlerFunc) http.HandlerFunc {
	return a.destructiveRateLimitGate(event, func(w http.ResponseWriter, r *http.Request) {
		username, _ := a.authenticatedUser(r)
		remoteIP := clientAddress(r)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next(rec, r)

		detail := r.Method + " " + r.URL.Path
		if rec.status >= 200 && rec.status < 300 {
			a.recordAuditDetail(event+"_success", username, remoteIP, detail)
		} else {
			a.recordAuditDetail(event+"_failure", username, remoteIP, detail)
		}
	})
}

// guardRestoreUpload wraps guardDestructive with an additional, much
// tighter concurrency-only gate (restoreLimiter, see its construction in
// NewSessionAuth) specifically for backup-restore and restore-preview -
// found in review: the shared destructiveLimiter's 30-per-5-minutes
// budget, combined with restore's own ~1 GiB per-request cap, let a
// single session have up to ~30 GiB of restore uploads in flight at
// once. Checked outside guardDestructive (before its own audit/shared-
// limiter logic runs), so a request this gate rejects is never even
// attributed to the shared budget - it never started. endAttempt is
// always called with failed=false: this is purely a concurrency cap, not
// a time-window quota, so a legitimate large restore never gets
// penalized against a later, unrelated restore attempt the way the
// shared limiter's own window would.
func (a *SessionAuth) guardRestoreUpload(event string, next http.HandlerFunc) http.HandlerFunc {
	guarded := a.guardDestructive(event, next)
	return func(w http.ResponseWriter, r *http.Request) {
		key := a.destructiveLimiterKey(r)
		if !a.restoreLimiter.beginAttempt(key) {
			username, _ := a.authenticatedUser(r)
			a.recordAuditDetail(event+"_rate_limited", username, clientAddress(r), r.Method+" "+r.URL.Path)
			api.WriteJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
			return
		}
		defer a.restoreLimiter.endAttempt(key, false)
		guarded(w, r)
	}
}
