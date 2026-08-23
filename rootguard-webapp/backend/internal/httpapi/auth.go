package httpapi

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const sessionCookieName = "rootguard_session"
const passwordIterations = 600_000

type SessionAuth struct {
	// expectedUsername is kept alongside expectedUserHash (rather than just
	// the hash) so a change-account request can return/persist the current
	// value and so a no-op rename ("change password only") doesn't require
	// the caller to already know it.
	expectedUsername     string
	expectedUserHash     [32]byte
	expectedPasswordHash []byte
	passwordSalt         []byte
	recoveryTokenHash    [32]byte
	recoveryEnabled      bool
	ttl                  time.Duration
	mu                   sync.Mutex
	sessions             map[string]session
	persistencePath      string
	credentialsPath      string
	loginLimiter         *rateLimiter
	recoveryLimiter      *rateLimiter
	destructiveLimiter   *rateLimiter
	auditPath            string
	auditMu              sync.Mutex
	auditEvents          []auditEvent
}

type session struct {
	// ID is a separate opaque value from the map key (the actual bearer
	// token) specifically so the inventory/revocation API below can
	// reference a session without ever putting a live credential in a
	// response body - a leaked listing response can't be replayed as a
	// session cookie the way a leaked token could.
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	UserAgent string    `json:"user_agent"`
	RemoteIP  string    `json:"remote_ip"`
}

type sessionSummary struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	UserAgent string    `json:"user_agent"`
	RemoteIP  string    `json:"remote_ip"`
	Current   bool      `json:"current"`
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type passwordReset struct {
	RecoveryToken string `json:"recovery_token"`
	NewPassword   string `json:"new_password"`
}

type accountUpdate struct {
	CurrentPassword string `json:"current_password"`
	NewUsername     string `json:"new_username"`
	NewPassword     string `json:"new_password"`
}

type persistedCredentials struct {
	// Username is omitted (empty) on credentials.json files written before
	// this field existed, or ones that only ever went through the recovery
	// flow (which never touches it) - loadCredentials treats an empty value
	// as "keep the env-var-configured username" rather than as an actual
	// stored empty username.
	Username     string `json:"username,omitempty"`
	Algorithm    string `json:"algorithm"`
	Iterations   int    `json:"iterations"`
	Salt         string `json:"salt"`
	PasswordHash string `json:"password_hash"`
}

func NewSessionAuth(expectedUser, expectedPassword, recoveryToken string, ttl time.Duration, persistencePath string) *SessionAuth {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	passwordSalt, passwordHash, err := securePassword(expectedPassword)
	if err != nil {
		panic("unable to initialize password hash: " + err.Error())
	}
	auth := &SessionAuth{
		expectedUsername:     expectedUser,
		expectedUserHash:     sha256.Sum256([]byte(expectedUser)),
		expectedPasswordHash: passwordHash,
		passwordSalt:         passwordSalt,
		recoveryTokenHash:    sha256.Sum256([]byte(recoveryToken)),
		recoveryEnabled:      recoveryToken != "",
		ttl:                  ttl,
		sessions:             make(map[string]session),
		persistencePath:      persistencePath,
		// 5 failures per 5 minutes is generous enough that a fumbled
		// real password never locks anyone out, but stops an online
		// brute-force attempt from making more than a handful of guesses
		// a minute. The recovery token is a long random secret rather
		// than an operator-chosen password, but gets the same protection
		// for defense in depth rather than relying on its length alone.
		loginLimiter:    newRateLimiter(5*time.Minute, 5),
		recoveryLimiter: newRateLimiter(5*time.Minute, 5),
		// A single shared budget across every destructive route (Unbound
		// activation, service updates, backup restore, AdGuard bootstrap,
		// ...) rather than one limiter per route - the goal is to bound how
		// much a single (possibly compromised) session can do overall, not
		// to give every individual action its own separate allowance. 30
		// requests in 5 minutes comfortably covers an operator actively
		// working through Setup or Unbound configuration while still
		// stopping a runaway script.
		destructiveLimiter: newRateLimiter(5*time.Minute, 30),
	}
	if persistencePath != "" {
		auth.credentialsPath = filepath.Join(filepath.Dir(persistencePath), "credentials.json")
		auth.auditPath = filepath.Join(filepath.Dir(persistencePath), "audit.json")
	}
	auth.loadCredentials()
	auth.loadSessions()
	auth.loadAudit()
	go auth.sweepLimitersPeriodically()
	return auth
}

