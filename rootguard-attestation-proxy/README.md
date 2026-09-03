# RootGuard Attestation Proxy

[![License](https://img.shields.io/github/license/foxly-it/rootguard)](LICENSE)

`rootguard-attestation-proxy` is a minimal, purpose-built CONNECT-only
forward proxy. It exists for exactly one reason: Core and the Updater
both hold the Docker socket and run only on RootGuard's `control` Docker
network, which is deliberately `internal: true` (no internet route at
all) - real privilege isolation for two components that can otherwise do
anything to the host. Both also verify every image they activate
against its signed Sigstore/SLSA provenance via `cosign
verify-attestation` before activation, which genuinely needs outbound
HTTPS to a handful of real internet hosts - something `control`'s own
isolation otherwise makes impossible.

This is the one, narrow, auditable bridge for exactly that call, and
nothing else. It is a RootGuard component, not a general-purpose
proxy - most users should install it through the pinned
[RootGuard Compose](https://github.com/foxly-it/rootguard) rather than
running it standalone.

## Security boundary

- **CONNECT-only.** No plain HTTP proxying, no TLS termination - the
  proxy is content-blind to everything past the CONNECT line itself.
  This isn't a MITM proxy.
- **Hardcoded, 3-host allowlist**, port 443 only: `ghcr.io` (registry
  API), `pkg-containers.githubusercontent.com` (GHCR blob storage),
  `tuf-repo-cdn.sigstore.dev` (Sigstore trust-root bootstrap) - the
  exact, empirically-confirmed set a real `cosign verify-attestation`
  call needs, no more. See `allowlist.go`'s own comment for how that set
  was derived.
- **Not an authentication boundary.** Every process that can reach this
  proxy (Core, the Updater) already holds the Docker socket and runs as
  root - i.e. already has full host privilege. The allowlist is
  defense-in-depth (keeps `control` itself provably internet-isolated
  except for this one narrow path), not a way to distinguish a more- vs.
  less-trusted caller.
- **The smallest possible RootGuard image.** Runtime base is `scratch` -
  zero CA certificates, zero shell, zero OS - because this binary never
  terminates TLS itself, it needs none of that. Runs as a fixed non-root
  numeric UID. No Docker socket, no host mounts, no privileges beyond
  binding one port.
- **Health check never depends on live third-party reachability.** A
  transient ghcr.io/Sigstore hiccup during stack startup must not flip
  this container unhealthy and block Core/the Updater's own
  `depends_on: condition: service_healthy` from ever starting - see
  `main.go`'s `runHealthcheck` doc comment.

## Configuration

None. The allowlist is compiled in, not configurable via environment or
flags - widening it requires a code change and a new release, by
design. The service listens on port `8888`; only `GET /healthz` (used
by the container `HEALTHCHECK`) and `CONNECT` are served.

## Development

```bash
go test ./...
go vet ./...
docker build -t rootguard-attestation-proxy:test .
```

Tests never depend on live network access - the allowed-CONNECT/tunnel
path is exercised against a local listener via `dialUpstream`, an
injection point matching the same pattern
`rootguard-core/internal/stack/attestation.go` already uses for its own
cosign invocation. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

RootGuard Attestation Proxy is licensed under the GNU Affero General
Public License v3.0 or later (AGPL-3.0-or-later). See the `LICENSE`
file for full details.
