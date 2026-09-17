// rootguard-docker-proxy is a purpose-built, filtering Docker Engine API
// proxy. Core and the Updater both need the Docker socket - Core to run
// the guided-setup DNS stack, drive updates/rollback, and export/restore
// backups; the Updater to swap Core/WebApp images and clean up after a
// successful update. Mounting `/var/run/docker.sock` directly into either
// container, as RootGuard did through 1.0.0, means "whoever controls this
// socket controls the host" (see docs/threat-model.md, actor 1): a bug in
// either component that lets an attacker issue their own Docker API calls
// leads straight to a privileged container with an arbitrary host bind
// mount, i.e. full host compromise, with no second line of defense.
//
// A generic allowlist-by-endpoint proxy (e.g. tecnativa/docker-socket-proxy)
// does not close that gap: Core legitimately needs `POST /containers/create`
// (its own `docker run` helpers, and everything `docker compose up` does
// under the hood) and `POST /containers/{id}/exec` (to reload the
// blockpage) - toggling those endpoints on at all would still let a
// compromised Core request `Privileged: true` or an arbitrary host bind
// mount through them. This proxy instead allow-lists by exact
// method+path AND inspects the request body of every endpoint that
// creates new capability (container create, exec create, network
// connect), rejecting anything outside what Core/the Updater's own real
// code actually sends. See allowlist.go/validate.go for the exact rules
// and rootguard-docker-proxy/README.md for the full security boundary.
//
// This binary only holds the real Docker socket - Core and the Updater
// talk to it instead, over the internal `control` network, via a plain
// `DOCKER_HOST=tcp://docker-proxy:2375` (no call-site code changes needed;
// the `docker` CLI/compose already honor DOCKER_HOST transparently).
// Dropping `USER root` from their own Dockerfiles now that neither holds
// the real socket anymore is a deliberate, separate follow-up (needs its
// own volume-ownership migration design first, tracked in ROADMAP.md) -
// wiring this proxy in doesn't require it.
//
// Wired into compose.release.yaml: Core and the Updater no longer mount
// the socket themselves, only this service does (see this repository's
// docs/threat-model.md, actor 1, and rootguard-docker-proxy/README.md for
// the current rollout state, including the backward-compatibility
// behavior for installations whose compose topology predates this
// service).
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

const listenAddr = ":2375"

const defaultSocketPath = "/var/run/docker.sock"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck())
	}

	socketPath := os.Getenv("ROOTGUARD_DOCKER_PROXY_SOCKET")
	if socketPath == "" {
		socketPath = defaultSocketPath
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", listenAddr, err)
	}
	log.Printf("rootguard-docker-proxy listening on %s, upstream socket %s", listenAddr, socketPath)

	server := &http.Server{
		Handler:      newDockerProxy(socketPath),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // image pulls can legitimately take a while
		IdleTimeout:  60 * time.Second,
	}
	log.Fatalf("serve: %v", server.Serve(ln))
}

// runHealthcheck is invoked as `rootguard-docker-proxy healthcheck` - the
// Dockerfile's own HEALTHCHECK instruction. Deliberately just a liveness
// check (does this process accept a connection and answer at all), not a
// live round-trip to the real Docker socket: a slow/busy dockerd during
// heavy image-pull traffic must not flip this container unhealthy and
// block Core/the Updater's own `depends_on: condition: service_healthy`.
// The allowlist/validation logic itself is covered by this package's own
// unit tests, not re-verified on every health-check interval - same
// pattern as rootguard-attestation-proxy's runHealthcheck.
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
