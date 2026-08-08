package httpapi

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
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

type persistedCredentials struct {
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
		expectedUserHash:     sha256.Sum256([]byte(expectedUser)),
		expectedPasswordHash: passwordHash,
		passwordSalt:         passwordSalt,
		recoveryTokenHash:    sha256.Sum256([]byte(recoveryToken)),
		recoveryEnabled:      recoveryToken != "",
		ttl:                  ttl,
		sessions:             make(map[string]session),
		persistencePath:      persistencePath,
	}
	if persistencePath != "" {
		auth.credentialsPath = filepath.Join(filepath.Dir(persistencePath), "credentials.json")
	}
	auth.loadCredentials()
	auth.loadSessions()
	return auth
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
		case "/api/auth/sessions":
			a.handleSessions(w, r)
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

	var input passwordReset
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	tokenHash := sha256.Sum256([]byte(input.RecoveryToken))
	if subtle.ConstantTimeCompare(tokenHash[:], a.recoveryTokenHash[:]) != 1 {
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
	a.mu.Lock()
	a.expectedPasswordHash = passwordHash
	a.passwordSalt = passwordSalt
	clear(a.sessions)
	if err := a.persistCredentialsLocked(); err != nil {
		a.mu.Unlock()
		http.Error(w, "Unable to persist credentials", http.StatusInternalServerError)
		return
	}
	if err := a.persistLocked(); err != nil {
		a.mu.Unlock()
		http.Error(w, "Unable to invalidate sessions", http.StatusInternalServerError)
		return
	}
	a.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]bool{"reset": true})
}

func (a *SessionAuth) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
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
	passwordSalt := append([]byte(nil), a.passwordSalt...)
	expectedPasswordHash := append([]byte(nil), a.expectedPasswordHash...)
	a.mu.Unlock()
	passwordHash, err := pbkdf2.Key(sha256.New, input.Password, passwordSalt, passwordIterations, sha256.Size)
	valid := subtle.ConstantTimeCompare(userHash[:], a.expectedUserHash[:]) == 1 &&
		err == nil &&
		subtle.ConstantTimeCompare(passwordHash, expectedPasswordHash) == 1
	if !valid {
		time.Sleep(250 * time.Millisecond)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_credentials"})
		return
	}

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
		RemoteIP:  clientAddress(r),
	}
	if err := a.persistLocked(); err != nil {
		delete(a.sessions, token)
		a.mu.Unlock()
		http.Error(w, "Unable to persist session", http.StatusInternalServerError)
		return
	}
	a.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt,
		MaxAge:   int(a.ttl.Seconds()),
	})
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
		delete(a.sessions, cookie.Value)
		_ = a.persistLocked()
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
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
	for token, entry := range a.sessions {
		if entry.ID != id {
			continue
		}
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
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
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
}

func (a *SessionAuth) persistCredentialsLocked() error {
	if a.credentialsPath == "" {
		return errors.New("credential persistence is not configured")
	}
	if err := os.MkdirAll(filepath.Dir(a.credentialsPath), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(persistedCredentials{
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

// clientAddress is purely descriptive - shown in the session inventory so
// an operator can recognize their own devices - never used for any
// access-control decision, so trusting a client-spoofable header here (when
// no reverse proxy overwrites it first) carries none of the risk it would
// for an actual security check.
func clientAddress(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if first, _, ok := strings.Cut(forwarded, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(forwarded)
	}
	return r.RemoteAddr
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
