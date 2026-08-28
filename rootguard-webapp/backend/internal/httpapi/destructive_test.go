package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
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

// TestGuardDestructiveRateLimitIsPerSessionNotPerAccount is the
// regression test for a real gap found in review: guardDestructive used
// to key the destructive-action limiter by username, so every session
// the same admin account happens to have open (the session-inventory
// feature explicitly allows more than one) shared a single combined
// budget - directly contradicting the limiter's own documented purpose
// ("bound how much a single... session can do", see its construction in
// NewSessionAuth).
func TestGuardDestructiveRateLimitIsPerSessionNotPerAccount(t *testing.T) {
	auth := NewSessionAuth("admin", "secret", "", time.Hour, "")
	auth.destructiveLimiter = newRateLimiter(time.Minute, 1)

	action := auth.guardDestructive("thing_done", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	call := func(cookie *http.Cookie) int {
		request := httptest.NewRequest(http.MethodPost, "/api/whatever", nil)
		request.AddCookie(cookie)
		recorder := httptest.NewRecorder()
		action(recorder, request)
		return recorder.Code
	}

	loginBody, _ := json.Marshal(credentials{Username: "admin", Password: "secret"})
	login := func() *http.Cookie {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
		response := httptest.NewRecorder()
		auth.Handler(http.NotFoundHandler()).ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("expected login 200, got %d", response.Code)
		}
		return response.Result().Cookies()[0]
	}
	sessionA := login()
	sessionB := login()

	if code := call(sessionA); code != http.StatusOK {
		t.Fatalf("expected session A's first call to succeed, got %d", code)
	}
	if code := call(sessionA); code != http.StatusTooManyRequests {
		t.Fatalf("expected session A's second call to be rate-limited (budget of 1), got %d", code)
	}
	if code := call(sessionB); code != http.StatusOK {
		t.Fatalf("expected session B (same account, different session) to have its own unused budget, got %d", code)
	}
}

// TestGuardDestructiveRateLimitBoundsTrulyConcurrentAttempts guards the
// same TOCTOU class already closed for login/recovery (see ratelimit.go's
// beginAttempt doc comment): guardDestructive used to call blocked() then
// recordFailure() as two separate mutex-protected steps, so truly
// concurrent requests could all observe zero recorded uses and all be
// admitted before any of them got counted. Unlike the login/recovery
// tests, reverting this specific fix doesn't make this exact test reliably
// fail - there's no expensive work (no PBKDF2 hashing here) between the
// two old calls to widen the window, so Go's scheduler rarely interleaves
// into it in a synchronous httptest run, even at very high goroutine
// counts. Confirmed the race is real a different way: a throwaway direct
// probe against blocked()/recordFailure() (bypassing the HTTP layer
// entirely) did show extra accepted attempts in roughly 1 of every 3
// runs. This test still guards the correct invariant under load going
// forward, just not as a guaranteed fails-without-the-fix regression
// gate the way the login/recovery ones are.
func TestGuardDestructiveRateLimitBoundsTrulyConcurrentAttempts(t *testing.T) {
	auth := NewSessionAuth("admin", "secret", "", time.Hour, "")
	auth.destructiveLimiter = newRateLimiter(time.Minute, 2)
	cookie := loggedInRequest(t, auth, http.MethodPost, "/api/whatever", nil).Cookies()[0]

	action := auth.guardDestructive("thing_done", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	const concurrentRequests = 30
	var start, done sync.WaitGroup
	start.Add(1)
	done.Add(concurrentRequests)
	codes := make([]int, concurrentRequests)
	for i := range concurrentRequests {
		go func(i int) {
			defer done.Done()
			request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/whatever/%d", i), nil)
			request.AddCookie(cookie)
			response := httptest.NewRecorder()
			start.Wait() // release every goroutine at once, not one by one
			action(response, request)
			codes[i] = response.Code
		}(i)
	}
	start.Done()
	done.Wait()

	var accepted, rejected int
	for _, code := range codes {
		switch code {
		case http.StatusOK:
			accepted++
		case http.StatusTooManyRequests:
			rejected++
		default:
			t.Fatalf("unexpected status code %d", code)
		}
	}
	if accepted != auth.destructiveLimiter.maxFailure {
		t.Fatalf("expected exactly %d concurrent requests to actually complete, got %d (rejected: %d)", auth.destructiveLimiter.maxFailure, accepted, rejected)
	}
	if rejected != concurrentRequests-auth.destructiveLimiter.maxFailure {
		t.Fatalf("expected the remaining %d requests to be rejected outright, got %d", concurrentRequests-auth.destructiveLimiter.maxFailure, rejected)
	}
}
