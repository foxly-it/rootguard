// rootguard-attestation-proxy is a minimal, purpose-built CONNECT-only
// forward proxy. Core and the Updater both hold the Docker socket and
// run only on the `control` Docker network, which is deliberately
// `internal: true` (no route to the internet at all) - real privilege
// isolation for two components that can otherwise do anything to the
// host. Both also verify every image they activate against its signed
// Sigstore/SLSA provenance via `cosign verify-attestation` before
// activation (see rootguard-core/internal/stack/attestation.go and
// rootguard-updater/attestation.go) - which genuinely needs outbound
// HTTPS to a handful of real internet hosts, something `control`'s own
// isolation otherwise makes impossible.
//
// This binary is the one, narrow, auditable bridge for exactly that:
// a hardcoded 3-host allowlist (see allowlist.go), CONNECT-tunnel only,
// nothing else. It never terminates TLS itself (a pure byte-copying
// tunnel), so it needs no CA certificates, no shell, no OS at all - it
// ships on a `scratch` base, the smallest possible RootGuard image.
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

const listenAddr = ":8888"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck())
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", listenAddr, err)
	}
	log.Printf("rootguard-attestation-proxy listening on %s", listenAddr)

	// serve only ever returns with a non-nil error (Accept failing is
	// its only exit) - staticcheck (SA4023) correctly flags the
	// equivalent `if err := ...; err != nil` form as always true.
	server := newProxyServer()
	log.Fatalf("serve: %v", server.serve(ln))
}

// runHealthcheck is invoked as `rootguard-attestation-proxy healthcheck`
// - the Dockerfile's own HEALTHCHECK instruction (exec form, no shell
// needed on `scratch`). Deliberately just a liveness check (does this
// process accept a connection and answer a plain HTTP request at all),
// not a live round-trip to any allowlisted host: a transient
// ghcr.io/Sigstore hiccup during stack startup must not flip this
// container unhealthy and block Core/Updater's own
// `depends_on: condition: service_healthy` from ever starting. The
// CONNECT/allowlist logic itself is covered by this package's own unit
// tests, not re-verified on every health-check interval.
func runHealthcheck() int {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1%s/healthz", listenAddr))
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck request failed:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck got status", resp.Status)
		return 1
	}
	return 0
}
