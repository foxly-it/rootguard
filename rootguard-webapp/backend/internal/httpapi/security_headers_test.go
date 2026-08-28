package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
)

func TestSecurityHeadersSetsHardeningHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "http://rootguard.local:8080/", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	header := recorder.Result().Header
	if header.Get("Content-Security-Policy") != contentSecurityPolicy {
		t.Fatalf("unexpected CSP: %q", header.Get("Content-Security-Policy"))
	}
	if header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unexpected X-Content-Type-Options: %q", header.Get("X-Content-Type-Options"))
	}
	if header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("unexpected X-Frame-Options: %q", header.Get("X-Frame-Options"))
	}
	if header.Get("Referrer-Policy") != "same-origin" {
		t.Fatalf("unexpected Referrer-Policy: %q", header.Get("Referrer-Policy"))
	}
	if header.Get("Permissions-Policy") == "" {
		t.Fatal("expected a Permissions-Policy header")
	}
}

// TestSecurityHeadersWithholdsCSPForAdGuardUI is the regression test for
// why the CSP check above is scoped to "/" rather than every path: the
// proxied AdGuard Home UI under /adguard-ui/ is a separate frontend we
// don't control the assets of, and applying our SPA's script-src hash to
// it would be as likely to break it as protect it.
func TestSecurityHeadersWithholdsCSPForAdGuardUI(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "http://rootguard.local:8080/adguard-ui/", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	header := recorder.Result().Header
	if header.Get("Content-Security-Policy") != "" {
		t.Fatalf("expected no CSP for /adguard-ui/, got %q", header.Get("Content-Security-Policy"))
	}
	// The path-independent headers still apply, though.
	if header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("expected X-Content-Type-Options to still be set for /adguard-ui/")
	}
}

// TestThemeScriptCSPHashMatchesFrontendSource guards the drift this
// package's own doc comment warns about: if frontend/index.html's inline
// theme script ever changes without the hash in security_headers.go being
// recomputed, the CSP would silently start blocking it in production
// (degrading gracefully to the default theme, but still a real regression
// no other test would catch). Reads the actual source file rather than a
// copy, so it fails the moment the two drift apart.
func TestThemeScriptCSPHashMatchesFrontendSource(t *testing.T) {
	content, err := os.ReadFile("../../../frontend/index.html")
	if err != nil {
		t.Fatalf("reading frontend/index.html: %v", err)
	}
	match := regexp.MustCompile(`(?s)<script>\n(.*?)</script>`).FindSubmatch(content)
	if match == nil {
		t.Fatal("could not find the inline theme script in frontend/index.html")
	}
	sum := sha256.Sum256(match[1])
	got := "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
	if got != themeScriptCSPHash {
		t.Fatalf("theme script hash drifted: got %s, want %s (recompute themeScriptCSPHash)", got, themeScriptCSPHash)
	}
}
