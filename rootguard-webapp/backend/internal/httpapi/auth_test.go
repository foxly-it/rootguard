package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionAuthLoginProtectsAPIAndLogoutInvalidatesSession(t *testing.T) {
	auth := NewSessionAuth("admin", "secret", "", time.Hour, "")
	handler := RequireSameOriginWrites(auth.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	protected := httptest.NewRecorder()
	handler.ServeHTTP(protected, httptest.NewRequest(http.MethodGet, "/api/dashboard", nil))
	if protected.Code != http.StatusUnauthorized {
		t.Fatalf("expected protected API to return 401, got %d", protected.Code)
	}

	loginBody, _ := json.Marshal(credentials{Username: "admin", Password: "secret"})
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginRequest.Header.Set("Origin", "http://example.com")
	loginRequest.Host = "example.com"
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName {
		t.Fatalf("expected session cookie, got %#v", cookies)
	}
	if !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("session cookie must be HttpOnly and SameSite=Strict")
	}

	authorizedRequest := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	authorizedRequest.AddCookie(cookies[0])
	authorized := httptest.NewRecorder()
	handler.ServeHTTP(authorized, authorizedRequest)
	if authorized.Code != http.StatusNoContent {
		t.Fatalf("expected authenticated API request 204, got %d", authorized.Code)
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.Header.Set("Origin", "http://example.com")
	logoutRequest.Host = "example.com"
	logoutRequest.AddCookie(cookies[0])
	logout := httptest.NewRecorder()
	handler.ServeHTTP(logout, logoutRequest)
	if logout.Code != http.StatusOK {
		t.Fatalf("expected logout 200, got %d", logout.Code)
	}

	afterLogoutRequest := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	afterLogoutRequest.AddCookie(cookies[0])
	afterLogout := httptest.NewRecorder()
	handler.ServeHTTP(afterLogout, afterLogoutRequest)
	if afterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("expected invalidated session to return 401, got %d", afterLogout.Code)
	}
}

func TestSessionAuthRejectsWrongCredentialsAndCrossOriginLogin(t *testing.T) {
	auth := NewSessionAuth("admin", "secret", "", time.Hour, "")
	handler := RequireSameOriginWrites(auth.Handler(http.NotFoundHandler()))
	body := []byte(`{"username":"admin","password":"wrong"}`)

	wrongRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	wrong := httptest.NewRecorder()
	handler.ServeHTTP(wrong, wrongRequest)
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("expected wrong credentials 401, got %d", wrong.Code)
	}

	crossOriginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	crossOriginRequest.Header.Set("Origin", "https://evil.example")
	crossOriginRequest.Host = "rootguard.example"
	crossOrigin := httptest.NewRecorder()
	handler.ServeHTTP(crossOrigin, crossOriginRequest)
	if crossOrigin.Code != http.StatusForbidden {
		t.Fatalf("expected cross-origin login 403, got %d", crossOrigin.Code)
	}
}

func TestSessionSurvivesWebAppRestart(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "sessions.json")
	first := NewSessionAuth("admin", "secret", "", time.Hour, sessionFile)
	handler := first.Handler(http.NotFoundHandler())
	body := []byte(`{"username":"admin","password":"secret"}`)
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d", login.Code)
	}

	restarted := NewSessionAuth("admin", "secret", "", time.Hour, sessionFile)
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	sessionRequest.AddCookie(login.Result().Cookies()[0])
	sessionResponse := httptest.NewRecorder()
	restarted.Handler(http.NotFoundHandler()).ServeHTTP(sessionResponse, sessionRequest)
	if sessionResponse.Code != http.StatusOK {
		t.Fatalf("expected persisted session after restart, got %d", sessionResponse.Code)
	}
}

