package stack

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestReleaseAttestationRequiresAllowlistedImmutableImage(t *testing.T) {
	called := false
	run := func(context.Context, string, ...string) ([]byte, error) { called = true; return nil, nil }
	for _, image := range []string{"rootguard-core:dev", "ghcr.io/attacker/rootguard-core:v1@sha256:abc", "ghcr.io/foxly-it/rootguard-core:latest"} {
		status, checked := verifyReleaseAttestationWith(context.Background(), "core", image, run, time.Now)
		if status != "not_applicable" || checked != "" {
			t.Fatalf("unexpected result for %s: %s %s", image, status, checked)
		}
	}
	if called {
		t.Fatal("verifier must not run for untrusted or mutable image references")
	}
}

func TestReleaseAttestationPinsSignerPolicy(t *testing.T) {
	resetAttestationCache()
	var arguments []string
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "cosign" {
			t.Fatalf("unexpected command %s", name)
		}
		arguments = args
		return []byte(`{"verified":true}`), nil
	}
	status, checked := verifyReleaseAttestationWith(context.Background(), "core", "ghcr.io/foxly-it/rootguard-core@sha256:abc", run, func() time.Time { return time.Unix(1, 0) })
	joined := strings.Join(arguments, " ")
	if status != "verified" || checked == "" {
		t.Fatalf("unexpected verification result: %s %s", status, checked)
	}
	for _, expected := range []string{"--type https://slsa.dev/provenance/v1", "foxly-it/rootguard", "release-alpha", "https://token.actions.githubusercontent.com"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing policy %q in %s", expected, joined)
		}
	}
}

// TestReleaseAttestationPinsWebappSignerPolicy guards against the policy
// silently drifting back to the archived per-component repo/workflow -
// TestReleaseAttestationPinsSignerPolicy only ever exercised "core", so
// webapp's stale policy (foxly-it/rootguard-webapp's build.yml, gone since
// the monorepo migration) went unnoticed until reported live.
func TestReleaseAttestationPinsWebappSignerPolicy(t *testing.T) {
	resetAttestationCache()
	var arguments []string
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "cosign" {
			t.Fatalf("unexpected command %s", name)
		}
		arguments = args
		return []byte(`{"verified":true}`), nil
	}
	status, checked := verifyReleaseAttestationWith(context.Background(), "webapp", "ghcr.io/foxly-it/rootguard-webapp@sha256:abc", run, func() time.Time { return time.Unix(1, 0) })
	joined := strings.Join(arguments, " ")
	if status != "verified" || checked == "" {
		t.Fatalf("unexpected verification result: %s %s", status, checked)
	}
	for _, expected := range []string{"--type https://slsa.dev/provenance/v1", "foxly-it/rootguard", "release-alpha"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing policy %q in %s", expected, joined)
		}
	}
	if strings.Contains(joined, "rootguard-webapp/.github") {
		t.Fatalf("policy still references the archived per-component repo: %s", joined)
	}
}

// TestReleaseAttestationCoversEveryReleasedComponent guards updater,
// unbound, blockpage, and attestation-proxy against the same silent policy
// gap webapp once had (see TestReleaseAttestationPinsWebappSignerPolicy) -
// all 6 components are published by the identical release-alpha.yml matrix
// build, so none of them should ever report not_applicable for a real,
// correctly signed release image.
func TestReleaseAttestationCoversEveryReleasedComponent(t *testing.T) {
	for _, service := range []string{"updater", "unbound", "blockpage", "attestation-proxy"} {
		resetAttestationCache()
		var arguments []string
		run := func(_ context.Context, name string, args ...string) ([]byte, error) {
			if name != "cosign" {
				t.Fatalf("unexpected command %s", name)
			}
			arguments = args
			return []byte(`{"verified":true}`), nil
		}
		image := "ghcr.io/foxly-it/rootguard-" + service + "@sha256:abc"
		status, checked := verifyReleaseAttestationWith(context.Background(), service, image, run, func() time.Time { return time.Unix(1, 0) })
		if status != "verified" || checked == "" {
			t.Fatalf("%s: unexpected verification result: %s %s", service, status, checked)
		}
		joined := strings.Join(arguments, " ")
		for _, expected := range []string{"--type https://slsa.dev/provenance/v1", "foxly-it/rootguard", "release-alpha", "https://token.actions.githubusercontent.com"} {
			if !strings.Contains(joined, expected) {
				t.Fatalf("%s: missing policy %q in %s", service, expected, joined)
			}
		}
	}
}

