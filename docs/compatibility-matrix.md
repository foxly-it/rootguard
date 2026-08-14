# Compatibility matrix

What RootGuard actually proves works together, and where that proof comes
from. Each axis below is backed by a real, currently-passing CI check or a
verified test run - not a claim without evidence.

## RootGuard version (upgrade continuity)

Upgrading from the immediately preceding published pre-release to the
current one is exercised on every release: `release-alpha.yml`'s
`upgrade-test` job deploys the previous release exactly as it shipped
(its own pinned Compose model and images), completes guided setup, verifies
DNS, then upgrades Core and WebApp in place through the real control-plane
updater - never a synthetic fixture - to the version being published, and
verifies the running images and DNS resolution afterward.

Scoped to N-1 -> N: RootGuard is pre-1.0 and doesn't yet promise
compatibility further back than the one release directly before the
current one.

## Docker platform and engine

See [platform-support.md](platform-support.md) for the full verification
matrix and how to repeat it. Currently verified: Linux `amd64`/`arm64` on
GitHub-hosted runners, and Docker Desktop on Apple Silicon.

## AdGuard Home channel

`ci-adguard-compat.yml` runs the full bootstrap -> status -> filtering
toggle -> DNS resolution -> filter-check surface against both channels
RootGuard's guided setup offers (`adguard_channel: stable` or `beta`), on a
matrix so a channel-specific break doesn't mask the other. Runs on every
push/PR touching the AdGuard integration code, plus weekly on a schedule to
catch upstream drift independent of RootGuard's own commits.

## Unbound version

Not an operator choice - RootGuard builds and pins its own patched Unbound
release from upstream source (see `rootguard-unbound/Dockerfile` and
`docs/threat-model.md`). `ci-unbound.yml` verifies `unbound-checkconf`,
DNSSEC/identity/version behavior, and trust-anchor volume compatibility
natively on both `amd64` and `arm64` before any multi-arch image is
published, plus a dedicated `scenario-tests` job exercising real guided
configurations (home network, VLANs, split DNS, IPv6-only records, broken
upstreams, DNSSEC failures) against a live resolver.