// sweepLimitersPeriodically bounds long-run memory growth from rate-limiter
// keys that are only ever queried once (see rateLimiter.sweep) - runs for
// the lifetime of the process, same as the server itself.
func (a *SessionAuth) sweepLimitersPeriodically() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		a.loginLimiter.sweep()
		a.recoveryLimiter.sweep()
		a.destructiveLimiter.sweep()
	}
}

func (a *SessionAuth) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			a.handleLogin(w, r)
			return
		case "/api/auth/logout":
			a.handleLogout(w, r)
			return
		case "/api/auth/session":
			a.handleSession(w, r)
			return
		case "/api/auth/recovery":
			a.handleRecovery(w, r)
			return
		case "/api/auth/account":
			a.handleAccount(w, r)
			return
		case "/api/auth/sessions":
			a.handleSessions(w, r)
			return
		case "/api/auth/audit":
			a.handleAudit(w, r)
			return
		case "/health":
			next.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/auth/sessions/") {
			a.handleRevokeSession(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/adguard-ui/") {
			if _, ok := a.authenticatedUser(r); !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"authenticated": false})
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (a *SessionAuth) handleRecovery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]bool{"enabled": a.recoveryEnabled})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.recoveryEnabled {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "recovery_disabled"})
		return
	}

	remoteIP := clientAddress(r)
	limiterKey := rateLimitKey(r)
	if a.recoveryLimiter.blocked(limiterKey) {
		a.recordAudit(auditLoginRateLimited, "", remoteIP)
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
		return
	}

	var input passwordReset
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	tokenHash := sha256.Sum256([]byte(input.RecoveryToken))
	if subtle.ConstantTimeCompare(tokenHash[:], a.recoveryTokenHash[:]) != 1 {
		a.recoveryLimiter.recordFailure(limiterKey)
		a.recordAudit(auditRecoveryFailure, "", remoteIP)
		time.Sleep(250 * time.Millisecond)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_recovery_token"})
		return
	}
	if len(input.NewPassword) < 12 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "weak_password"})
		return
	}

	passwordSalt, passwordHash, err := securePassword(input.NewPassword)
	if err != nil {
		http.Error(w, "Unable to secure credentials", http.StatusInternalServerError)
		return
	}
	// Order matters for failure safety: apply and persist the session wipe
	// first, then the new password, rolling each back individually on a
	// write failure - so a partial failure can only ever leave the *old*
	// password in effect (safe: sessions get needlessly cleared, forcing a
	// re-login, never a security regression) and can never leave a *new*
	// password active in memory without it having actually reached disk.
	// The previous unconditional order (mutate both, then persist both)
	// could do the opposite: hold a new password in memory that failed to
	// persist (reverts on restart while accepted until then), or persist
	// the new password while a stale sessions.json survives to revive
	// invalidated sessions after a future restart.
	a.mu.Lock()
	oldSessions := a.sessions
	a.sessions = make(map[string]session)
	if err := a.persistLocked(); err != nil {
		a.sessions = oldSessions
		a.mu.Unlock()
		http.Error(w, "Unable to invalidate sessions", http.StatusInternalServerError)
		return
	}
	oldPasswordHash, oldPasswordSalt := a.expectedPasswordHash, a.passwordSalt
	a.expectedPasswordHash = passwordHash
	a.passwordSalt = passwordSalt
	if err := a.persistCredentialsLocked(); err != nil {
		a.expectedPasswordHash, a.passwordSalt = oldPasswordHash, oldPasswordSalt
		a.mu.Unlock()
		http.Error(w, "Unable to persist credentials", http.StatusInternalServerError)
		return
	}
	a.mu.Unlock()
	a.recoveryLimiter.reset(limiterKey)
	a.recordAudit(auditRecoverySuccess, "", remoteIP)
	writeJSON(w, http.StatusOK, map[string]bool{"reset": true})
}

