# Clean-install platform verification

Tracked by [issue #39](https://github.com/foxly-it/rootguard/issues/39).

RootGuard's public beta uses one immutable Compose model on every supported
Docker platform. The clean-install verifier proves more than image
availability: it starts the control plane, signs in, runs the AIO preflight,
deploys AdGuard Home and Unbound, resolves a public name, and verifies that an
invalid DNSSEC chain is rejected.

## Verification matrix

| Platform | Architecture | Verification | Status |
| --- | --- | --- | --- |
| Linux, GitHub-hosted Ubuntu 24.04 | `amd64` | Native automated runner | Passed 2026-07-28 |
| Linux, GitHub-hosted Ubuntu 24.04 | `arm64` | Native automated runner | Passed 2026-07-28 |
| Docker Desktop 4.x on macOS | Apple Silicon / `arm64` | Same portable verifier | Passed 2026-07-28 |

The Docker Desktop result used Docker Engine 29.6.2 and Docker Compose 5.3.1.
It installed the immutable `v0.1.0-alpha.2` images, completed the managed DNS
deployment, returned a recursive IPv4 answer, and rejected
`dnssec-failed.org` with `SERVFAIL`.

Docker Desktop on Intel uses the same published `amd64` manifests but is not a
separately verified platform in the current verification matrix.

## Repeat the test

This test is intended only for an empty disposable Docker environment:

```sh
git clone https://github.com/foxly-it/rootguard.git
cd rootguard
./scripts/verify-clean-install.sh
```

Requirements are Docker Engine or Docker Desktop with Compose v2, `curl`,
`dig`, and `jq`. Set `ROOTGUARD_TEST_ARCH=amd64` or `arm64` to require an exact
Docker architecture.

The verifier refuses to start if any RootGuard container, named data volume, or
DNS network already exists. On a clean host it creates only RootGuard resources
and removes those resources when the test finishes. It never calls a global
Docker prune command and does not inspect or delete unrelated containers,
images, networks, or volumes.

## Automated evidence

The `Public clean install` workflow executes the verifier on native
GitHub-hosted `amd64` and `arm64` Linux runners. Each job records the runner,
Docker architecture, Docker Engine version, and Compose version in its job
summary. A failed install prints bounded control-plane and DNS-service logs
before cleaning up its own resources.

The first complete native matrix passed in
[GitHub Actions run 30353823582](https://github.com/foxly-it/rootguard/actions/runs/30353823582).

## Supported platforms (frozen for 0.9)

- **Linux** (any distribution) with Docker Engine + Compose v2, `amd64` or
  `arm64` - the primary, fully verified target (native CI matrix above).
- **Docker Desktop on macOS**, Apple Silicon (`arm64`) - verified; Intel
  Macs use the same published `amd64` manifests but aren't separately
  tested.
- **Docker Desktop on Windows** (WSL2 backend) - not yet in the verification
  matrix. Expected to work (same Compose model, same published images) but
  unverified; treat as best-effort until a native run is added.

Not supported: bare-metal/systemd installs and multi-node deployments are
explicitly out of scope for 1.0 (`ROADMAP.md`'s "Post-1.0 / Future"
section) - RootGuard 1.0 is a single-node Docker appliance only.

## Minimum requirements

No hard minimum is enforced by the installer, but
[the 0.9 performance baseline](performance-baseline.md) measured RootGuard's
full stack (Core, WebApp, Updater, AdGuard Home, Unbound) running
comfortably on a constrained 1 vCPU / 2 GB RAM host at light-to-moderate
query load (steady-state memory well under 100 MB across all five
containers). A 1 vCPU host is not recommended as a real target - it becomes
the throughput ceiling under sustained load, not RootGuard itself (see that
document's medium-network section). Practical recommendation: **2 vCPU,
2 GB RAM** as a comfortable floor for a real household network; more only
matters for very large query volumes or the `large` Unbound resource
profile's bigger caches.

## Known limitations

- **Single-node only.** No high availability, no failover between
  instances - see [the disaster-recovery runbook](disaster-recovery.md) for
  what to do when the one node fails.
- **Upgrade compatibility is N-1 → N only** - see
  [the compatibility matrix](compatibility-matrix.md). Skipping versions
  when upgrading isn't tested or supported; upgrade through each release in
  sequence.
- **Restore is a clean-replacement operation, not an in-place merge** - see
  [the backup/restore docs](backup-export.md). It deploys into a fresh,
  never-installed target; it cannot be run against an already-installed
  instance without tearing it down first.
- **Pre-1.0 status**: RootGuard is not yet recommended as the sole DNS
  service for a production network (see the README's own notice). This
  changes at `1.0.0` per `ROADMAP.md`'s exit criteria for the `0.9`/release
  candidate stage.

## Support policy

- Each `0.1.0-{alpha,beta}.N` / future `1.0.x` release is supported until
  the next one ships - only the current release receives fixes; there's no
  parallel maintenance of older lines pre-1.0.
- Security-relevant fixes land as a new release promptly rather than being
  backported; there is no separate security-only release channel before
  1.0.
- Once `1.0.0` ships, this section will be revised with a concrete support
  window per release line - deferred until then since the pre-1.0 series
  moves too fast for a fixed window to mean anything yet.
