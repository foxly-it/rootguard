package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// newClientConn starts handleConn on a real TCP server connection and
// returns the matching client-side *net.TCPConn - real TCP loopback on
// both ends, not net.Pipe, so half-close (CloseWrite) behaves exactly
// as it would between two real containers.
func newClientConn(t *testing.T) net.Conn {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
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
	t.Cleanup(func() { client.Close() })
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