// handleAccount lets the currently logged-in operator change their own
// username and/or password, re-authenticating with the current password
// rather than the recovery token (unlike handleRecovery, which is a
// locked-out escape hatch and always wipes every session). Unlike recovery,
// this keeps the calling session alive and only invalidates every *other*
// session, so a credential change on one device doesn't also log the
// operator out of the browser tab they just used to make it.
func (a *SessionAuth) handleAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")

	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"authenticated": false})
		return
	}
	if _, ok := a.authenticatedUser(r); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"authenticated": false})
		return
	}
	currentToken := cookie.Value
	remoteIP := clientAddress(r)

	limiterKey := rateLimitKey(r)
	if a.loginLimiter.blocked(limiterKey) {
		a.recordAudit(auditLoginRateLimited, "", remoteIP)
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
		return
	}

	var input accountUpdate
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	newUsername := strings.TrimSpace(input.NewUsername)
	if newUsername == "" && input.NewPassword == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nothing_to_update"})
		return
	}
	if len(newUsername) > 128 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username_too_long"})
		return
	}
	if input.NewPassword != "" && len(input.NewPassword) < 12 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "weak_password"})
		return
	}

	a.mu.Lock()
	passwordSalt := append([]byte(nil), a.passwordSalt...)
	expectedPasswordHash := append([]byte(nil), a.expectedPasswordHash...)
	a.mu.Unlock()
	currentPasswordHash, err := pbkdf2.Key(sha256.New, input.CurrentPassword, passwordSalt, passwordIterations, sha256.Size)
	if err != nil || subtle.ConstantTimeCompare(currentPasswordHash, expectedPasswordHash) != 1 {
		a.loginLimiter.recordFailure(limiterKey)
		a.recordAudit(auditAccountFailure, "", remoteIP)
		time.Sleep(250 * time.Millisecond)
		// 403, not 401: the caller's session cookie is valid (already
		// confirmed above) - only the submitted current-password field is
		// wrong. The frontend's shared API client treats any 401 as "the
		// session itself is invalid" and clears the local login state on
		// it, so a 401 here would silently sign a correctly-logged-in
		// operator out just for mistyping their current password.
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid_current_password"})
		return
	}
	a.loginLimiter.reset(limiterKey)

	newPasswordSalt, newPasswordHash := passwordSalt, expectedPasswordHash
	if input.NewPassword != "" {
		newPasswordSalt, newPasswordHash, err = securePassword(input.NewPassword)
		if err != nil {
			http.Error(w, "Unable to secure credentials", http.StatusInternalServerError)
			return
		}
	}

	// Unlike handleRecovery (a single resource - only ever the password),
	// this handler mutates two persisted resources - credentials and
	// sessions - and the credential write must go first specifically so a
	// failure there needs no session rollback at all: nothing session-side
	// has been touched yet, so it's a clean no-op failure. Doing it in the
	// other order (as an earlier version of this code did) let a
	// credential-persist failure roll back the in-memory username/password
	// while the already-persisted session wipe stayed applied - leaving the
	// calling session's stored Username renamed on disk even though the
	// account's real username had reverted. Once the credential write does
	// succeed, it's durably committed; a subsequent session-persist failure
	// only rolls back the in-memory session map (matching sessions.json,
	// which never got overwritten) and costs a needless "other devices stay
	// logged in a bit longer," never a credential inconsistency.
	a.mu.Lock()
	oldUsername, oldUserHash := a.expectedUsername, a.expectedUserHash
	oldPasswordHash, oldPasswordSalt := a.expectedPasswordHash, a.passwordSalt
	if newUsername != "" {
		a.expectedUsername = newUsername
		a.expectedUserHash = sha256.Sum256([]byte(newUsername))
	}
	a.expectedPasswordHash = newPasswordHash
	a.passwordSalt = newPasswordSalt
	if err := a.persistCredentialsLocked(); err != nil {
		a.expectedUsername, a.expectedUserHash = oldUsername, oldUserHash
		a.expectedPasswordHash, a.passwordSalt = oldPasswordHash, oldPasswordSalt
		a.mu.Unlock()
		http.Error(w, "Unable to persist credentials", http.StatusInternalServerError)
		return
	}
	resultUsername := a.expectedUsername

	// The current session's entry is kept (and its stored Username updated
	// in place, since the rename above just durably committed) so
	// /api/auth/session immediately reflects it without requiring the
	// caller to log back in; every other session is invalidated.
	oldSessions := a.sessions
	currentSession, hasCurrent := oldSessions[currentToken]
	a.sessions = map[string]session{}
	if hasCurrent {
		currentSession.Username = resultUsername
		a.sessions[currentToken] = currentSession
	}
	if err := a.persistLocked(); err != nil {
		a.sessions = oldSessions
		// The credential write above already succeeded and is durably on
		// disk - leaving it in place here would mean the response says
		// "failed" while the account was actually renamed/repassworded.
		// Try to undo it by persisting the old credentials back.
		newUsernameApplied, newUserHashApplied := a.expectedUsername, a.expectedUserHash
		newPasswordHashApplied, newPasswordSaltApplied := a.expectedPasswordHash, a.passwordSalt
		a.expectedUsername, a.expectedUserHash = oldUsername, oldUserHash
		a.expectedPasswordHash, a.passwordSalt = oldPasswordHash, oldPasswordSalt
		if rollbackErr := a.persistCredentialsLocked(); rollbackErr != nil {
			// Both writes failed: disk still holds the NEW credentials from
			// the successful write above (the rollback attempt's own write
			// failed to overwrite it), so keep memory matching disk instead
			// of diverging from what's actually durable. This is a genuine
			// partial state - say so explicitly rather than returning the
			// same generic 500 as a clean rollback, so the operator knows
			// to sign in with the *new* credentials and check other devices
			// themselves rather than assuming nothing happened.
			a.expectedUsername, a.expectedUserHash = newUsernameApplied, newUserHashApplied
			a.expectedPasswordHash, a.passwordSalt = newPasswordHashApplied, newPasswordSaltApplied
			a.mu.Unlock()
			a.recordAuditDetail(auditAccountPartial, resultUsername, remoteIP, "credentials changed, session invalidation failed")
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error":    "partial_update",
				"username": resultUsername,
			})
			return
		}
		a.mu.Unlock()
		http.Error(w, "Unable to invalidate sessions", http.StatusInternalServerError)
		return
	}
	a.mu.Unlock()

	var detail string
	switch {
	case newUsername != "" && input.NewPassword != "":
		detail = "username,password"
	case newUsername != "":
		detail = "username"
	default:
		detail = "password"
	}
	a.recordAuditDetail(auditAccountUpdated, resultUsername, remoteIP, detail)
	writeJSON(w, http.StatusOK, map[string]any{"updated": true, "username": resultUsername})
}

