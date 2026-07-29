# RootGuard 0.1.0-alpha.3

RootGuard 0.1.0-alpha.3 is a visibility and interface-quality update for the
public evaluation stack. It adds privacy-preserving live dashboard metrics,
improves the readability of logs and configuration details, and makes guided
Unbound actions more consistent.

This remains an evaluation release and is not recommended as the only
production DNS path for a network.

## Install

```sh
mkdir rootguard-alpha && cd rootguard-alpha
curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.3/compose.alpha.yaml
curl -Lo .env https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.3/.env.alpha.example
```

Replace all placeholder values in `.env`. Generate the internal API token and
recovery token independently:

```sh
openssl rand -hex 32
openssl rand -hex 32
docker compose -f compose.alpha.yaml up -d
```

Open `http://<docker-host>:8080/login`. The guided Setup validates the selected
host address and port before creating and securing AdGuard Home and Unbound.

## New in alpha.3

- Live aggregate CPU and memory usage for the five allowlisted RootGuard
  containers.
- Aggregate AdGuard Home query, blocked-query, and filter-rate metrics without
  exposing query names or client identities to RootGuard.
- Automatic dashboard and Stack Center refresh to prevent stale post-update
  service states.
- Large, scrollable modal views for protected service logs, live Unbound
  configuration, and supported-directive details.
- Consistent action buttons across private domains, DNS target servers,
  forwarding zones, and the Private DNS draft workflow.
- Correct Advisor profile updates when rapidly switching operating profiles.
- Improved responsive website feature cards and an updated public dashboard
  preview and documentation.

## Included stack

- Versioned `amd64` and `arm64` images for Core, WebApp, Updater, and Unbound.
- Authenticated German/English WebGUI and guided AIO installation.
- Private AdGuard Home administration through RootGuard.
- Recursive Unbound resolution with DNSSEC validation.
- Guided resolver settings, local records, conditional forwarding, expert
  configuration, validation, history, and rollback.
- Allowlisted data-plane and paired control-plane update foundations.

## Known limitations

- Keep backups and an alternative DNS path available.
- There is no built-in HTTPS. Do not expose the WebGUI directly to the internet.
- Complete backup export/import and disaster recovery are not available yet.
- Core and Updater require the Docker socket and belong inside the trusted host
  boundary.
- AdGuard filter, client, and query-log management remain in the protected
  native AdGuard Home interface.
- Images are versioned and digest-pinned but not yet signed. SBOM, provenance,
  and signature verification remain beta release-engineering gates.
- Migration between arbitrary development snapshots is unsupported.

## Upgrade from alpha.2

Keep the existing `.env` and named volumes. Download the alpha.3 Compose file
next to the existing installation and retain a copy of the previous
alpha.2 files:

```sh
cp compose.alpha.yaml compose.alpha.2.yaml
curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.3/compose.alpha.yaml
docker compose -f compose.alpha.yaml pull
docker compose -f compose.alpha.yaml up -d
```

Do not use `docker compose down --volumes`; named volumes hold installation
state, sessions, the password verifier, Unbound configuration, and AdGuard data.

## Report problems

Include the RootGuard version, host architecture, Docker and Compose versions,
the affected workflow, and redacted service logs. Never publish `.env`, session
files, API tokens, recovery tokens, passwords, query names, or client details.
