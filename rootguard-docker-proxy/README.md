# RootGuard Docker Proxy

[![License](https://img.shields.io/github/license/foxly-it/rootguard)](LICENSE)

`rootguard-docker-proxy` is a purpose-built, filtering Docker Engine API
proxy. `rootguard-core` and `rootguard-updater` both need the Docker
socket - Core to run the guided-setup DNS stack, drive updates/rollback,
and export/restore backups; the Updater to swap Core/WebApp images and
clean up after a successful update. Through 1.0.0, both mounted
`/var/run/docker.sock` directly: whoever controls that socket effectively
controls the host (`docs/threat-model.md`, actor 1) - a bug in either
component that lets an attacker issue their own Docker API calls leads
straight to a privileged container with an arbitrary host bind mount,
i.e. full host compromise, with no second line of defense.

This binary is that second line of defense: the only container that
holds the real socket, reachable only from `control` (RootGuard's
internet-isolated internal network), speaking an allow-listed, validated
subset of the Docker Engine API back to Core and the Updater.

**Status: standalone, not yet wired into the RootGuard stack.** This
component's own allowlist and validators are complete and unit-tested,
but `compose.release.yaml`, Core/Updater's Dockerfiles, and a
`ROOTGUARD_DOCKER_PROXY_URL` preflight check are a deliberate follow-up
change - see this repository's `docs/threat-model.md` and
`docs/release-process.md` for the rollout plan once that lands.

## Why not a generic allowlist-by-endpoint proxy?

Tools like `tecnativa/docker-socket-proxy` allow-list by *resource type*
(containers/images/networks on or off), not by *request body*. Core
legitimately needs `POST /containers/create` (its own `docker run`
helpers, and everything `docker compose up` does under the hood) and
`POST /containers/{id}/exec` (to reload the blockpage) - toggling those
endpoints on at all would still let a compromised Core request
`Privileged: true` or an arbitrary host bind mount through them. This
proxy inspects the body of every call that can grant new capability and
rejects anything outside what Core/the Updater's own real code actually
sends - see `validate.go`.

## Security boundary

- **Allow-listed by exact method + path** (`allowlist.go`) - every entry
  traces back to an actual `docker`/`docker compose` subcommand this
  repo's own code issues today (`rootguard-core/internal/dockercli`,
  `rootguard-updater/docker.go`), not a guess at what the Docker API
  offers in general. No Swarm, Plugins, Secrets, Services, Nodes, Build,
  Commit, Session, or Configs endpoints exist in the allowlist at all.
- **Body-validated for the three capability-granting calls**
  (`validate.go`):
  - `POST /containers/create` - rejects `Privileged`, `NetworkMode`/
    `PidMode`/`IpcMode`/`UTSMode: host`, any `Devices`, any `CapAdd`
    outside `{CHOWN, SETUID, SETGID}`, and any bind/mount whose source
    isn't one of RootGuard's own named volumes (an arbitrary host path is
    exactly the primitive this proxy exists to close off) or whose
    `Image` isn't one of RootGuard's own known image repositories.
  - `POST /containers/{id}/exec` - only `rootguard-blockpage`, only the
    two literal commands Core's own code ever sends.
  - `POST /networks/{id}/connect` - only `rootguard-dns`, only
    RootGuard's own containers.
- **Not a caller-authentication boundary.** Core and the Updater both sit
  on the same `control` network and aren't distinguished from each other
  at the proxy level - this is a known, documented scope limit, not an
  oversight. The allowlist used is Core's (a strict superset of the
  Updater's own, narrower needs).
- **Still needs a root-capable process somewhere.** Socket access
  requires either root or matching the host's docker-group GID, which
  differs per installation - see the comment above `EXPOSE` in the
  `Dockerfile`. This component doesn't eliminate that; it concentrates it
  into one small, single-purpose, exhaustively unit-tested request
  filter instead of two large, feature-rich services (Core's whole
  WebGUI-driven API, the Updater's update/rollback logic).
- **Health check never depends on the real Docker socket.** A slow/busy
  dockerd during heavy image-pull traffic must not flip this container
  unhealthy and block Core/the Updater's own
  `depends_on: condition: service_healthy` - see `main.go`'s
  `runHealthcheck` doc comment.

## Configuration

- `ROOTGUARD_DOCKER_PROXY_SOCKET` - path to the real Docker socket
  (default `/var/run/docker.sock`).

The allowlist and validators themselves are compiled in, not configurable
via environment or flags - widening them requires a code change and a
new release, by design, same philosophy as `rootguard-attestation-proxy`.
The service listens on port `2375`.

## Development

```bash
go test ./...
go vet ./...
docker build -t rootguard-docker-proxy:test .
```

Tests never depend on a real Docker socket - every allowed/rejected case
is exercised against a fake in-process HTTP server via `dialUpstream`, an
injection point matching the same pattern
`rootguard-attestation-proxy/proxy.go` already uses. See
[CONTRIBUTING.md](CONTRIBUTING.md).

## License

RootGuard Docker Proxy is licensed under the GNU Affero General Public
License v3.0 or later (AGPL-3.0-or-later). See the `LICENSE` file for
full details.