// setSessionCookie writes the session cookie shared shape used both to set
// a fresh session (handleLogin) and to clear one (handleLogout, value=""
// and an already-expired Expires/MaxAge=-1).
func (a *SessionAuth) setSessionCookie(w http.ResponseWriter, r *http.Request, value string, expires time.Time, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteStrictMode,
		Expires:  expires,
		MaxAge:   maxAge,
	})
}

func (a *SessionAuth) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	remoteIP := clientAddress(r)
	limiterKey := rateLimitKey(r)
	if a.loginLimiter.blocked(limiterKey) {
		a.recordAudit(auditLoginRateLimited, "", remoteIP)
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
		return
	}

	var input credentials
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	userHash := sha256.Sum256([]byte(input.Username))
	a.mu.Lock()
	expectedUserHash := a.expectedUserHash
	passwordSalt := append([]byte(nil), a.passwordSalt...)
	expectedPasswordHash := append([]byte(nil), a.expectedPasswordHash...)
	a.mu.Unlock()
	passwordHash, err := pbkdf2.Key(sha256.New, input.Password, passwordSalt, passwordIterations, sha256.Size)
	valid := subtle.ConstantTimeCompare(userHash[:], expectedUserHash[:]) == 1 &&
		err == nil &&
		subtle.ConstantTimeCompare(passwordHash, expectedPasswordHash) == 1
	if !valid {
		a.loginLimiter.recordFailure(limiterKey)
		a.recordAudit(auditLoginFailure, input.Username, remoteIP)
		time.Sleep(250 * time.Millisecond)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_credentials"})
		return
	}
	a.loginLimiter.reset(limiterKey)
	a.recordAudit(auditLoginSuccess, input.Username, remoteIP)

	token, err := randomSessionToken()
	if err != nil {
		http.Error(w, "Unable to create session", http.StatusInternalServerError)
		return
	}
	sessionID, err := randomSessionToken()
	if err != nil {
		http.Error(w, "Unable to create session", http.StatusInternalServerError)
		return
	}
	expiresAt := time.Now().Add(a.ttl)
	a.mu.Lock()
	a.deleteExpiredLocked(time.Now())
	a.sessions[token] = session{
		ID:        sessionID,
		Username:  input.Username,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
		UserAgent: r.Header.Get("User-Agent"),
		RemoteIP:  remoteIP,
	}
	if err := a.persistLocked(); err != nil {
		delete(a.sessions, token)
		a.mu.Unlock()
		http.Error(w, "Unable to persist session", http.StatusInternalServerError)
		return
	}
	a.mu.Unlock()

	a.setSessionCookie(w, r, token, expiresAt, int(a.ttl.Seconds()))
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "username": input.Username})
}