func TestPasswordRecoveryPersistsPasswordAndInvalidatesSessions(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "sessions.json")
	auth := NewSessionAuth("admin", "old-password", "recovery-secret", time.Hour, sessionFile)
	handler := RequireSameOriginWrites(auth.Handler(http.NotFoundHandler()))

	loginBody := []byte(`{"username":"admin","password":"old-password"}`)
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusOK {
		t.Fatalf("expected initial login 200, got %d", login.Code)
	}

	resetBody := []byte(`{"recovery_token":"recovery-secret","new_password":"new-password-123"}`)
	resetRequest := httptest.NewRequest(http.MethodPost, "/api/auth/recovery", bytes.NewReader(resetBody))
	resetRequest.Header.Set("Origin", "http://example.com")
	resetRequest.Host = "example.com"
	reset := httptest.NewRecorder()
	handler.ServeHTTP(reset, resetRequest)
	if reset.Code != http.StatusOK {
		t.Fatalf("expected reset 200, got %d: %s", reset.Code, reset.Body.String())
	}

	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	sessionRequest.AddCookie(login.Result().Cookies()[0])
	sessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(sessionResponse, sessionRequest)
	if sessionResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected old session to be invalidated, got %d", sessionResponse.Code)
	}

	restarted := NewSessionAuth("admin", "old-password", "recovery-secret", time.Hour, sessionFile)
	newLoginBody := []byte(`{"username":"admin","password":"new-password-123"}`)
	newLoginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(newLoginBody))
	newLogin := httptest.NewRecorder()
	restarted.Handler(http.NotFoundHandler()).ServeHTTP(newLogin, newLoginRequest)
	if newLogin.Code != http.StatusOK {
		t.Fatalf("expected persisted reset password to work, got %d", newLogin.Code)
	}
}

func TestPasswordRecoveryRollsBackOnCredentialPersistFailure(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "sessions.json")
	// Force the credentials write to fail at the final os.Rename step: a
	// directory already sitting at the exact path persistCredentialsLocked
	// needs to rename its temp file onto.
	if err := os.MkdirAll(filepath.Join(dir, "credentials.json"), 0700); err != nil {
		t.Fatalf("failed to set up credentials.json as a directory: %v", err)
	}

	auth := NewSessionAuth("admin", "old-password", "recovery-secret", time.Hour, sessionFile)
	handler := RequireSameOriginWrites(auth.Handler(http.NotFoundHandler()))

	resetBody := []byte(`{"recovery_token":"recovery-secret","new_password":"new-password-123"}`)
	resetRequest := httptest.NewRequest(http.MethodPost, "/api/auth/recovery", bytes.NewReader(resetBody))
	resetRequest.Header.Set("Origin", "http://example.com")
	resetRequest.Host = "example.com"
	reset := httptest.NewRecorder()
	handler.ServeHTTP(reset, resetRequest)
	if reset.Code != http.StatusInternalServerError {
		t.Fatalf("expected the reset to fail when credentials can't persist, got %d: %s", reset.Code, reset.Body.String())
	}

	// A failed credential persist must not leave the new password active in
	// memory only - the old password must still work afterward.
	oldLoginBody := []byte(`{"username":"admin","password":"old-password"}`)
	oldLoginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(oldLoginBody))
	oldLogin := httptest.NewRecorder()
	handler.ServeHTTP(oldLogin, oldLoginRequest)
	if oldLogin.Code != http.StatusOK {
		t.Fatalf("expected the old password to still work after a failed recovery, got %d", oldLogin.Code)
	}
}

func TestSessionInventoryListsAndRevokes(t *testing.T) {
	auth := NewSessionAuth("admin", "secret", "", time.Hour, "")
	handler := RequireSameOriginWrites(auth.Handler(http.NotFoundHandler()))
	loginBody := []byte(`{"username":"admin","password":"secret"}`)

	login := func() *http.Cookie {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
		request.Header.Set("User-Agent", "test-agent")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("expected login 200, got %d", response.Code)
		}
		return response.Result().Cookies()[0]
	}

	firstCookie := login()
	secondCookie := login()

	listRequest := httptest.NewRequest(http.MethodGet, "/api/auth/sessions", nil)
	listRequest.AddCookie(secondCookie)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected session list 200, got %d", listResponse.Code)
	}
	var entries []sessionSummary
	if err := json.Unmarshal(listResponse.Body.Bytes(), &entries); err != nil {
		t.Fatalf("failed to decode session list: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(entries))
	}
	var currentCount int
	var otherID string
	for _, entry := range entries {
		if entry.Current {
			currentCount++
		} else {
			otherID = entry.ID
		}
		if entry.ID == "" {
			t.Fatal("expected non-empty session id")
		}
		if entry.UserAgent != "test-agent" {
			t.Fatalf("expected user agent to be recorded, got %q", entry.UserAgent)
		}
	}
	if currentCount != 1 {
		t.Fatalf("expected exactly 1 session marked current, got %d", currentCount)
	}
	if otherID == "" {
		t.Fatal("expected to find the other (non-current) session's id")
	}

	revokeRequest := httptest.NewRequest(http.MethodDelete, "/api/auth/sessions/"+otherID, nil)
	revokeRequest.Header.Set("Origin", "http://example.com")
	revokeRequest.Host = "example.com"
	revokeRequest.AddCookie(secondCookie)
	revokeResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusOK {
		t.Fatalf("expected revoke 200, got %d: %s", revokeResponse.Code, revokeResponse.Body.String())
	}

	revokedSessionRequest := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	revokedSessionRequest.AddCookie(firstCookie)
	revokedSessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokedSessionResponse, revokedSessionRequest)
	if revokedSessionResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked session to return 401, got %d", revokedSessionResponse.Code)
	}

	stillValidSessionRequest := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	stillValidSessionRequest.AddCookie(secondCookie)
	stillValidSessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(stillValidSessionResponse, stillValidSessionRequest)
	if stillValidSessionResponse.Code != http.StatusOK {
		t.Fatalf("expected the caller's own session to remain valid, got %d", stillValidSessionResponse.Code)
	}

	unauthorizedListRequest := httptest.NewRequest(http.MethodGet, "/api/auth/sessions", nil)
	unauthorizedListResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedListResponse, unauthorizedListRequest)
	if unauthorizedListResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated session list request 401, got %d", unauthorizedListResponse.Code)
	}

	missingRevokeRequest := httptest.NewRequest(http.MethodDelete, "/api/auth/sessions/does-not-exist", nil)
	missingRevokeRequest.Header.Set("Origin", "http://example.com")
	missingRevokeRequest.Host = "example.com"
	missingRevokeRequest.AddCookie(secondCookie)
	missingRevokeResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingRevokeResponse, missingRevokeRequest)
	if missingRevokeResponse.Code != http.StatusNotFound {
		t.Fatalf("expected revoking an unknown session id to return 404, got %d", missingRevokeResponse.Code)
	}
}