// TestReleaseAttestationAcceptsTagPlusDigestReference is the regression
// test for a real, live-impacting gap: the previous "@"-only anchor
// (added the same session, to close a same-prefix-sibling-name gap)
// broke 1.0.0-rc.3's very first fresh install. installer.Manager's own
// resolveDigest returns an already-"@sha256:"-qualified image completely
// unchanged if it already contains one - and .env.release.example's
// static ROOTGUARD_UNBOUND_IMAGE/ROOTGUARD_BLOCKPAGE_IMAGE pins are
// exactly that: "repo:1.0.0-rc.3@sha256:..." with the tag kept alongside
// the digest, unlike the self-update path's own digestFromPullOutput,
// which always strips the tag first. An anchor requiring "@" to
// immediately follow the repo name rejected every one of these as
// "not_applicable" - refused by RequireAttestation - even though the
// underlying cosign attestation was completely valid (confirmed live
// against the real published image with a raw `cosign verify-attestation`
// call). Both shapes must resolve identically.
func TestReleaseAttestationAcceptsTagPlusDigestReference(t *testing.T) {
	resetAttestationCache()
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "cosign" {
			t.Fatalf("unexpected command %s", name)
		}
		return []byte(`{"verified":true}`), nil
	}
	status, checked := verifyReleaseAttestationWith(context.Background(), "unbound", "ghcr.io/foxly-it/rootguard-unbound:1.0.0-rc.3@sha256:abc", run, func() time.Time { return time.Unix(1, 0) })
	if status != "verified" || checked == "" {
		t.Fatalf("expected a tag-plus-digest reference to verify, got: %s %s", status, checked)
	}
}

func TestReleaseAttestationCachesResultByDigestReference(t *testing.T) {
	resetAttestationCache()
	calls := 0
	now := time.Unix(10, 0)
	run := func(context.Context, string, ...string) ([]byte, error) { calls++; return nil, nil }
	image := "ghcr.io/foxly-it/rootguard-webapp@sha256:def"
	verifyReleaseAttestationWith(context.Background(), "webapp", image, run, func() time.Time { return now })
	verifyReleaseAttestationWith(context.Background(), "webapp", image, run, func() time.Time { return now.Add(time.Minute) })
	if calls != 1 {
		t.Fatalf("expected one verification, got %d", calls)
	}
}

func TestClassifyAttestationResult(t *testing.T) {
	tests := []struct {
		output   string
		err      error
		expected string
	}{
		{"", nil, "verified"},
		{"no attestations found", errors.New("exit status 1"), "missing"},
		{"certificate identity mismatch", errors.New("exit status 1"), "failed"},
		{"connection refused", errors.New("exit status 1"), "unavailable"},
		{"", context.DeadlineExceeded, "unavailable"},
	}
	for _, test := range tests {
		if actual := classifyAttestationResult([]byte(test.output), test.err); actual != test.expected {
			t.Fatalf("expected %s, got %s", test.expected, actual)
		}
	}
}

func resetAttestationCache() {
	attestationCache.Lock()
	defer attestationCache.Unlock()
	attestationCache.items = make(map[string]attestationResult)
}

// TestCheckAttestationProxyReachableUnset and the two tests below cover
// the pre-flight check found in review, round of cutting 1.0.0-rc.2:
// distinguishing "no proxy configured at all" (stale, pre-proxy compose
// topology) from "configured but unreachable" (proxy service present in
// the env var but not actually running/reachable), each with its own
// specific, actionable message - rather than letting cosign's own
// generic network-error text stand in for both.
func TestCheckAttestationProxyReachableUnset(t *testing.T) {
	t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", "")
	err := CheckAttestationProxyReachable()
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

	err := CheckAttestationProxyReachable()
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
	if err := CheckAttestationProxyReachable(); err != nil {
		t.Fatalf("expected no error against a real, reachable listener: %v", err)
	}
}

// TestRequireAttestationFailsClearlyWithoutCallingCosignWhenProxyMissing
// is the direct regression test for the fix itself: RequireAttestation
// must refuse an update with the specific, actionable message before
// ever invoking cosign - not fall through to cosign's own opaque
// network-error text - when the attestation proxy isn't configured.
func TestRequireAttestationFailsClearlyWithoutCallingCosignWhenProxyMissing(t *testing.T) {
	t.Setenv("ROOTGUARD_ATTESTATION_PROXY_URL", "")
	resetAttestationCache()
	called := false
	original := attestationRun
	attestationRun = func(context.Context, string, ...string) ([]byte, error) { called = true; return nil, nil }
	defer func() { attestationRun = original }()

	err := RequireAttestation(context.Background(), "core", "ghcr.io/foxly-it/rootguard-core:v1@sha256:abc")
	if err == nil {
		t.Fatal("expected RequireAttestation to refuse activation when no proxy is configured")
	}
	if !strings.Contains(err.Error(), "no attestation proxy configured") {
		t.Fatalf("expected the proxy-specific message, got: %v", err)
	}
	if called {
		t.Fatal("cosign must not be invoked at all once the proxy pre-check has already failed")
	}
}
