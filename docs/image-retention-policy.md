# RootGuard image integrity and retention

Last reviewed: 2026-08-01

RootGuard release Compose files use a readable version tag together with the
verified multi-architecture manifest digest:

```text
ghcr.io/foxly-it/rootguard-core:0.1.0-alpha.2@sha256:…
```

Docker resolves the digest, not the mutable tag. The tag remains visible only
to make the selected release understandable to an operator.

## Recorded 0.1.0-alpha.2 manifests

| Component | Release | Multi-architecture manifest |
| --- | --- | --- |
| Core | `0.1.0-alpha.2` | `sha256:cfdc7aee3ba06bbd598986dbc83e521a37cee5879c04f537734aaff574c15e10` |
| WebApp | `0.1.0-alpha.2` | `sha256:62242437eee6fcead12748080fc86205a45747fa0b479f6c4a901d603f74c169` |
| Updater | `0.1.0-alpha.2` | `sha256:1e08cdcb4868e99389db586a43130e08d73045aeac67d7e724c9cf669ebbe45a` |
| Unbound | `0.1.0-alpha.2` | `sha256:93b705586814042469547677b79648ff7bc09de45efea36553402d3c3fb9026d` |

## Recorded 0.1.0-alpha.3 manifests

| Component | Version tag | Multi-architecture manifest digest |
| --- | --- | --- |
| Core | `0.1.0-alpha.3` | `sha256:c22f918bb563740cb9b99cf09a438fc1801eabaf1df0dd3eb25295825d6ca321` |
| WebApp | `0.1.0-alpha.3` | `sha256:5a9baa7c0819cdaa5db9da9248e03b50038b4875aa43c71f52081c123219fd8d` |
| Updater | `0.1.0-alpha.3` | `sha256:1e08cdcb4868e99389db586a43130e08d73045aeac67d7e724c9cf669ebbe45a` |
| Unbound | `0.1.0-alpha.3` | `sha256:f74fc78b223d8aa8492aed4562279d510e328aaf8133ee9ea6ac9fe4a40e49e3` |
| AdGuard Home | `v0.107.78` | `sha256:1ea34eafe5dc691007946e8eaab7bf46b0de9412f39213d8c06e48b53bf9a6c5` |

The Updater source did not change between alpha.2 and alpha.3. Its tested
multi-architecture manifest is therefore intentionally reused under the new
coordinated stack version.

## Recorded 0.1.0-alpha.4 manifests

| Component | Version tag | Multi-architecture manifest digest |
| --- | --- | --- |
| Core | `0.1.0-alpha.4` | `sha256:b8054daa075644364fd2367b8cbf7d2c3a2c78040079031c7a9e70558371b1ca` |
| WebApp | `0.1.0-alpha.4` | `sha256:19aa457d2ad0c63f610c18a78c26193b48fd71cdcc5340c3644d85be1afe779a` |
| Updater | `0.1.0-alpha.4` | `sha256:849ff328b83032a405ce42bb19ee811d1552bd3502c81ca8194b7fefc28f0723` |
| Unbound | `0.1.0-alpha.4` | `sha256:82ab9ff2356188e9ede37d6931039e726fb566fb3b6c5053e2d2fd3f48dcddf9` |
| AdGuard Home | `v0.107.78` | `sha256:1ea34eafe5dc691007946e8eaab7bf46b0de9412f39213d8c06e48b53bf9a6c5` |
| AdGuard Home Beta | `beta` (verified 2026-08-01) | `sha256:2b77703b27730d5c0c7045fcd6c98834169cd5c69af5f86a43947425f2d367fd` |

The Updater release includes dependency-workflow maintenance. The Unbound
release keeps the stable non-root `100:101` identity so existing named volumes
remain writable after upgrading.

## Recorded 0.1.0-alpha.6 manifests

