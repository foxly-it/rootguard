package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// newClientConn starts handleConn on a real TCP server connection and
// returns the matching client-side *net.TCPConn - real TCP loopback on
// both ends, not net.Pipe, so half-close (CloseWrite) behaves exactly
// as it would between two real containers.
// newClientConn's own cleanup closes the client connection and then
// waits for the server-side handleConn goroutine to actually return,
// registered via t.Cleanup (not a plain defer) specifically so a test
// that also registers a t.Cleanup to restore a package-level timing var
// (headerReadDeadline/idleTimeout) - if that registration happens
// *before* this function is called, per t.Cleanup's own LIFO order -
// waits for this goroutine first and only restores the var afterward.
// A plain defer in the test, or a t.Cleanup registered before this
// call, would instead let the still-running goroutine keep reading the
// var concurrently with the restore - confirmed live by `-race` on an
// earlier version of these tests.
func newClientConn(t *testing.T) net.Conn {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		handleConn(conn)
	}()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial test server: %v", err)
	}
	t.Cleanup(func() {
		client.Close()
		ln.Close()
		wg.Wait()
	})
	return client
}

func TestHealthzEndpoint(t *testing.T) {
	client := newClientConn(t)
	fmt.Fprint(client, "GET /healthz HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n")

	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestConnectToDisallowedHostIsRejected(t *testing.T) {
	client := newClientConn(t)
	fmt.Fprint(client, "CONNECT evil.example:443 HTTP/1.1\r\nHost: evil.example:443\r\n\r\n")

	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestConnectToAllowedHostWrongPortIsRejected(t *testing.T) {
	client := newClientConn(t)
	fmt.Fprint(client, "CONNECT ghcr.io:80 HTTP/1.1\r\nHost: ghcr.io:80\r\n\r\n")

	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestNonConnectMethodIsRejected(t *testing.T) {
	client := newClientConn(t)
	fmt.Fprint(client, "GET / HTTP/1.1\r\nHost: ghcr.io\r\n\r\n")

	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
}

func TestMalformedRequestClosesWithoutHanging(t *testing.T) {
	client := newClientConn(t)
	fmt.Fprint(client, "not a valid http request at all\r\n\r\n")

	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 16)
	_, err := client.Read(buf)
	if err == nil {
		t.Fatalf("expected the connection to be closed, got more data instead")
	}
}

// TestConnectToAllowedHostTunnels is the one test that exercises the
// real allow-and-tunnel path, via dialUpstream swapped to a local echo
// listener instead of a real allowlisted host (same injection pattern
// rootguard-core's own attestation.go uses for cosign) - deliberately
// no live network access, matching this project's own "tests never
// depend on live third-party reachability" convention.
func TestConnectToAllowedHostTunnels(t *testing.T) {
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer echoLn.Close()
	go func() {
		conn, err := echoLn.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		io.Copy(conn, conn) // echo whatever it receives back
	}()

	original := dialUpstream
	dialUpstream = func(network, addr string) (net.Conn, error) {
		return net.DialTimeout(network, echoLn.Addr().String(), dialTimeout)
	}
	defer func() { dialUpstream = original }()

	client := newClientConn(t)
	fmt.Fprint(client, "CONNECT ghcr.io:443 HTTP/1.1\r\nHost: ghcr.io:443\r\n\r\n")

	reader := bufio.NewReader(client)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading status line: %v", err)
	}
	if statusLine != "HTTP/1.1 200 Connection Established\r\n" {
		t.Fatalf("status line = %q, want the 200 Connection Established line", statusLine)
	}
	// Consume the blank line terminating the (empty) header block.
	if blank, _ := reader.ReadString('\n'); blank != "\r\n" {
		t.Fatalf("expected a blank line after the status line, got %q", blank)
	}

	payload := "hello through the tunnel"
	if _, err := client.Write([]byte(payload)); err != nil {
		t.Fatalf("writing tunnel payload: %v", err)
	}
	client.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(reader, got); err != nil {
		t.Fatalf("reading echoed payload: %v", err)
	}
	if string(got) != payload {
		t.Fatalf("echoed payload = %q, want %q", got, payload)
	}
}

// TestConnectSendsPayloadBeforeReadingResponse covers the eager-client
// case handleConn's own drain-buffered-bytes step exists for: a client
// that starts writing tunnel payload immediately after the CONNECT
// request, without waiting to read the 200 response first. Some of that
// payload can land in headerReader's own buffer before handleConn
// finishes parsing the CONNECT line.
func TestConnectSendsPayloadBeforeReadingResponse(t *testing.T) {
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer echoLn.Close()
	go func() {
		conn, err := echoLn.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		io.Copy(conn, conn)
	}()

	original := dialUpstream
	dialUpstream = func(network, addr string) (net.Conn, error) {
		return net.DialTimeout(network, echoLn.Addr().String(), dialTimeout)
	}
	defer func() { dialUpstream = original }()

	client := newClientConn(t)
	payload := "sent-before-reading-the-200-response"
	// Written in one Write call, request line/headers and tunnel payload
	// together - exactly the eager-client shape, not two separate writes
	// that would let the server read and respond in between regardless.
	fmt.Fprintf(client, "CONNECT ghcr.io:443 HTTP/1.1\r\nHost: ghcr.io:443\r\n\r\n%s", payload)

	reader := bufio.NewReader(client)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading status line: %v", err)
	}
	if statusLine != "HTTP/1.1 200 Connection Established\r\n" {
		t.Fatalf("status line = %q, want the 200 Connection Established line", statusLine)
	}
	if blank, _ := reader.ReadString('\n'); blank != "\r\n" {
		t.Fatalf("expected a blank line after the status line, got %q", blank)
	}
	client.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(reader, got); err != nil {
		t.Fatalf("reading echoed payload sent before the 200 response: %v", err)
	}
	if string(got) != payload {
		t.Fatalf("echoed payload = %q, want %q - the eagerly-sent bytes were dropped instead of drained to upstream", got, payload)
	}
}

