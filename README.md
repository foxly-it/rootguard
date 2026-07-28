# RootGuard Updater

[![CI](https://github.com/foxly-it/rootguard-updater/actions/workflows/ci.yml/badge.svg)](https://github.com/foxly-it/rootguard-updater/actions/workflows/ci.yml)
[![Container](https://img.shields.io/badge/GHCR-rootguard--updater-2496ed?logo=docker)](https://github.com/foxly-it/rootguard-updater/pkgs/container/rootguard-updater)
[![License](https://img.shields.io/github/license/foxly-it/rootguard-updater)](LICENSE)

`rootguard-updater` is the internal lifecycle helper for the RootGuard control
plane. It is deliberately separate from Core and WebApp so it stays available
while those two containers are replaced.

It is a RootGuard component, not a general-purpose Docker update service. Most
users should install it through the pinned
[RootGuard Compose](https://github.com/foxly-it/rootguard) instead of running it
by itself.

## Security boundary

The helper:

- accepts only bearer-authenticated requests from the internal control network;
- manages only the fixed Compose services `core` and `webapp`;
- obtains target images exclusively from environment configuration;
- compares actual Docker image IDs instead of tags;
- replaces and verifies Core and WebApp as a pair;
- pins both previous image IDs if either health check fails;
- persists status in the protected `rootguard-data` volume;
- exposes no host port.

It does not accept image names, container names, Compose files, or command
arguments through its HTTP API. `ROOTGUARD_UPDATER_SKIP_PULL` exists only for
local/integration tests using prebuilt images and must remain disabled for
release deployments.

The container needs the Docker socket because replacing Core and WebApp is its
single responsibility. RootGuard deliberately keeps that privilege away from
the browser-facing WebApp. The allowlist and internal bearer authentication are
release boundaries and must not be made configurable through HTTP requests.

## Configuration

| Variable | Purpose |
| --- | --- |
| `ROOTGUARD_UPDATER_TOKEN` | Required internal bearer token |
| `ROOTGUARD_UPDATER_DATA_DIR` | Persistent status, history, and image override directory |
| `ROOTGUARD_COMPOSE_FILE` | Server-controlled RootGuard Compose path |
| `ROOTGUARD_COMPOSE_PROJECT` | Fixed Compose project name |
| `ROOTGUARD_CORE_UPDATE_IMAGE` | Server-controlled immutable Core target |
| `ROOTGUARD_WEBAPP_UPDATE_IMAGE` | Server-controlled immutable WebApp target |
| `ROOTGUARD_SESSION_DIR` | Shared WebApp session directory prepared during startup |

The service listens on port `8082` inside the private control network. Only
`/health` is unauthenticated. No host port should be published.

## Development

```bash
go test ./...
go vet ./...
docker build -t rootguard-updater:test .
```

Changes to update, verification, rollback, image retention, or Docker command
construction require tests that demonstrate the allowlist and failure path.
`integration/run.sh` additionally builds real old, new, and deliberately
unhealthy fixture images. It proves that the running Core/WebApp containers are
updated as a pair and that both previous image IDs are restored when either
candidate fails its HTTP health check.
See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

RootGuard Updater is licensed under the GNU Affero General Public License v3.0
or later (AGPL-3.0-or-later). See the `LICENSE` file for full details.
