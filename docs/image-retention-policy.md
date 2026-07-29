# RootGuard image integrity and retention

Last reviewed: 2026-07-28

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

The Updater source did not change between alpha.2 and alpha.3. Its tested
multi-architecture manifest is therefore intentionally reused under the new
coordinated stack version.
| AdGuard Home | `v0.107.78` | `sha256:1ea34eafe5dc691007946e8eaab7bf46b0de9412f39213d8c06e48b53bf9a6c5` |

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

Development Compose files may continue to use local or mutable image references.
They are not release artifacts and are deliberately kept separate from
`compose.alpha.yaml`.

## Preparing the next release

Build and test all component images first. Resolve the published
multi-architecture digest for each version tag, update `compose.alpha.yaml` and
`.env.alpha.example` together, and run the clean installation smoke test. The
integration workflow rejects a release Compose model if a required image lacks
an `@sha256:` digest or if a service image uses `latest`.
