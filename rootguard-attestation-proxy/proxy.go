package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// Trust model: this allowlist is defense-in-depth, not an authentication
// boundary. Every process that can reach this proxy (Core, the Updater)
// already holds the Docker socket and runs as root - i.e. already has
// full host privilege - so there's no distinct, less-trusted party to
// authenticate against here. The point of this proxy is to keep the
// `control` network itself provably internet-isolated while still
// making the one narrow, legitimate egress path (cosign's own
// attestation-verification calls) explicit and auditable, rather than
// reopening internet access wholesale for everything on that network.
const (
	maxHeaderBytes     = 8 * 1024
	headerReadDeadline = 10 * time.Second
	dialTimeout        = 10 * time.Second
	idleTimeout        = 60 * time.Second
	maxConnections     = 64
)

// dialUpstream is swapped out in tests so the allowed-CONNECT/tunnel
// path can be exercised end-to-end against a local listener instead of
// a real allowlisted host - same injection pattern
// rootguard-core/internal/stack/attestation.go already uses for its own
// cosign invocation.
var dialUpstream = func(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, dialTimeout)
}

type proxyServer struct {
	sem chan struct{}
}

func newProxyServer() *proxyServer {
	return &proxyServer{sem: make(chan struct{}, maxConnections)}
}

// serve accepts connections on ln until it returns an error (e.g. the
// listener was closed). Each connection gets its own goroutine, bounded
// by sem - a connection that arrives once the cap is already full is
// rejected immediately (closed, no queueing) rather than left to pile
// up. This is a circuit breaker against a bug in Core/Updater (e.g. a
// runaway retry loop), not an anti-abuse control against an attacker -
// see the trust-model note above.
func (p *proxyServer) serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		select {
		case p.sem <- struct{}{}:
			go func() {
				defer func() { <-p.sem }()
				handleConn(conn)
			}()
		default:
			conn.Close()
		}
	}
}

func writeStatusLine(conn net.Conn, status string) {
	_, _ = conn.Write([]byte("HTTP/1.1 " + status + "\r\n\r\n"))
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	// Bounds how long a client may take to finish sending the CONNECT
	// request line/headers - cleared below once tunneling actually
	// starts, which gets its own, separate idle deadline instead.
	if err := conn.SetReadDeadline(time.Now().Add(headerReadDeadline)); err != nil {
		return
	}
	// io.LimitReader here bounds only the header-parsing phase - once
	// parsing is done we stop reading through this reader entirely
	// (after draining anything it already buffered, below), so this
	// limit can never truncate real tunnel payload.
	headerReader := bufio.NewReader(io.LimitReader(conn, maxHeaderBytes))
	req, err := http.ReadRequest(headerReader)
	if err != nil {
		return
	}
	if req.Method == http.MethodGet && req.URL.Path == "/healthz" {
		// Plain liveness probe - see main.go's runHealthcheck doc
		// comment for why this deliberately doesn't touch the
		// allowlist/tunnel logic at all.
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"))
		return
	}
	if req.Method != http.MethodConnect {
		writeStatusLine(conn, "405 Method Not Allowed")
		return
	}
	host, port, err := net.SplitHostPort(req.Host)
	if err != nil {
		writeStatusLine(conn, "400 Bad Request")
		return
	}
	if !isAllowed(host, port) {
		log.Printf("rejected CONNECT %s (not on the allowlist)", req.Host)
		writeStatusLine(conn, "403 Forbidden")
		return
	}

	upstream, err := dialUpstream("tcp", req.Host)
	if err != nil {
		log.Printf("dial %s failed: %v", req.Host, err)
		writeStatusLine(conn, "502 Bad Gateway")
		return
	}
	defer upstream.Close()

	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return
	}
	if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}
	log.Printf("tunneling to %s", req.Host)

	// A client that starts sending tunnel payload (e.g. the TLS
	// ClientHello) immediately after the CONNECT request, without
	// waiting for the 200 response, can leave bytes already sitting in
	// headerReader's own buffer - drain those to upstream before
	// switching to reading conn directly, or they're silently dropped.
	if buffered := headerReader.Buffered(); buffered > 0 {
		if _, err := io.CopyN(upstream, headerReader, int64(buffered)); err != nil {
			return
		}
	}
	tunnel(conn, upstream)
}

// tunnel copies bytes bidirectionally until both directions finish. On
// EOF (or any read error) in one direction, the destination's write
// side is half-closed (CloseWrite) so the peer sees FIN promptly rather
// than hanging until an idle timeout; the connections themselves are
// only fully closed by handleConn's own deferred Close, once both
// directions here have actually finished (not just one).
func tunnel(client, upstream net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); pipe(upstream, client) }()
	go func() { defer wg.Done(); pipe(client, upstream) }()
	wg.Wait()
}

func pipe(dst, src net.Conn) {
	buf := make([]byte, 32*1024)
	for {
		if err := src.SetReadDeadline(time.Now().Add(idleTimeout)); err != nil {
			break
		}
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}
	if half, ok := dst.(interface{ CloseWrite() error }); ok {
		_ = half.CloseWrite()
	}
}
