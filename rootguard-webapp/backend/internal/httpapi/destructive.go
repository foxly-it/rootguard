package httpapi

import "net/http"

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

// guardDestructive wraps a mutating route handler with the same
// rate-limit-then-audit shape the login/recovery handlers already use
// inline (see auth.go), generalized into one wrapper since destructive
// routes span many otherwise-unrelated handlers across the app rather than
// two closely related ones. The caller must already sit behind
// SessionAuth.Handler's session check, so authenticatedUser/
// authenticatedSessionID here are expected to succeed; they're re-read
// anyway since guardDestructive has no other way to learn which session
// is acting.
func (a *SessionAuth) guardDestructive(event string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, _ := a.authenticatedUser(r)
		remoteIP := clientAddress(r)
		// Keyed by session, not by account - found in review: this used
		// to key by username, so every session the same admin account
		// happens to have open (the session-inventory feature explicitly
		// allows more than one) shared a single combined budget, directly
		// contradicting this limiter's own documented purpose ("bound how
		// much a single... session can do", see its construction in
		// NewSessionAuth). Falls back to the IP-based key only when
		// there's genuinely no session, which shouldn't happen given the
		// caller's own precondition above - defensive, not the normal
		// path.
		key, ok := a.authenticatedSessionID(r)
		if !ok {
			key = rateLimitKey(r)
		}

		// beginAttempt/endAttempt, not blocked()/recordFailure() - found
		// in review, the same TOCTOU gap already fixed for login/recovery
		// (see ratelimit.go's own doc comment): many concurrent requests
		// could all observe zero recorded uses and all be admitted before
		// any of them got counted, so the limit never actually bounded
		// concurrent volume, only sequential. Every attempt counts here
		// (endAttempt(key, true) unconditionally below), not just
		// failures - the thing being bounded is request volume itself,
		// matching this limiter's pre-existing recordFailure-on-every-call
		// behavior.
		if !a.destructiveLimiter.beginAttempt(key) {
			a.recordAuditDetail(event+"_rate_limited", username, remoteIP, r.Method+" "+r.URL.Path)
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
			return
		}
		defer a.destructiveLimiter.endAttempt(key, true)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next(rec, r)

		detail := r.Method + " " + r.URL.Path
		if rec.status >= 200 && rec.status < 300 {
			a.recordAuditDetail(event+"_success", username, remoteIP, detail)
		} else {
			a.recordAuditDetail(event+"_failure", username, remoteIP, detail)
		}
	}
}
