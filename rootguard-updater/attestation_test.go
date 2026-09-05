package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

// The three tests below mirror
// rootguard-core/internal/stack/attestation_test.go's identical
// coverage - see checkAttestationProxyReachable's own doc comment for
// the full rationale (found live, cutting 1.0.0-rc.2).
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
	if err := checkAttestationProxyReachable(); err != nil {
		t.Fatalf("expected no error against a real, reachable listener: %v", err)
	}
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
