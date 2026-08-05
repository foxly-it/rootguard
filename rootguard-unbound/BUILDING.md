# Build integrity

RootGuard Unbound is compiled from a pinned upstream release rather than an
operating-system Unbound package. The Dockerfile is the source of truth for all
build inputs.

## Pinned inputs

| Input | Pin |
| --- | --- |
| Unbound | `1.25.2` |
| Source archive | `https://nlnetlabs.nl/downloads/unbound/unbound-1.25.2.tar.gz` |
| Source SHA-256 | `0d92275c703d5f5f8baba3dab22117dd8c29b495588a5c229768ed6581566600` |
| Base | `debian:13-slim` |
| Multi-architecture base digest | `sha256:020c0d20b9880058cbe785a9db107156c3c75c2ac944a6aa7ab59f2add76a7bd` |

Direct build and runtime packages are pinned to exact Debian 13 versions in
the Dockerfile. APT verifies package hashes through Debian's signed repository
metadata. The resulting image records the upstream URL, source checksum, base
digest, and compiled version as OCI labels.

## Local verification

```sh
docker build --pull=false -t rootguard-unbound:test .
docker run --rm --entrypoint unbound rootguard-unbound:test -V
docker run --rm rootguard-unbound:test \
  unbound-checkconf /etc/unbound/unbound.conf
docker image inspect rootguard-unbound:test --format '{{json .Config.Labels}}'
```

The CI smoke test additionally verifies recursive DNS, DNSSEC failure handling,
the non-root identity, trust-anchor volume compatibility, and both `amd64` and
`arm64` builds. Published images include BuildKit SBOM and provenance
attestations.

## Updating Unbound or Debian inputs

1. Obtain the current release and checksum from the official NLnet Labs download
   page and verify the checksum file over HTTPS.
2. Update the version, archive SHA-256, base digest, and exact package versions
   in the Dockerfile.
3. Update the expected version and metadata in the workflow and this document.
4. Run all local checks above and let the pull-request workflow validate both
   target architectures before merging.

Security updates are intentional, reviewed input changes. The daily scheduled
workflow detects reproducibility or repository-availability regressions; it does
not silently replace pinned dependencies.