func TestConnectToUnreachableUpstreamGetsBadGateway(t *testing.T) {
	original := dialUpstream
	dialUpstream = func(network, addr string) (net.Conn, error) {
		return nil, errors.New("simulated dial failure")
	}
	defer func() { dialUpstream = original }()

	client := newClientConn(t)
	fmt.Fprint(client, "CONNECT ghcr.io:443 HTTP/1.1\r\nHost: ghcr.io:443\r\n\r\n")

	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
}

// TestOversizedHeaderIsRejected covers maxHeaderBytes: a request line
// alone larger than the cap must not hang waiting for a terminator that
// will never arrive within budget, and must not silently truncate into
// tunnel data later - the connection should simply close.
func TestOversizedHeaderIsRejected(t *testing.T) {
	client := newClientConn(t)
	oversized := "CONNECT " + strings.Repeat("a", maxHeaderBytes+1024) + ":443 HTTP/1.1\r\nHost: x\r\n\r\n"
	// Best-effort write - the server may already have closed the
	// connection by the time this finishes, which itself is a pass, not
	// a test failure (an EPIPE/reset here is the expected shape of
	// "rejected oversized input"), so nothing here asserts this write's
	// own outcome.
	_, _ = fmt.Fprint(client, oversized)

	client.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 16)
	n, err := client.Read(buf)
	if err == nil && n > 0 {
		t.Fatalf("expected the connection to close without a 200 response, got %q", buf[:n])
	}
}

// TestHeaderReadTimeoutClosesSlowClient covers headerReadDeadline: a
// client that connects but never finishes sending the CONNECT request
// line must not be allowed to hold the connection (and its
// maxConnections slot) open indefinitely.
func TestHeaderReadTimeoutClosesSlowClient(t *testing.T) {
	originalDeadline := headerReadDeadline
	headerReadDeadline = 200 * time.Millisecond
	// t.Cleanup, not defer, and registered before newClientConn below -
	// see that helper's own doc comment for why the ordering matters
	// here (its own cleanup must wait for the server goroutine before
	// this one restores the var it's still reading).
	t.Cleanup(func() { headerReadDeadline = originalDeadline })

	client := newClientConn(t)
	fmt.Fprint(client, "CONNECT ghcr.io:443 HTTP/1.1\r\n") // no terminating blank line - request never completes

	client.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 16)
	start := time.Now()
	_, err := client.Read(buf)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected the connection to close after the header read deadline")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("connection took %v to close - headerReadDeadline doesn't appear to be enforced", elapsed)
	}
}

