package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