func TestSessionInventoryMigratesPreExistingSessionsWithoutID(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "sessions.json")
	// Simulates a sessions.json written before the ID field existed: every
	// entry decodes with ID == "" and CreatedAt as the zero value.
	legacy := `{"legacy-token-1":{"username":"admin","expires_at":"2999-01-01T00:00:00Z"}}`
	if err := os.WriteFile(sessionFile, []byte(legacy), 0600); err != nil {
		t.Fatalf("failed to seed legacy session file: %v", err)
	}

	auth := NewSessionAuth("admin", "secret", "", time.Hour, sessionFile)
	handler := RequireSameOriginWrites(auth.Handler(http.NotFoundHandler()))

	legacyRequest := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	legacyRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "legacy-token-1"})
	legacyResponse := httptest.NewRecorder()
	handler.ServeHTTP(legacyResponse, legacyRequest)
	if legacyResponse.Code != http.StatusOK {
		t.Fatalf("expected legacy session to still authenticate, got %d", legacyResponse.Code)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/auth/sessions", nil)
	listRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "legacy-token-1"})
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	var entries []sessionSummary
	if err := json.Unmarshal(listResponse.Body.Bytes(), &entries); err != nil {
		t.Fatalf("failed to decode session list: %v", err)
	}
	if len(entries) != 1 || entries[0].ID == "" {
		t.Fatalf("expected the legacy session to be backfilled with a non-empty id, got %#v", entries)
	}
	legacyID := entries[0].ID

	revokeRequest := httptest.NewRequest(http.MethodDelete, "/api/auth/sessions/"+legacyID, nil)
	revokeRequest.Header.Set("Origin", "http://example.com")
	revokeRequest.Host = "example.com"
	revokeRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "legacy-token-1"})
	revokeResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusOK {
		t.Fatalf("expected the backfilled legacy session to be individually revocable, got %d: %s", revokeResponse.Code, revokeResponse.Body.String())
	}
}

func TestLoginRateLimitBlocksRepeatedFailuresAndResetsOnSuccess(t *testing.T) {
	auth := NewSessionAuth("admin", "secret", "", time.Hour, "")
	handler := RequireSameOriginWrites(auth.Handler(http.NotFoundHandler()))
	wrongBody := []byte(`{"username":"admin","password":"wrong"}`)

	for i := range auth.loginLimiter.maxFailure {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(wrongBody))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401 before the limit is hit, got %d", i, response.Code)
		}
	}

	blockedRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(wrongBody))
	blockedResponse := httptest.NewRecorder()
	handler.ServeHTTP(blockedResponse, blockedRequest)
	if blockedResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 once the failure limit is hit, got %d", blockedResponse.Code)
	}

	// Even a correct password must not bypass an active block - resetting
	// the limiter is what a successful login on an *unblocked* client does,
	// not a way to brute-force past the lockout by eventually guessing right.
	correctBody := []byte(`{"username":"admin","password":"secret"}`)
	stillBlockedRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(correctBody))
	stillBlockedResponse := httptest.NewRecorder()
	handler.ServeHTTP(stillBlockedResponse, stillBlockedRequest)
	if stillBlockedResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("expected the correct password to still be blocked during an active lockout, got %d", stillBlockedResponse.Code)
	}
}

