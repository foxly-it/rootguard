# RootGuard 0.1.0-alpha.6

RootGuard 0.1.0-alpha.6 focuses on the WebGUI: a semantic design-token theme
system with System/Light/Dark modes, a reworked header with global search, an
accessible sidebar, and a fix for washed-out shadows in light mode. It also
moves RootGuard's own source to a monorepo, which does not change anything
for users but simplifies how the project is built and released.

This remains an evaluation release and is not recommended as the only
production DNS path for a network.

## Install

```sh
mkdir rootguard-alpha && cd rootguard-alpha
curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.6/compose.alpha.yaml
curl -Lo .env https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.6/.env.alpha.example
```

Replace every placeholder in `.env`, generate the internal API and recovery
tokens independently, and start the stack:

```sh
openssl rand -hex 32
openssl rand -hex 32
docker compose -f compose.alpha.yaml up -d
```

Open `http://<docker-host>:8080/login` and complete the guided setup.

## New in alpha.6

- Semantic design tokens and a persistent System/Light/Dark theme, covering
  the app shell, login, Dashboard, Setup, Stack Center, AdGuard, and Unbound
  settings. Code/config viewers (expert editor, live-config, logs) stay dark
  by design, like a code block.
- A reworked header: a coherent utility bar, and language/appearance/sign-out
  consolidated into one accessible user menu.
- Global, local-only search (`S` or `Ctrl`/`Cmd`+`K`) covering every page,
  Unbound's tabs, guided settings, and technical directive names, with full
  keyboard navigation.
- Sidebar collapse control moved to its bottom edge, default new desktop
  sessions to the collapsed icon view, and kept visible while long pages
  scroll.
- Fixed washed-out light mode: every `box-shadow` was tuned for the dark
  theme's near-black background and read as a muddy grey halo once reused
  against light surfaces; shadows are now theme-aware.
- Unbound's expert configuration editor gained a fullscreen mode and an
  inline, collapsed-by-default view of the immutable base configuration's
  already-active directives, so experts no longer need a separate popup.
- Fixed the Dashboard services KPI showing a stale `5 / 2` instead of the
  live `5 / 5` count.
- Removed distracting decorative hero circles from the Dashboard, Setup,
  Stack Center, and Unbound pages.
- Documented a tested rootless Podman path alongside Docker.
- RootGuard's source moved to a single monorepo (`rootguard-core/`,
  `rootguard-webapp/`, `rootguard-unbound/`, `rootguard-updater/` are now
  directories here, full history preserved). The four previously separate
  repositories are archived, read-only, with a pointer back here.

## Provenance interpretation

Only immutable, digest-pinned RootGuard Core, WebApp, Unbound, and Updater
release images are eligible for RootGuard's release trust policy. Local
development builds, mutable tags, and third-party components show
**Not applicable**. A temporary registry or Sigstore outage is shown
separately and is not reported as confirmed tampering.

The release can also be checked independently - all four images are now
signed by the same monorepo workflow:

```sh
gh attestation verify \
  oci://ghcr.io/foxly-it/rootguard-core:0.1.0-alpha.6 \
  --repo foxly-it/rootguard \
  --signer-workflow foxly-it/rootguard/.github/workflows/release-alpha.yml

gh attestation verify \
  oci://ghcr.io/foxly-it/rootguard-webapp:0.1.0-alpha.6 \
  --repo foxly-it/rootguard \
  --signer-workflow foxly-it/rootguard/.github/workflows/release-alpha.yml
```

## Known limitations

- Keep backups and an alternative DNS path available.
- There is no built-in HTTPS. Do not expose the WebGUI directly to the internet.
- Complete backup export/import and disaster recovery are not available yet.
- Core and Updater require the Docker socket and belong inside the trusted host
  boundary.
- AdGuard filter, client, and query-log management remain in the protected
  native AdGuard Home interface.
- Migration between arbitrary development snapshots is unsupported.

## Upgrade from alpha.5

Keep the existing `.env` and named volumes. Download the Alpha 6 Compose file
next to the existing installation and retain a copy of the previous file:

```sh
cp compose.alpha.yaml compose.alpha.5.yaml
curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.6/compose.alpha.yaml
docker compose -f compose.alpha.yaml pull
docker compose -f compose.alpha.yaml up -d
```

Do not use `docker compose down --volumes`; named volumes contain installation
state, sessions, the password verifier, Unbound configuration, and AdGuard
data.

## Report problems

Include the RootGuard version, host architecture, Docker and Compose versions,
the affected workflow, the Stack Center provenance status, and redacted logs.
Never publish `.env`, session files, tokens, recovery keys, passwords, query
names, or client details.
