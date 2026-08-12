package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func loggedInRequest(t *testing.T, auth *SessionAuth, method, path string, body []byte) *http.Request {
	t.Helper()
	loginBody, _ := json.Marshal(credentials{Username: "admin", Password: "secret"})
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	login := httptest.NewRecorder()
	auth.Handler(http.NotFoundHandler()).ServeHTTP(login, loginRequest)
	if login.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", login.Code, login.Body.String())
	}

	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	request := httptest.NewRequest(method, path, reader)
	request.AddCookie(login.Result().Cookies()[0])
	return request
}

func TestGuardDestructiveRecordsSuccessAndFailure(t *testing.T) {
	auth := NewSessionAuth("admin", "secret", "", time.Hour, "")

	succeed := auth.guardDestructive("thing_done", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	fail := auth.guardDestructive("thing_done", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusBadRequest)
	})

	ok := httptest.NewRecorder()
	succeed(ok, loggedInRequest(t, auth, http.MethodPost, "/api/whatever", nil))
	if ok.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", ok.Code)
	}

	bad := httptest.NewRecorder()
	fail(bad, loggedInRequest(t, auth, http.MethodPost, "/api/whatever", nil))
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", bad.Code)
	}

	events := auth.auditSnapshot()
	var sawSuccess, sawFailure bool
	for _, event := range events {
		if event.Event == "thing_done_success" && event.Username == "admin" {
			sawSuccess = true
		}
		if event.Event == "thing_done_failure" && event.Username == "admin" {
			sawFailure = true
		}
	}
	if !sawSuccess {
		t.Fatal("expected a thing_done_success audit event")
	}
	if !sawFailure {
		t.Fatal("expected a thing_done_failure audit event")
	}
}

func TestGuardDestructiveRateLimitsSharedAcrossActions(t *testing.T) {
	auth := NewSessionAuth("admin", "secret", "", time.Hour, "")
	auth.destructiveLimiter = newRateLimiter(time.Minute, 2)

	first := auth.guardDestructive("action_one", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	second := auth.guardDestructive("action_two", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	loginBody, _ := json.Marshal(credentials{Username: "admin", Password: "secret"})
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	login := httptest.NewRecorder()
	auth.Handler(http.NotFoundHandler()).ServeHTTP(login, loginRequest)
	cookie := login.Result().Cookies()[0]

	call := func(handler http.HandlerFunc) int {
		request := httptest.NewRequest(http.MethodPost, "/api/whatever", nil)
		request.AddCookie(cookie)
		recorder := httptest.NewRecorder()
		handler(recorder, request)
		return recorder.Code
	}

	if code := call(first); code != http.StatusOK {
		t.Fatalf("expected first call to succeed, got %d", code)
	}
	if code := call(second); code != http.StatusOK {
		t.Fatalf("expected second call (different action, shared budget) to succeed, got %d", code)
	}
	if code := call(first); code != http.StatusTooManyRequests {
		t.Fatalf("expected third call across actions to be rate-limited, got %d", code)
	}

	var sawRateLimited bool
	for _, event := range auth.auditSnapshot() {
		if event.Event == "action_one_rate_limited" {
			sawRateLimited = true
		}
	}
	if !sawRateLimited {
		t.Fatal("expected an action_one_rate_limited audit event")
	}
}
