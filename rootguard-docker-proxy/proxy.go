package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"time"
)

// maxBodyBytes bounds every body this proxy actually inspects (container
// create, exec create, network connect) - all real, legitimate bodies for
// those calls are a few hundred bytes to a few KiB. Anything larger is
// already not a request this proxy's own known callers would ever send.
const maxBodyBytes = 1 << 20 // 1 MiB

// dialTimeout bounds connecting to the real Docker socket - a hung
// dockerd should surface as a clear, fast error, not a wedged proxy.
const dialTimeout = 5 * time.Second

// dialUpstream is a var, not a call wired directly into the transport, so
// tests can point it at a fake in-process Docker-API listener instead of
// a real Unix socket - same injection pattern rootguard-attestation-proxy
// uses for its own dialUpstream.
var dialUpstream = func(ctx context.Context, socketPath string) (net.Conn, error) {
	d := net.Dialer{Timeout: dialTimeout}
	return d.DialContext(ctx, "unix", socketPath)
}

// dockerProxy is the http.Handler that fronts the real Docker socket: it
// allow-lists every request by method+path, runs a body validator for the
// handful of capability-granting calls, and only then forwards to the
// real daemon via httputil.ReverseProxy. Anything not on the allowlist,
// or that fails its validator, is rejected with 403 before ever reaching
// the socket.
type dockerProxy struct {
	upstream *httputil.ReverseProxy
}

func newDockerProxy(socketPath string) *dockerProxy {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialUpstream(ctx, socketPath)
		},
	}
	rp := &httputil.ReverseProxy{
		// Rewrite, not the deprecated Director: pr.Out is already a clone
		// of the incoming request (same path/query untouched), so only
		// scheme/host need setting here.
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL.Scheme = "http"
			// The host here is never actually resolved/dialed - DialContext
			// above ignores it and always connects to socketPath - but
			// net/http requires a non-empty Host to build a valid request.
			pr.Out.URL.Host = "docker-proxy"
		},
		Transport: transport,
		ErrorLog:  log.New(os.Stderr, "rootguard-docker-proxy upstream: ", log.LstdFlags),
	}
	return &dockerProxy{upstream: rp}
}

func (p *dockerProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
		// Plain liveness probe - see main.go's runHealthcheck doc comment
		// for why this deliberately never touches the real Docker socket.
		w.WriteHeader(http.StatusOK)
		return
	}

	matched, ok := matchRule(r.Method, r.URL.Path)
	if !ok {
		log.Printf("rejected %s %s: not on the allowlist", r.Method, r.URL.Path)
		http.Error(w, "forbidden: operation not allowed", http.StatusForbidden)
		return
	}

	if matched.validate != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
		if err != nil {
			http.Error(w, "bad request: cannot read body", http.StatusBadRequest)
			return
		}
		if len(body) > maxBodyBytes {
			http.Error(w, "bad request: body too large", http.StatusBadRequest)
			return
		}
		if err := matched.validate(r, body); err != nil {
			log.Printf("rejected %s %s: %v", r.Method, r.URL.Path, err)
			http.Error(w, "forbidden: "+err.Error(), http.StatusForbidden)
			return
		}
		// The ReverseProxy reads the body a second time downstream - put
		// back exactly what was validated, since io.ReadAll above already
		// drained the original.
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
	}

	p.upstream.ServeHTTP(w, r)
}
