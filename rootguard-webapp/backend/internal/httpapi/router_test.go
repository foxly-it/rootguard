package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

// TestRouterMethodDispatch is router.go's first direct test - added
// alongside converting its 9 manual "if r.Method != X { 405 }" wrappers to
// Go 1.22's "METHOD /path" ServeMux patterns (found in review: duplicated
// verbatim across the file, while newer routes here already used the
// pattern).
//
// Originally verified (and asserted) that the wrong method got a bare 404
// here instead of net/http's own auto-generated 405: that auto-405 only
// fires when nothing else would match the request, and this file's SPA-
// fallback catch-all ("/", at the end of NewRouter) matches every path
// under every method, so it won ahead of the 405 rule. Fixed in a v1.0.0
// correctness review (#552) via methodMismatchAllowed, which the catch-all
// now consults directly instead of relying on ServeMux's own (here,
// unreachable) 405 synthesis - this test now asserts the corrected 405
// response instead of documenting the old 404 as intended.
func TestRouterMethodDispatch(t *testing.T) {
	core := coreclient.New("http://127.0.0.1:1", "test-token")
	auth := newTestSessionAuth()
	mux := NewRouter(core, auth)

	cases := []struct {
		path          string
		allowedMethod string
		deniedMethod  string
	}{
		{"/api/dashboard", http.MethodGet, http.MethodPost},
		{"/api/system", http.MethodGet, http.MethodPost},
		{"/api/services", http.MethodGet, http.MethodPost},
		{"/api/service/adguard/restart", http.MethodPost, http.MethodGet},
		{"/api/adguard/status", http.MethodGet, http.MethodPost},
		{"/api/adguard/bootstrap", http.MethodPost, http.MethodGet},
		{"/api/adguard/filter-report", http.MethodGet, http.MethodPost},
		{"/api/adguard/filtering", http.MethodPost, http.MethodGet},
		{"/api/adguard/protection", http.MethodPost, http.MethodGet},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			denied := httptest.NewRecorder()
			mux.ServeHTTP(denied, httptest.NewRequest(tc.deniedMethod, tc.path, nil))
			if denied.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: expected 405 for a registered path hit with the wrong method, got %d", tc.deniedMethod, tc.path, denied.Code)
			}
			if allow := denied.Header().Get("Allow"); !strings.Contains(allow, tc.allowedMethod) {
				t.Errorf("%s %s: expected Allow header to name %s, got %q", tc.deniedMethod, tc.path, tc.allowedMethod, allow)
			}

			allowed := httptest.NewRecorder()
			mux.ServeHTTP(allowed, httptest.NewRequest(tc.allowedMethod, tc.path, nil))
			if allowed.Code == http.StatusMethodNotAllowed || allowed.Code == http.StatusNotFound {
				t.Errorf("%s %s: expected the allowed method to reach the handler, got %d", tc.allowedMethod, tc.path, allowed.Code)
			}
		})
	}
}

// TestRouterReturnsNotFoundForAGenuinelyUnknownAPIPath confirms
// methodMismatchAllowed's fix didn't turn every unmatched /api/ path into
// a 405 - a path with no route at all, under any method, must still 404.
func TestRouterReturnsNotFoundForAGenuinelyUnknownAPIPath(t *testing.T) {
	core := coreclient.New("http://127.0.0.1:1", "test-token")
	auth := newTestSessionAuth()
	mux := NewRouter(core, auth)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/this-path-does-not-exist", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a genuinely unknown API path, got %d", recorder.Code)
	}
}

// TestUnboundPreviewRoutesAreRateLimited is the regression test for a real
// gap found in review: /api/unbound/custom/preview and
// /api/unbound/import/preview used to be registered as bare handlers with
// no dest() wrapper at all, unlike every other route in this file including
// their own apply siblings (PUT /api/unbound/custom, POST
// /api/unbound/import) - despite hitting the exact same applyMu-guarded,
// docker-exec-backed validateCombined() those apply routes do (see
// rootguard-core/internal/unbound/custom.go/bundle.go), making them exactly
// as expensive per request. A session could have flooded either one to
// starve every other Unbound operation on the shared applyMu.
func TestUnboundPreviewRoutesAreRateLimited(t *testing.T) {
	core := coreclient.New("http://127.0.0.1:1", "test-token")

	for _, path := range []string{"/api/unbound/custom/preview", "/api/unbound/import/preview"} {
		t.Run(path, func(t *testing.T) {
			auth := newTestSessionAuth()
			auth.destructiveLimiter = newRateLimiter(time.Minute, 1)
			mux := NewRouter(core, auth)

			first := httptest.NewRecorder()
			mux.ServeHTTP(first, httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}")))
			if first.Code == http.StatusTooManyRequests {
				t.Fatalf("expected the first request to reach the handler, got 429 immediately")
			}

			second := httptest.NewRecorder()
			mux.ServeHTTP(second, httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}")))
			if second.Code != http.StatusTooManyRequests {
				t.Fatalf("expected the second request to be rate-limited once the shared budget was exhausted, got %d", second.Code)
			}
		})
	}
}

// TestUpdateCheckRoutesAreRateLimited is the regression test for a
// follow-up review finding: /api/updates/check, /api/control-plane-
// updates/check, and /api/updater-updates/check were the only three
// mutating routes in this file registered without a dest() wrapper,
// despite each one starting a real background `docker pull` against every
// configured service's upstream image (updater.Manager.StartCheck) -
// genuinely expensive, network-bound work whose near-instant 202/409
// response let an unbounded caller re-trigger it back-to-back
// indefinitely, risking GHCR pull-rate-limit exhaustion (which would then
// also block RootGuard's own legitimate self-update mechanism) with no
// audit trail of who triggered it.
func TestUpdateCheckRoutesAreRateLimited(t *testing.T) {
	core := coreclient.New("http://127.0.0.1:1", "test-token")

	for _, path := range []string{"/api/updates/check", "/api/control-plane-updates/check", "/api/updater-updates/check"} {
		t.Run(path, func(t *testing.T) {
			auth := newTestSessionAuth()
			auth.destructiveLimiter = newRateLimiter(time.Minute, 1)
			mux := NewRouter(core, auth)

			first := httptest.NewRecorder()
			mux.ServeHTTP(first, httptest.NewRequest(http.MethodPost, path, nil))
			if first.Code == http.StatusTooManyRequests {
				t.Fatalf("expected the first request to reach the handler, got 429 immediately")
			}

			second := httptest.NewRecorder()
			mux.ServeHTTP(second, httptest.NewRequest(http.MethodPost, path, nil))
			if second.Code != http.StatusTooManyRequests {
				t.Fatalf("expected the second request to be rate-limited once the shared budget was exhausted, got %d", second.Code)
			}
		})
	}
}
