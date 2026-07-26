# RootGuard 0.1.0-alpha.1

RootGuard 0.1.0-alpha.1 is the first public evaluation release that starts
without cloning or building the component repositories. It is intended for
testing the complete product path, not for production use.

## Install

```sh
mkdir rootguard-alpha && cd rootguard-alpha
curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.1/compose.alpha.yaml
curl -Lo .env https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.1/.env.alpha.example
```

Replace `ROOTGUARD_API_TOKEN` and `ROOTGUARD_ADMIN_PASSWORD` in `.env`, then:

```sh
docker compose -f compose.alpha.yaml up -d
```

Open `http://<docker-host>:8080/login`. The guided Setup creates and secures
AdGuard Home and Unbound after validating the selected host address and port.

## Included

- Versioned `amd64` and `arm64` images for Core, WebApp, Updater, and Unbound.
- Authenticated German/English WebGUI and guided AIO installation.
- Private AdGuard administration through RootGuard.
- Recursive Unbound resolution with DNSSEC validation.
- Guided resolver settings, profiles, local records, conditional forwarding,
  expert configuration, validation, history, and rollback.
- Allowlisted data-plane and paired control-plane update foundations.

## Known limitations

- Keep backups and an alternative DNS path available.
- There is no built-in HTTPS. Do not expose the WebGUI directly to the internet.
- Complete backup export/import and disaster recovery are not available yet.
- Core and Updater require the Docker socket and belong inside the trusted host
  boundary.
- Filter, client, query-log, and statistics management are not first-class
  RootGuard workflows yet.
- Images are versioned but not yet signed. Digest pinning, SBOM, provenance,
  and signature verification remain release-engineering gates.
- Migration between arbitrary development snapshots is unsupported.

## Operate and stop

```sh
docker compose -f compose.alpha.yaml ps
docker compose -f compose.alpha.yaml logs --tail=200 core webapp updater
docker compose -f compose.alpha.yaml stop
docker compose -f compose.alpha.yaml start
```

Named volumes hold installation state, sessions, Unbound configuration, and
AdGuard data. `docker compose down --volumes` is not a normal stop command and
does not remove every DNS volume created by Setup. Follow the clean-install
instructions in the matching manual before deleting data.

## Report problems

Include the RootGuard version, host architecture, Docker and Compose versions,
the failed setup step, and redacted service logs. Never publish `.env`, session
files, generated AdGuard credentials, API tokens, or administrator passwords.