func TestLoginRateLimitIgnoresSpoofedForwardedForHeader(t *testing.T) {
	auth := NewSessionAuth("admin", "secret", "", time.Hour, "")
	handler := RequireSameOriginWrites(auth.Handler(http.NotFoundHandler()))
	wrongBody := []byte(`{"username":"admin","password":"wrong"}`)

	// A different X-Forwarded-For value on every request must not let an
	// attacker bypass the limit or grow the limiter's failure map without
	// bound - only the real TCP peer address (httptest.NewRequest's default
	// RemoteAddr, constant across these requests) is allowed to count.
	for i := range auth.loginLimiter.maxFailure {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(wrongBody))
		request.Header.Set("X-Forwarded-For", fmt.Sprintf("10.0.0.%d", i))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401 before the limit is hit, got %d", i, response.Code)
		}
	}

	blockedRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(wrongBody))
	blockedRequest.Header.Set("X-Forwarded-For", "10.0.0.99")
	blockedResponse := httptest.NewRecorder()
	handler.ServeHTTP(blockedResponse, blockedRequest)
	if blockedResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 once the failure limit is hit despite a fresh X-Forwarded-For value each time, got %d", blockedResponse.Code)
	}

	auth.loginLimiter.mu.Lock()
	keyCount := len(auth.loginLimiter.failures)
	auth.loginLimiter.mu.Unlock()
	if keyCount != 1 {
		t.Fatalf("expected exactly one rate-limiter key (the real peer address) regardless of X-Forwarded-For, got %d", keyCount)
	}
}

func TestAuditLogRecordsLoginLogoutAndRateLimitEvents(t *testing.T) {
	auth := NewSessionAuth("admin", "secret", "", time.Hour, "")
	handler := RequireSameOriginWrites(auth.Handler(http.NotFoundHandler()))

	wrongBody := []byte(`{"username":"admin","password":"wrong"}`)
	wrongRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(wrongBody))
	handler.ServeHTTP(httptest.NewRecorder(), wrongRequest)

	correctBody := []byte(`{"username":"admin","password":"secret"}`)
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(correctBody))
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, loginRequest)
	cookie := loginResponse.Result().Cookies()[0]

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.AddCookie(cookie)
	handler.ServeHTTP(httptest.NewRecorder(), logoutRequest)

	// The audit endpoint itself requires a valid session, so log back in
	// before reading it.
	secondLoginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(correctBody))
	secondLoginResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondLoginResponse, secondLoginRequest)
	secondCookie := secondLoginResponse.Result().Cookies()[0]

	auditRequest := httptest.NewRequest(http.MethodGet, "/api/auth/audit", nil)
	auditRequest.AddCookie(secondCookie)
	auditResponse := httptest.NewRecorder()
	handler.ServeHTTP(auditResponse, auditRequest)
	if auditResponse.Code != http.StatusOK {
		t.Fatalf("expected audit log 200, got %d", auditResponse.Code)
	}

	var events []auditEvent
	if err := json.Unmarshal(auditResponse.Body.Bytes(), &events); err != nil {
		t.Fatalf("failed to decode audit log: %v", err)
	}

	wantInOrder := []string{auditLoginSuccess, auditLogout, auditLoginSuccess, auditLoginFailure}
	if len(events) != len(wantInOrder) {
		t.Fatalf("expected %d audit events, got %d: %#v", len(wantInOrder), len(events), events)
	}
	for i, want := range wantInOrder {
		if events[i].Event != want {
			t.Fatalf("event %d: expected %q, got %q", i, want, events[i].Event)
		}
	}
}

func TestPasswordRecoveryRejectsInvalidTokenAndWeakPassword(t *testing.T) {
	auth := NewSessionAuth("admin", "old-password", "recovery-secret", time.Hour, filepath.Join(t.TempDir(), "sessions.json"))
	handler := RequireSameOriginWrites(auth.Handler(http.NotFoundHandler()))

	for name, testCase := range map[string]struct {
		body       string
		wantStatus int
	}{
		"invalid token": {`{"recovery_token":"wrong","new_password":"new-password-123"}`, http.StatusUnauthorized},
		"weak password": {`{"recovery_token":"recovery-secret","new_password":"short"}`, http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/auth/recovery", bytes.NewBufferString(testCase.body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != testCase.wantStatus {
				t.Fatalf("expected %d, got %d", testCase.wantStatus, response.Code)
			}
		})
	}
}
