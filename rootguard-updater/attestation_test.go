package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The tests below mirror rootguard-core/internal/stack/attestation_test.go's
// identical coverage - see checkAttestationProxyReachable's own doc comment
// for the full rationale (found live, cutting 1.0.0-rc.2, and again in a
// v1.0.0 correctness review when this copy was found to have drifted from
// Core's hardened one).
func TestCheckAttestationProxyReachableUnset(t *testing.T) {
	t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", "")
	err := checkAttestationProxyReachable()
	if err == nil {
		t.Fatal("expected an error when ROOTGUARD_ATTESTATION_PROXY_URL is unset")
	}
	if !strings.Contains(err.Error(), "no attestation proxy configured") {
		t.Fatalf("expected the unset-specific message, got: %v", err)
	}
}

func TestCheckAttestationProxyReachableConfiguredButUnreachable(t *testing.T) {
	t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", "http://attestation-proxy:8888")
	original := dialProxy
	dialProxy = func(network, addr string) (net.Conn, error) {
		return nil, errors.New("connection refused")
	}
	defer func() { dialProxy = original }()

	err := checkAttestationProxyReachable()
	if err == nil {
		t.Fatal("expected an error when the configured proxy is unreachable")
	}
	if !strings.Contains(err.Error(), "unreachable") || !strings.Contains(err.Error(), "attestation-proxy:8888") {
		t.Fatalf("expected the unreachable-specific message naming the configured URL, got: %v", err)
	}
}

func TestCheckAttestationProxyReachableConfiguredAndUp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", server.URL)
	if err := checkAttestationProxyReachable(); err != nil {
		t.Fatalf("expected no error against a real, healthy /healthz endpoint: %v", err)
	}
}

// TestCheckAttestationProxyReachableConfiguredWithoutScheme is the
// regression test for a second-pass review finding on Core's copy of
// this function: switching from a raw TCP dial to client.Get (for the
// fix above) accidentally dropped the old code's tolerance for a bare
// "host:port" value with no scheme - url.Parse reads the part before
// the first colon as a URI scheme rather than a hostname for a value
// like that, producing a confusing "unsupported protocol scheme"
// instead of an actual reachability check.
func TestCheckAttestationProxyReachableConfiguredWithoutScheme(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	bareHostPort := strings.TrimPrefix(server.URL, "http://")
	t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", bareHostPort)
	if err := checkAttestationProxyReachable(); err != nil {
		t.Fatalf("expected a schemeless host:port value to default to http://, got: %v", err)
	}
}

// TestCheckAttestationProxyReachableConfiguredButUnhealthy is the
// regression test for the fix itself: a bare TCP connect used to be
// enough to pass this check, even against something that merely
// accepts connections without ever answering /healthz - the real
// cosign call would then fail anyway, with only its own generic
// network-error text. A listener that accepts but never speaks HTTP at
// all must now fail this check, not just one that answers with a
// non-200 status.
func TestCheckAttestationProxyReachableConfiguredButUnhealthy(t *testing.T) {
	t.Run("non-200 healthz response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", server.URL)
		err := checkAttestationProxyReachable()
		if err == nil {
			t.Fatal("expected an error against a non-200 /healthz response")
		}
		if !strings.Contains(err.Error(), "unhealthy") {
			t.Fatalf("expected the unhealthy-specific message, got: %v", err)
		}
	})

	t.Run("accepts connections but never speaks HTTP", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer ln.Close()
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				conn.Close()
			}
		}()

		t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", "http://"+ln.Addr().String())
		if err := checkAttestationProxyReachable(); err == nil {
			t.Fatal("expected an error against a listener that never answers /healthz")
		}
	})
}

// TestVerifyAttestationFailsClearlyWhenProxyMissing is the direct
// regression test for the fix: verifyAttestation must refuse with the
// specific, actionable message before ever invoking the real cosign
// binary - not fall through to its own opaque network-error text -
// when the attestation proxy isn't configured. Uses a digest-qualified
// image matching attestationImagePrefix so the function reaches the
// proxy check at all, rather than short-circuiting on eligibility.
func TestVerifyAttestationFailsClearlyWhenProxyMissing(t *testing.T) {
	t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", "")
	err := verifyAttestation(context.Background(), "core", "ghcr.io/foxly-it/rootguard-core@sha256:abc")
	if err == nil {
		t.Fatal("expected verifyAttestation to refuse when no proxy is configured")
	}
	if !strings.Contains(err.Error(), "no attestation proxy configured") {
		t.Fatalf("expected the proxy-specific message, got: %v", err)
	}
}

// TestVerifyAttestationAcceptsTagPlusDigestReference is the regression
// test for a real, live-impacting gap found the same session: an
// "@"-only prefix anchor rejected every tag-plus-digest image reference
// (e.g. "repo:1.0.0-rc.3@sha256:...", the shape a release's own
// pre-pinned .env.release.example entries carry) as "not eligible",
// even though the underlying cosign attestation was completely valid -
// see rootguard-core/internal/stack/attestation_test.go's identical
// test for the full live-break narrative. Proxy left unset deliberately,
// so this exercises only the eligibility anchor, not the proxy check.
func TestVerifyAttestationAcceptsTagPlusDigestReference(t *testing.T) {
	t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", "")
	err := verifyAttestation(context.Background(), "core", "ghcr.io/foxly-it/rootguard-core:1.0.0-rc.3@sha256:abc")
	if err == nil || strings.Contains(err.Error(), "is not eligible for attestation verification") {
		t.Fatalf("expected a tag-plus-digest reference to pass eligibility and reach the proxy check, got: %v", err)
	}
	if !strings.Contains(err.Error(), "no attestation proxy configured") {
		t.Fatalf("expected the proxy-specific message once eligibility passed, got: %v", err)
	}
}