func (a *SessionAuth) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		a.mu.Lock()
		entry, existed := a.sessions[cookie.Value]
		delete(a.sessions, cookie.Value)
		persistErr := a.persistLocked()
		a.mu.Unlock()
		// Mirrors handleRevokeSession: a failed persist here can leave a
		// stale sessions.json that revives this exact session on the next
		// restart, even though the browser's own cookie is already gone -
		// worth surfacing as an error rather than silently reporting a
		// clean logout that isn't durable yet.
		if persistErr != nil {
			http.Error(w, "Unable to persist session revocation", http.StatusInternalServerError)
			return
		}
		if existed {
			a.recordAudit(auditLogout, entry.Username, clientAddress(r))
		}
	}
	a.setSessionCookie(w, r, "", time.Unix(1, 0), -1)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func (a *SessionAuth) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	username, ok := a.authenticatedUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "username": username})
}

func (a *SessionAuth) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := a.authenticatedUser(r); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"authenticated": false})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	currentToken := ""
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		currentToken = cookie.Value
	}

	now := time.Now()
	a.mu.Lock()
	a.deleteExpiredLocked(now)
	entries := make([]sessionSummary, 0, len(a.sessions))
	for token, entry := range a.sessions {
		entries = append(entries, sessionSummary{
			ID:        entry.ID,
			Username:  entry.Username,
			CreatedAt: entry.CreatedAt,
			ExpiresAt: entry.ExpiresAt,
			UserAgent: entry.UserAgent,
			RemoteIP:  entry.RemoteIP,
			Current:   token == currentToken,
		})
	}
	a.mu.Unlock()

	sort.Slice(entries, func(i, j int) bool { return entries[i].CreatedAt.After(entries[j].CreatedAt) })
	writeJSON(w, http.StatusOK, entries)
}

func (a *SessionAuth) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := a.authenticatedUser(r); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"authenticated": false})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	id := strings.TrimPrefix(r.URL.Path, "/api/auth/sessions/")
	if id == "" {
		http.Error(w, "Missing session id", http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	var found bool
	var revokedUsername string
	for token, entry := range a.sessions {
		if entry.ID != id {
			continue
		}
		revokedUsername = entry.Username
		delete(a.sessions, token)
		found = true
		break
	}
	var persistErr error
	if found {
		persistErr = a.persistLocked()
	}
	a.mu.Unlock()

	if !found {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}
	if persistErr != nil {
		http.Error(w, "Unable to persist session revocation", http.StatusInternalServerError)
		return
	}
	a.recordAudit(auditSessionRevoked, revokedUsername, clientAddress(r))
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (a *SessionAuth) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := a.authenticatedUser(r); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"authenticated": false})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, a.auditSnapshot())
}

func (a *SessionAuth) authenticatedUser(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return "", false
	}
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	entry, ok := a.sessions[cookie.Value]
	if !ok || !entry.ExpiresAt.After(now) {
		delete(a.sessions, cookie.Value)
		_ = a.persistLocked()
		return "", false
	}
	return entry.Username, true
}

func (a *SessionAuth) deleteExpiredLocked(now time.Time) {
	for token, entry := range a.sessions {
		if !entry.ExpiresAt.After(now) {
			delete(a.sessions, token)
		}
	}
}

