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
