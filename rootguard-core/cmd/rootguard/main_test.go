package main

import (
	"context"
	"net/http"
	"net/http/httptest"
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

	t.Run("schemeless host:port defaults to http", func(t *testing.T) {
		t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", "attestation-proxy:8443")
		transport, ok := githubReleaseTransport().(*http.Transport)
		if !ok {
			t.Fatalf("expected *http.Transport, got %T", githubReleaseTransport())
		}
		proxyURL, err := transport.Proxy(&http.Request{URL: mustParseURL(t, "https://api.github.com/repos/foxly-it/rootguard/releases")})
		if err != nil {
			t.Fatalf("proxy func returned error: %v", err)
		}
		if proxyURL == nil || proxyURL.String() != "http://attestation-proxy:8443" {
			t.Errorf("expected proxy URL http://attestation-proxy:8443, got %v", proxyURL)
		}
	})

	t.Run("empty host falls back to default transport", func(t *testing.T) {
		t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", "http://")
		if got := githubReleaseTransport(); got != http.DefaultTransport {
			t.Errorf("expected http.DefaultTransport fallback, got %T", got)
		}
	})
}

func TestCheckBlockpageHealthyAt(t *testing.T) {
	t.Run("200 OK is healthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		if err := checkBlockpageHealthyAt(context.Background(), server.URL); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("non-200 status is unhealthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()
		if err := checkBlockpageHealthyAt(context.Background(), server.URL); err == nil {
			t.Fatal("expected an error for a non-200 status")
		}
	})

	t.Run("unreachable server is unhealthy", func(t *testing.T) {
		if err := checkBlockpageHealthyAt(context.Background(), "http://127.0.0.1:1"); err == nil {
			t.Fatal("expected an error for an unreachable server")
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