func (a *SessionAuth) loadSessions() {
	if a.persistencePath == "" {
		return
	}
	data, err := os.ReadFile(a.persistencePath)
	if err != nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_ = json.Unmarshal(data, &a.sessions)
	a.deleteExpiredLocked(time.Now())
	// Sessions persisted before the ID field existed all decode with the
	// same zero value ("") - left as-is, that collides as a React list key
	// on the frontend and can never be revoked individually (the revoke
	// endpoint rejects an empty id). Backfilling a fresh ID (and a
	// reasonable CreatedAt, since that field is equally absent on these
	// entries) self-heals the persisted file on the very next load instead
	// of leaving every pre-upgrade session broken until it naturally
	// expires.
	var migrated bool
	for token, entry := range a.sessions {
		if entry.ID != "" {
			continue
		}
		id, err := randomSessionToken()
		if err != nil {
			continue
		}
		entry.ID = id
		if entry.CreatedAt.IsZero() {
			entry.CreatedAt = time.Now()
		}
		a.sessions[token] = entry
		migrated = true
	}
	if migrated {
		_ = a.persistLocked()
	}
}

func (a *SessionAuth) loadCredentials() {
	if a.credentialsPath == "" {
		return
	}
	data, err := os.ReadFile(a.credentialsPath)
	if err != nil {
		return
	}
	var stored persistedCredentials
	if json.Unmarshal(data, &stored) != nil {
		return
	}
	if stored.Algorithm != "pbkdf2-sha256" || stored.Iterations != passwordIterations {
		return
	}
	salt, err := base64.RawStdEncoding.DecodeString(stored.Salt)
	if err != nil || len(salt) != 16 {
		return
	}
	passwordHash, err := base64.RawStdEncoding.DecodeString(stored.PasswordHash)
	if err != nil || len(passwordHash) != sha256.Size {
		return
	}
	a.passwordSalt = salt
	a.expectedPasswordHash = passwordHash
	if stored.Username != "" {
		a.expectedUsername = stored.Username
		a.expectedUserHash = sha256.Sum256([]byte(stored.Username))
	}
}

func (a *SessionAuth) persistCredentialsLocked() error {
	if a.credentialsPath == "" {
		return errors.New("credential persistence is not configured")
	}
	if err := os.MkdirAll(filepath.Dir(a.credentialsPath), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(persistedCredentials{
		Username:     a.expectedUsername,
		Algorithm:    "pbkdf2-sha256",
		Iterations:   passwordIterations,
		Salt:         base64.RawStdEncoding.EncodeToString(a.passwordSalt),
		PasswordHash: base64.RawStdEncoding.EncodeToString(a.expectedPasswordHash),
	})
	if err != nil {
		return err
	}
	temp := a.credentialsPath + ".tmp"
	if err := os.WriteFile(temp, data, 0600); err != nil {
		return err
	}
	return os.Rename(temp, a.credentialsPath)
}

func securePassword(password string) ([]byte, []byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, err
	}
	passwordHash, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, sha256.Size)
	if err != nil {
		return nil, nil, err
	}
	return salt, passwordHash, nil
}

func (a *SessionAuth) persistLocked() error {
	if a.persistencePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(a.persistencePath), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(a.sessions)
	if err != nil {
		return err
	}
	temp := a.persistencePath + ".tmp"
	if err := os.WriteFile(temp, data, 0600); err != nil {
		return err
	}
	return os.Rename(temp, a.persistencePath)
}

func randomSessionToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buffer)
	if token == "" {
		return "", errors.New("empty session token")
	}
	return token, nil
}

func requestIsHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// clientAddress is purely descriptive - shown in the session inventory and
// audit log so an operator can recognize their own devices - never used for
// any access-control decision. See rateLimitKey for the address actually
// used to make a decision.
func clientAddress(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if first, _, ok := strings.Cut(forwarded, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(forwarded)
	}
	return r.RemoteAddr
}

// rateLimitKey is the actual TCP peer address, used to key every rate
// limiter. Unlike clientAddress, this must never trust a client-controlled
// header: RootGuard's WebApp container publishes its port directly (no
// built-in reverse proxy hop), and none of the documented reverse-proxy
// setups (docs/https-reverse-proxy.md) ask an operator to forward
// X-Forwarded-For - only X-Forwarded-Proto, for the session cookie's Secure
// flag. Trusting X-Forwarded-For here would let a caller send a different
// value on every request, both bypassing the limit entirely and growing the
// limiter's failure map without bound.
func rateLimitKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