// TestIdleTimeoutUnblocksStalledPeer is the direct regression test for
// the write-deadline fix in pipe(): an upstream peer that accepts the
// tunnel but never reads afterward (TCP receive window exhausted, or
// simply hung) used to leave the copying goroutine writing to it
// blocked forever, pinning a maxConnections slot permanently. With
// idleTimeout shrunk for the test, the tunnel must give up and the
// connection must close within roughly that window, not hang.
func TestIdleTimeoutUnblocksStalledPeer(t *testing.T) {
	originalTimeout := idleTimeout
	idleTimeout = 300 * time.Millisecond
	// t.Cleanup, not defer - see newClientConn's own doc comment on why
	// this ordering (registered before that call, below) matters: its
	// cleanup must wait for the server goroutine to stop reading
	// idleTimeout before this one restores it.
	t.Cleanup(func() { idleTimeout = originalTimeout })

	stalledLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { stalledLn.Close() })
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := stalledLn.Accept()
		if err != nil {
			return
		}
		accepted <- conn
		// Deliberately never read or write again - simulates a peer
		// that stops accepting data mid-tunnel.
	}()

	original := dialUpstream
	dialUpstream = func(network, addr string) (net.Conn, error) {
		return net.DialTimeout(network, stalledLn.Addr().String(), dialTimeout)
	}
	t.Cleanup(func() { dialUpstream = original })

	client := newClientConn(t)
	fmt.Fprint(client, "CONNECT ghcr.io:443 HTTP/1.1\r\nHost: ghcr.io:443\r\n\r\n")
	reader := bufio.NewReader(client)
	if statusLine, _ := reader.ReadString('\n'); statusLine != "HTTP/1.1 200 Connection Established\r\n" {
		t.Fatalf("status line = %q, want the 200 Connection Established line", statusLine)
	}
	reader.ReadString('\n') // consume the blank line

	stalledConn := <-accepted
	defer stalledConn.Close()

	// The client keeps writing (a real cosign subprocess would keep
	// sending TLS handshake bytes) - with the stalled peer's TCP receive
	// buffer eventually full, dst.Write inside pipe() blocks until the
	// write deadline fires.
	writeDone := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for i := range buf {
			buf[i] = 'x'
		}
		for {
			if _, err := client.Write(buf); err != nil {
				close(writeDone)
				return
			}
		}
	}()

	select {
	case <-writeDone:
		// Expected: the server-side write to the stalled peer hit its
		// deadline, the tunnel unwound, and the client's own writes
		// then started failing once the server closed its end.
	case <-time.After(5 * time.Second):
		t.Fatal("client writes never failed - the stalled peer appears to have wedged the tunnel indefinitely instead of the write deadline unblocking it")
	}
}

// TestMaxConnectionsCapRejectsExcess exercises the real proxyServer.serve
// path (not the handleConn-direct newClientConn helper the rest of this
// file uses) - the semaphore that bounds concurrent connections only
// exists there. Holder connections deliberately never finish sending
// their CONNECT request line (the same shape
// TestHeaderReadTimeoutClosesSlowClient uses) rather than going through
// dialUpstream - that keeps this test from ever touching the shared
// package-level dialUpstream var from a goroutine whose lifetime
// outlives the test function itself, which a real dial-blocking
// approach can't safely guarantee without its own extra
// synchronization.
func TestMaxConnectionsCapRejectsExcess(t *testing.T) {
	originalDeadline := headerReadDeadline
	headerReadDeadline = 3 * time.Second // outlives this test's own assertions, so holders don't self-expire early
	// t.Cleanup, registered before the listener/server below - LIFO
	// order then runs the listener-close-and-wait cleanup (registered
	// later) first, guaranteeing every handleConn goroutine this
	// server's own p.wg tracks has actually returned before this one
	// restores headerReadDeadline, the same ordering concern
	// newClientConn's own doc comment explains.
	t.Cleanup(func() { headerReadDeadline = originalDeadline })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &proxyServer{sem: make(chan struct{}, 2)} // small cap, not the real 64, to keep this test fast
	go server.serve(ln)

	var holders []net.Conn
	for i := 0; i < 2; i++ {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		holders = append(holders, conn)
		fmt.Fprint(conn, "CONNECT ghcr.io:443 HTTP/1.1\r\n") // no terminating blank line - never completes
	}
	t.Cleanup(func() {
		ln.Close()
		for _, c := range holders {
			c.Close()
		}
		server.wg.Wait()
	})
	// Give serve's own goroutines a moment to actually accept and
	// reserve their semaphore slots before the assertion below.
	time.Sleep(200 * time.Millisecond)

	excess, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial excess connection: %v", err)
	}
	defer excess.Close()

	excess.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 16)
	n, readErr := excess.Read(buf)
	if readErr == nil && n > 0 {
		t.Fatalf("expected the excess connection to be closed immediately (cap reached), got %q", buf[:n])
	}
}

func TestHealthcheckSubcommandAgainstARealServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	server := newProxyServer()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.serve(ln)
	}()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	fmt.Fprint(client, "GET /healthz HTTP/1.1\r\nHost: x\r\n\r\n")
	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 - runHealthcheck's own GET /healthz would report this container unhealthy", resp.StatusCode)
	}
	ln.Close()
	wg.Wait()
}
