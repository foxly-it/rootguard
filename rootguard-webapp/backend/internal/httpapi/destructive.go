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
// SessionAuth.Handler's session check, so authenticatedUser here is
// expected to succeed; it's re-read anyway since guardDestructive has no
// other way to learn which session is acting.
func (a *SessionAuth) guardDestructive(event string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, _ := a.authenticatedUser(r)
		remoteIP := clientAddress(r)
		key := username
		if key == "" {
			key = remoteIP
		}

		if a.destructiveLimiter.blocked(key) {
			a.recordAuditDetail(event+"_rate_limited", username, remoteIP, r.Method+" "+r.URL.Path)
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
			return
		}
		a.destructiveLimiter.recordFailure(key)

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
