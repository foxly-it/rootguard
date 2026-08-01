# RootGuard 0.1.0-alpha.4

RootGuard 0.1.0-alpha.4 strengthens resolver updates, expands guided Unbound
controls, and makes AdGuard Home filtering visible through privacy-preserving
local diagnostics. It also applies a safer DNS baseline during guided setup.

This remains an evaluation release and is not recommended as the only
production DNS path for a network.

## Install

```sh
mkdir rootguard-alpha && cd rootguard-alpha
curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.4/compose.alpha.yaml
curl -Lo .env https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.4/.env.alpha.example
```

Replace all placeholder values in `.env`, then generate the internal API token
and recovery token independently:

```sh
openssl rand -hex 32
openssl rand -hex 32
docker compose -f compose.alpha.yaml up -d
```

Open `http://<docker-host>:8080/login`. Guided setup validates the selected
host address and port before creating and securing AdGuard Home and Unbound.

## New in alpha.4

- Unbound 1.25.2 with a stable non-root `100:101` runtime identity and a safe
  ownership migration for existing configuration and RFC5011 trust-anchor data.
- System-managed Unbound socket buffers avoid repeated `so-sndbuf` warnings on
  hosts that do not grant the requested kernel buffer size.
- Guided controls for serve-expired behavior, DNSSEC caching, EDNS buffer size,
  resolver resource profiles, and bounded temporary diagnostic logging.
- Privacy-oriented AdGuard Home defaults: exclusive Unbound upstream, no public
  fallback, filtering and protection enabled, DNSSEC DO enabled, ECS disabled,
  refused ANY queries, bounded rate/cache/TTL values, and daily filter updates.
- A local filter diagnostic that checks representative ad and tracker hostnames
  through AdGuard Home's API without opening external test websites or storing
  client and query-log details in RootGuard.
- Collapsible symbol-only WebGUI navigation with tooltips, plus matching Lucide
  metric symbols in the public website preview.

## Filter diagnostic interpretation

The report shows what the currently installed AdGuard filter lists actually
block. The default list may intentionally leave some representative hostnames
unblocked; this is reported as real coverage rather than treated as proof that
AdGuard Home is broken. Operators can adjust filter lists in AdGuard Home and
run the diagnostic again.

## Included stack

- Digest-pinned `amd64` and `arm64` images for Core, WebApp, Updater, and
  Unbound.
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
- Images are versioned and digest-pinned but not yet signed. Complete SBOM,
  provenance, and signature verification remain beta release-engineering gates.
- The Unbound 1.25.2 image still installs the Debian Forky/Sid package. A
  reproducible checksum-pinned source build on Debian 13 Slim is the next
  roadmap slice.
- Migration between arbitrary development snapshots is unsupported.

## Upgrade from alpha.3

Keep the existing `.env` and named volumes. Download the alpha.4 Compose file
next to the existing installation and retain a copy of the previous file:

```sh
cp compose.alpha.yaml compose.alpha.3.yaml
curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.4/compose.alpha.yaml
docker compose -f compose.alpha.yaml pull
docker compose -f compose.alpha.yaml up -d
```

Do not use `docker compose down --volumes`; named volumes hold installation
state, sessions, the password verifier, Unbound configuration, and AdGuard data.

## Report problems

Include the RootGuard version, host architecture, Docker and Compose versions,
the affected workflow, and redacted service logs. Never publish `.env`, session
files, API tokens, recovery tokens, passwords, query names, or client details.
