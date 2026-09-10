package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

// TestRouterMethodDispatch is router.go's first direct test - added
// alongside converting its 9 manual "if r.Method != X { 405 }" wrappers to
// Go 1.22's "METHOD /path" ServeMux patterns (found in review: duplicated
// verbatim across the file, while newer routes here already used the
// pattern).
//
// Verified while writing this test, not assumed: the wrong method here
// gets 404, not net/http's own auto-generated 405. That auto-405 only
// fires when nothing else would match the request; this file's SPA-
// fallback catch-all ("/", at the end of NewRouter) matches every path
// under every method, so it wins ahead of the 405 rule and returns its own
// 404 for anything under /api/ it doesn't recognize. This is not new: every
// other single-method route already using the plain "METHOD /path" form
// before this change (e.g. "GET /api/installation") behaves identically -
// confirmed against this same file before converting anything. These 9
// routes now match that already-established, already-shipped file
// convention instead of diverging from it with their own explicit 405.
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
			if denied.Code != http.StatusNotFound {
				t.Errorf("%s %s: expected 404 (this file's established convention for a wrong method, see the SPA-fallback comment above), got %d", tc.deniedMethod, tc.path, denied.Code)
			}

			allowed := httptest.NewRecorder()
			mux.ServeHTTP(allowed, httptest.NewRequest(tc.allowedMethod, tc.path, nil))
			if allowed.Code == http.StatusMethodNotAllowed || allowed.Code == http.StatusNotFound {
				t.Errorf("%s %s: expected the allowed method to reach the handler, got %d", tc.allowedMethod, tc.path, allowed.Code)
			}
		})
	}
}
