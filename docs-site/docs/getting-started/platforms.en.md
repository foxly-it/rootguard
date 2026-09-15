# Clean install, same verification

Every release is verified with the same guarded end-to-end test on native Linux amd64/arm64 runners and Docker Desktop. It covers login, AIO deployment, recursive DNS resolution, and rejection of an invalid DNSSEC chain.

## Verification matrix

| Platform | Architecture | Verification | Status |
| --- | --- | --- | --- |
| Linux, GitHub-hosted runner (Ubuntu 24.04) | `amd64` | Native automated runner | Passed 2026-07-28 |
| Linux, GitHub-hosted runner (Ubuntu 24.04) | `arm64` | Native automated runner | Passed 2026-07-28 |
| Docker Desktop 4.x on macOS | Apple Silicon / `arm64` | Same portable verifier | Passed 2026-07-28 |

Upgrade (a real N-1 → N upgrade through the control-plane updater) and backup/restore (a real export → teardown → fresh install → restore cycle) run on the same native `amd64`/`arm64` matrix, not just clean install.

## Repeat the test yourself

Intended only for an empty, disposable Docker environment:

```shell
git clone https://github.com/foxly-it/rootguard.git
cd rootguard
./scripts/verify-clean-install.sh
```

Requirements: Docker Engine or Docker Desktop with Compose v2, `curl`, `dig`, and `jq`. Set `ROOTGUARD_TEST_ARCH=amd64` or `arm64` to require an exact Docker architecture.

The verifier refuses to start if any RootGuard container, named data volume, or DNS network already exists. On a clean host it creates only RootGuard resources and removes them again after the test - never via a global Docker prune command, and without touching unrelated containers, images, networks, or volumes.

## Supported platforms

- **Linux** (any distribution) with Docker Engine + Compose v2, `amd64` or `arm64` - the primary, fully verified target.
- **Docker Desktop on macOS**, Apple Silicon (`arm64`) - verified; Intel Macs use the same published `amd64` manifests but aren't separately tested.
- **Docker Desktop on Windows** (WSL2 backend) - not yet in the verification matrix. Expected to work (same Compose model, same published images) but unverified - treat as best-effort until a native run is added.

Bare-metal/systemd installs and multi-node deployments are deliberately unsupported: RootGuard is a single-node Docker appliance.

## Docker Engine version

RootGuard's Core and Updater containers call `docker cp` in three places (backup export, backup restore, and update rollback). Three `docker cp` vulnerabilities (CVE-2026-41567, CVE-2026-41568, CVE-2026-42306) were fixed upstream in Docker Engine 29.5.1. **Run Docker Engine 29.5.1 or later**, or confirm your distribution's own package has backported all three fixes - some distributions patch security issues without bumping the version string they report.

The installer's own preflight surfaces this as a non-blocking advisory (`docker_engine_cp_cve`) whenever it can read the Docker Engine version unambiguously as below 29.5.1 - it warns instead of blocking because backported distro packages (e.g. Debian/Ubuntu's own `docker.io`) are common enough that a plain version comparison would produce real false positives.

## Minimum requirements

No hard minimum is enforced by the installer, but the performance baseline measured RootGuard's full stack (Core, WebApp, Updater, AdGuard Home, Unbound) running comfortably on a constrained 1 vCPU / 2 GB RAM host at light-to-moderate query load (steady-state memory well under 100 MB across all five containers). A 1 vCPU host is not recommended as a real target - it becomes the throughput ceiling under sustained load, not RootGuard itself. Practical recommendation: **2 vCPU, 2 GB RAM** as a comfortable floor for a real household network.

## Known limitations

- **Single-node only.** No high availability, no failover between instances.
- **Upgrade compatibility is N-1 → N only.** Skipping versions when upgrading isn't tested or supported - upgrade through each release in sequence.
- **Restore is a clean-replacement operation, not an in-place merge.** It deploys into a fresh, never-installed target; it cannot be run against an already-installed instance without tearing it down first.
