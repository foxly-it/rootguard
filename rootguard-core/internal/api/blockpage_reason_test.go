package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/foxly-it/rootguard-core/internal/adguard"
)

// TestBlockpageReasonHandlerRejectsMissingToken is the regression test for a
// follow-up review finding: this endpoint replaces blockpage's nginx
// proxying directly to AdGuard's admin API with the real AdGuard admin
// credentials - now only Core ever holds those, and blockpage authenticates
// with a much narrower, unrelated service token instead (see
// adguard.VerifyBlockpageServiceToken). No token at all, or a wrong one,
// must never reach ReasonForHost.
func TestBlockpageReasonHandlerRejectsMissingToken(t *testing.T) {
	manager := adguard.NewManager("", "", t.TempDir(), "", t.TempDir())
	handler := blockpageReasonHandler(manager)

	request := httptest.NewRequest(http.MethodGet, "/api/blockpage/reason?host=example.com", nil)
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d", response.Code)
	}
}

func TestBlockpageReasonHandlerRejectsWrongToken(t *testing.T) {
	authDir := t.TempDir()
	writeServiceToken(t, authDir, "correct-token")
	manager := adguard.NewManager("", "", t.TempDir(), "", authDir)
	handler := blockpageReasonHandler(manager)

	request := httptest.NewRequest(http.MethodGet, "/api/blockpage/reason?host=example.com", nil)
	request.Header.Set("Authorization", "Bearer wrong-token")
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for a wrong token, got %d", response.Code)
	}
}

func TestBlockpageReasonHandlerRejectsMalformedHostWithValidToken(t *testing.T) {
	authDir := t.TempDir()
	writeServiceToken(t, authDir, "correct-token")
	manager := adguard.NewManager("", "", t.TempDir(), "", authDir)
	handler := blockpageReasonHandler(manager)

	request := httptest.NewRequest(http.MethodGet, "/api/blockpage/reason?host=not+a+host", nil)
	request.Header.Set("Authorization", "Bearer correct-token")
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a malformed host, got %d", response.Code)
	}
}

// TestBlockpageReasonRouteBypassesAdminBearerToken is the regression test
// for the routing half of the same finding: registering this handler under
// apiMux instead of root would have put it behind requireBearerToken(deps.Token,
// ...) - Core's own admin token, shared with the WebApp and the updater and
// far more powerful than anything blockpage needs. A request carrying the
// service token (never the admin one) must reach past that gate.
func TestBlockpageReasonRouteBypassesAdminBearerToken(t *testing.T) {
	authDir := t.TempDir()
	writeServiceToken(t, authDir, "service-secret")
	manager := adguard.NewManager("", "", t.TempDir(), "", authDir)
	handler := RegisterRoutes(Dependencies{Token: "admin-secret", AdGuard: manager})

	request := httptest.NewRequest(http.MethodGet, "/api/blockpage/reason?host=example.com", nil)
	request.Header.Set("Authorization", "Bearer service-secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	// ReasonForHost itself fails (apiURL is empty here, no AdGuard to call)
	// - a 502, not the point of this test. Getting anything other than 401
	// proves the service token cleared the auth gate at all, which is only
	// possible if this route bypassed requireBearerToken(deps.Token, ...).
	if response.Code == http.StatusUnauthorized {
		t.Fatal("expected the blockpage service token to bypass the admin bearer-token gate, got 401")
	}
}

func writeServiceToken(t *testing.T, authDir, token string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(authDir, "service-token"), []byte(token), 0600); err != nil {
		t.Fatal(err)
	}
}
