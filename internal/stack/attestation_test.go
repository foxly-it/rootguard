package stack

import (
	"context"
	"errors"
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
	status, checked := verifyReleaseAttestationWith(context.Background(), "core", "ghcr.io/foxly-it/rootguard-core:v1@sha256:abc", run, func() time.Time { return time.Unix(1, 0) })
	joined := strings.Join(arguments, " ")
	if status != "verified" || checked == "" {
		t.Fatalf("unexpected verification result: %s %s", status, checked)
	}
	for _, expected := range []string{"--type slsaprovenance", "foxly-it/rootguard", "release-alpha", "https://token.actions.githubusercontent.com"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing policy %q in %s", expected, joined)
		}
	}
}

func TestReleaseAttestationCachesResultByDigestReference(t *testing.T) {
	resetAttestationCache()
	calls := 0
	now := time.Unix(10, 0)
	run := func(context.Context, string, ...string) ([]byte, error) { calls++; return nil, nil }
	image := "ghcr.io/foxly-it/rootguard-webapp:v1@sha256:def"
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
