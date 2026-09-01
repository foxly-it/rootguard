package main

import "testing"

func TestIsAllowed(t *testing.T) {
	cases := []struct {
		name string
		host string
		port string
		want bool
	}{
		{"ghcr.io allowed", "ghcr.io", "443", true},
		{"pkg-containers allowed", "pkg-containers.githubusercontent.com", "443", true},
		{"tuf-cdn allowed", "tuf-repo-cdn.sigstore.dev", "443", true},
		{"unlisted host rejected", "example.com", "443", false},
		{"unlisted host rejected even with a trailing-dot lookalike", "ghcr.io.evil.example", "443", false},
		{"allowed host, wrong port rejected", "ghcr.io", "80", false},
		{"allowed host, plain http port rejected", "ghcr.io", "8080", false},
		{"empty host rejected", "", "443", false},
		{"fulcio not on the list", "fulcio.sigstore.dev", "443", false},
		{"rekor not on the list", "rekor.sigstore.dev", "443", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isAllowed(c.host, c.port); got != c.want {
				t.Errorf("isAllowed(%q, %q) = %v, want %v", c.host, c.port, got, c.want)
			}
		})
	}
}
