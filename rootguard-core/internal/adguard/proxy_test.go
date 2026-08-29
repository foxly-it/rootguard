package adguard

import (
	"net/http"
	"testing"
)

func TestRewriteAdGuardUIResponseKeepsProxyPrefix(t *testing.T) {
	response := &http.Response{Header: make(http.Header)}
	response.Header.Set("Location", "/login.html")
	response.Header.Add("Set-Cookie", "agh_session=value; Path=/; HttpOnly")

	if err := rewriteAdGuardUIResponse(response); err != nil {
		t.Fatal(err)
	}
	if location := response.Header.Get("Location"); location != "/adguard-ui/login.html" {
		t.Fatalf("unexpected rewritten location %q", location)
	}
	if cookie := response.Header.Get("Set-Cookie"); cookie != "agh_session=value; Path=/adguard-ui/; HttpOnly" {
		t.Fatalf("unexpected rewritten cookie %q", cookie)
	}
}

// TestRewriteAdGuardSetCookieHandlesRealisticVariance is the regression
// test for a review finding: the old implementation looked for the exact
// substring "Path=/;", which missed every one of these - all perfectly
// ordinary, spec-legal Set-Cookie shapes.
func TestRewriteAdGuardSetCookieHandlesRealisticVariance(t *testing.T) {
	tests := map[string]string{
		// Path=/ as the last attribute - no trailing semicolon.
		"agh_session=value; HttpOnly; Path=/": "agh_session=value; Path=/adguard-ui/; HttpOnly",
		// Different attribute-name casing.
		"agh_session=value; path=/; HttpOnly": "agh_session=value; Path=/adguard-ui/; HttpOnly",
		// Extra whitespace around the attribute.
		"agh_session=value;  Path=/ ; HttpOnly": "agh_session=value; Path=/adguard-ui/; HttpOnly",
		// Path=/ with no other attributes at all.
		"agh_session=value; Path=/": "agh_session=value; Path=/adguard-ui/",
	}
	for input, want := range tests {
		if got := rewriteAdGuardSetCookie(input); got != want {
			t.Errorf("rewriteAdGuardSetCookie(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestRewriteAdGuardSetCookieKeepsUnknownAttributes is the regression test
// for a follow-up review finding: Cookie.Unparsed holds any attribute-value
// pair ParseSetCookie couldn't map to one of its own known fields (a
// vendor-specific or not-yet-standard attribute), but Cookie.String() never
// serializes it back - so the rewrite was silently dropping anything it
// didn't specifically recognize, not just leaving it untouched.
func TestRewriteAdGuardSetCookieKeepsUnknownAttributes(t *testing.T) {
	raw := "agh_session=value; Path=/; HttpOnly; FutureAttr=xyz; AnotherOne"
	want := "agh_session=value; Path=/adguard-ui/; HttpOnly; FutureAttr=xyz; AnotherOne"
	if got := rewriteAdGuardSetCookie(raw); got != want {
		t.Fatalf("rewriteAdGuardSetCookie(%q) = %q, want %q", raw, got, want)
	}
}

// TestRewriteAdGuardSetCookieLeavesOthersAlone ensures the rewrite stays
// scoped to exactly what the old code touched: a cookie with no Path
// attribute, or a Path that isn't the bare root, is passed through
// unchanged - and an unparseable Set-Cookie value doesn't get dropped or
// corrupted, just left as-is.
func TestRewriteAdGuardSetCookieLeavesOthersAlone(t *testing.T) {
	tests := []string{
		"agh_session=value; HttpOnly",
		"agh_session=value; Path=/api; HttpOnly",
		"not a valid set-cookie at all;;;",
	}
	for _, raw := range tests {
		if got := rewriteAdGuardSetCookie(raw); got != raw {
			t.Errorf("rewriteAdGuardSetCookie(%q) = %q, want it unchanged", raw, got)
		}
	}
}