| Component | Version tag | Multi-architecture manifest digest |
| --- | --- | --- |
| Core | `0.1.0-alpha.6` | `sha256:fedcb900d5d4f56ee7833555ce064b5773bb10e27e726b0f14b4cf3f5ca65790` |
| WebApp | `0.1.0-alpha.6` | `sha256:b156427f404da1d7ca71bc6c7621a31d36a1eaf0379b41926d59803ac9204d45` |
| Updater | `0.1.0-alpha.6` | `sha256:8fa7bf11aa044c2e5586d9811eca7c1355998f6044d685de2c3ab398e0cf185b` |
| Unbound | `0.1.0-alpha.6` | `sha256:4e5a8fa999774726cdc3fba88cc996d27514749d14e80de71b8b48c7349d021e` |
| AdGuard Home | `v0.107.78` | `sha256:1ea34eafe5dc691007946e8eaab7bf46b0de9412f39213d8c06e48b53bf9a6c5` |
| AdGuard Home Beta | `beta` (verified 2026-08-01) | `sha256:2b77703b27730d5c0c7045fcd6c98834169cd5c69af5f86a43947425f2d367fd` |

All four RootGuard images are now built and pushed from the `rootguard`
monorepo's `release-alpha.yml` (previously Core only; WebApp, Unbound, and
Updater required separately tagged pushes to their own repositories).

## Recorded 0.1.0-alpha.5 manifests

| Component | Version tag | Multi-architecture manifest digest |
| --- | --- | --- |
| Core | `0.1.0-alpha.5` | `sha256:1f78eade2093c9a89e97aeaeb360b4da03b56edf63e8b4f925453d44b8ab8e00` |
| WebApp | `0.1.0-alpha.5` | `sha256:90c76e40d42b1b01811a94f1ef5cde11b85e3ad1946104b9cce80c27577a80f0` |
| Updater | `0.1.0-alpha.5` | `sha256:80e1965674efe2812f5c65173080d4e871964f7953dc7044aac4e234d64b2d7c` |
| Unbound | `0.1.0-alpha.5` | `sha256:7c7fa1404fb1c0f42c9c9151e9254635434032e259c44eb2acd01fcf3fada789` |
| AdGuard Home | `v0.107.78` | `sha256:1ea34eafe5dc691007946e8eaab7bf46b0de9412f39213d8c06e48b53bf9a6c5` |
| AdGuard Home Beta | `beta` (verified 2026-08-01) | `sha256:2b77703b27730d5c0c7045fcd6c98834169cd5c69af5f86a43947425f2d367fd` |

The Core and WebApp release manifests have GitHub-issued SLSA provenance
attestations. The Unbound image is built reproducibly from the checksum-pinned
Unbound 1.25.2 source on Debian 13 Slim. The Updater passed its paired real
update and rollback integration scenarios before publication.

These are OCI index digests covering the supported platform manifests. They
were verified directly against GHCR and Docker Hub before being recorded.

## Update and retention rules

1. A release changes image references only as one reviewed and tested set.
2. Installation and initial update targets use the same immutable references.
3. RootGuard retains the active and previous successful image for rollback.
4. After a verified update, only older image IDs already recorded in RootGuard's
   own successful update history may become cleanup candidates.
5. An image used by any container is never removed.
6. RootGuard never runs `docker system prune`, `docker image prune`, or a global
   volume prune.
7. A volume is eligible only when unused and explicitly labeled
   `io.rootguard.cleanup=true`. Configuration, DNS data, state, sessions, and
   backups remain protected.
8. The manual preview uses Docker's unique image-layer and volume usage values
   as reclaimed-space estimates, shows skipped resources separately, and never
   accepts a resource identifier from the browser.
9. A confirmed manual cleanup recomputes the complete allowlist immediately
   before removal and records its result in the bounded update history.

Development Compose files may continue to use local or mutable image references.
They are not release artifacts and are deliberately kept separate from
`compose.alpha.yaml`.

## Preparing the next release

Build and test all component images first. Resolve the published
multi-architecture digest for each version tag, update `compose.alpha.yaml` and
`.env.alpha.example` together, and run the clean installation smoke test. The
integration workflow rejects a release Compose model if a required image lacks
an `@sha256:` digest or if a service image uses `latest`.
