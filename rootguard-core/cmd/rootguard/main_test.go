package main

import (
	"net/http"
	"net/url"
	"testing"
)

func TestGithubReleaseTransport(t *testing.T) {
	t.Run("no proxy configured uses default transport", func(t *testing.T) {
		t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", "")
		if got := githubReleaseTransport(); got != http.DefaultTransport {
			t.Errorf("expected http.DefaultTransport, got %T", got)
		}
	})

	t.Run("valid proxy URL is applied to the transport", func(t *testing.T) {
		t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", "http://attestation-proxy:8443")
		transport, ok := githubReleaseTransport().(*http.Transport)
		if !ok {
			t.Fatalf("expected *http.Transport, got %T", githubReleaseTransport())
		}
		if transport.Proxy == nil {
			t.Fatal("expected a proxy function to be set")
		}
		proxyURL, err := transport.Proxy(&http.Request{URL: mustParseURL(t, "https://api.github.com/repos/foxly-it/rootguard/releases")})
		if err != nil {
			t.Fatalf("proxy func returned error: %v", err)
		}
		if proxyURL == nil || proxyURL.String() != "http://attestation-proxy:8443" {
			t.Errorf("expected proxy URL http://attestation-proxy:8443, got %v", proxyURL)
		}
	})

	t.Run("invalid proxy URL falls back to default transport", func(t *testing.T) {
		t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", "://not-a-valid-url")
		if got := githubReleaseTransport(); got != http.DefaultTransport {
			t.Errorf("expected http.DefaultTransport fallback, got %T", got)
		}
	})
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return parsed
}
