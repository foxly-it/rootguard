# RootGuard 0.1.0-alpha.5

RootGuard 0.1.0-alpha.5 is the first public evaluation release with
cryptographically verifiable Core and WebApp build provenance. It also moves
Unbound to a reproducible Debian 13 source build, makes the Stack Center more
trustworthy, and adds an explicit Stable/Beta choice for AdGuard Home.

This remains an evaluation release and is not recommended as the only
production DNS path for a network.

## Install

```sh
mkdir rootguard-alpha && cd rootguard-alpha
curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.5/compose.alpha.yaml
curl -Lo .env https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.5/.env.alpha.example
```

Replace every placeholder in `.env`, generate the internal API and recovery
tokens independently, and start the stack:

```sh
openssl rand -hex 32
openssl rand -hex 32
docker compose -f compose.alpha.yaml up -d
```

Open `http://<docker-host>:8080/login` and complete the guided setup.

## New in alpha.5

- Signed SLSA v1 provenance for Core and WebApp release images. RootGuard pins
  Cosign 3.0.6 by digest and verifies the image digest, expected repository and
  workflow identity, GitHub Actions OIDC issuer, certificate chain, claims,
  and Sigstore transparency-log inclusion.
- A bilingual Stack Center provenance status that distinguishes verified,
  missing, invalid, temporarily unavailable, and non-applicable results.
- Reproducible Unbound 1.25.2 builds from a checksum-verified upstream source
  on Debian 13 Slim for native `amd64` and `arm64`, retaining the stable
  non-root `100:101` volume identity.
- Richer five-service runtime visibility: immutable image references, OCI
  version/revision/source metadata, health, start time, restart count, ports,
  and bounded redacted logs.
- A guided AdGuard Home Stable/Beta selection. Stable remains the default;
  Beta carries an explicit warning that DNS resolution and blocking may fail.
- The official Beta image is allowlisted in Core and pinned to a verified
  multi-architecture digest in this release stack.

## Provenance interpretation

Only immutable, digest-pinned RootGuard Core and WebApp release images are
eligible for RootGuard's release trust policy. Local development builds,
mutable tags, and third-party components show **Not applicable**. A temporary
registry or Sigstore outage is shown separately and is not reported as
confirmed tampering.

The release can also be checked independently:

```sh
gh attestation verify \
  oci://ghcr.io/foxly-it/rootguard-core:0.1.0-alpha.5 \
  --repo foxly-it/rootguard \
  --signer-workflow foxly-it/rootguard/.github/workflows/release-alpha.yml

gh attestation verify \
  oci://ghcr.io/foxly-it/rootguard-webapp:0.1.0-alpha.5 \
  --repo foxly-it/rootguard-webapp \
  --signer-workflow foxly-it/rootguard-webapp/.github/workflows/build.yml
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

## Upgrade from alpha.4

Keep the existing `.env` and named volumes. Download the Alpha 5 Compose file
next to the existing installation and retain a copy of the previous file:

```sh
cp compose.alpha.yaml compose.alpha.4.yaml
curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v0.1.0-alpha.5/compose.alpha.yaml
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
