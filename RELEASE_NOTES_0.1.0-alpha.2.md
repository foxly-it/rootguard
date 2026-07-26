# RootGuard 0.1.0-alpha.2

RootGuard 0.1.0-alpha.2 is a security and usability follow-up to the first
public evaluation release. It adds a complete local password-recovery path
while preserving the single-Compose installation and the protected
AdGuard Home → Unbound DNS architecture.

This remains an evaluation release and is not recommended as the only
production DNS path for a network.

## Install

```sh
mkdir rootguard-alpha && cd rootguard-alpha
curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.2/compose.alpha.yaml
curl -Lo .env https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.2/.env.alpha.example
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

## New in alpha.2

- Bilingual **Forgot password?** flow on the RootGuard login screen.
- Separate installation recovery key that cannot create a session or access
  the internal Core API.
- New administrator passwords are stored only as salted PBKDF2-SHA256
  verifiers with 600,000 iterations.
- Every active session is invalidated after a successful password reset.
- Reset credentials persist across controlled WebApp restarts and updates in
  the restricted session volume.
- Explicit local `.env` recovery instructions when no recovery key is
  configured.
- End-to-end CI verifies reset, old-session rejection, WebApp restart, and
  login with the recovered password.

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

Named volumes hold installation state, sessions, the password verifier,
Unbound configuration, and AdGuard data. `docker compose down --volumes` is not
a normal stop command.

## Report problems

Include the RootGuard version, host architecture, Docker and Compose versions,
the failed setup or recovery step, and redacted service logs. Never publish
`.env`, session files, API tokens, recovery tokens, or passwords.
