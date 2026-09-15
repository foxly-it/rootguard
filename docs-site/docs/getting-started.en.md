# Getting started

## Requirements

- Docker Engine or Docker Desktop with Docker Compose v2
- At least 2 vCPUs and 2 GB RAM recommended for the Docker host
- A fixed LAN IP or DHCP reservation for the RootGuard host
- Available TCP and UDP port 53 on the selected host address
- Git for the repository
- A strong admin password and a random internal API token

!!! success "Stable"
    RootGuard 1.0 has shipped: core paths, updates with rollback, and immutable, attested releases are production-ready. As with any network infrastructure, keep backups and an alternative DNS path available regardless.

## Platforms and verification

Every release is verified with the same guarded end-to-end test on native Linux amd64/arm64 runners and Docker Desktop. It covers login, AIO deployment, recursive DNS resolution, and rejection of an invalid DNSSEC chain.

| Platform | Architecture | Verification | Status |
| --- | --- | --- | --- |
| Linux, GitHub-hosted runner (Ubuntu 24.04) | `amd64` | Native automated runner | Passed 2026-07-28 |
| Linux, GitHub-hosted runner (Ubuntu 24.04) | `arm64` | Native automated runner | Passed 2026-07-28 |
| Docker Desktop 4.x on macOS | Apple Silicon / `arm64` | Same portable verifier | Passed 2026-07-28 |

Upgrade (a real N-1 → N upgrade through the control-plane updater) and backup/restore (a real export → teardown → fresh install → restore cycle) run on the same native `amd64`/`arm64` matrix, not just clean install.

**Supported platforms:**

- **Linux** (any distribution) with Docker Engine + Compose v2, `amd64` or `arm64` - the primary, fully verified target.
- **Docker Desktop on macOS**, Apple Silicon (`arm64`) - verified; Intel Macs use the same published `amd64` manifests but aren't separately tested.
- **Docker Desktop on Windows** (WSL2 backend) - not yet in the verification matrix, but expected to work.

Bare-metal/systemd installs and multi-node deployments are deliberately unsupported: RootGuard is a single-node Docker appliance.

!!! warning "Docker Engine version"
    RootGuard's Core and Updater containers call `docker cp` in three places (backup export, backup restore, update rollback). Three `docker cp` vulnerabilities (CVE-2026-41567, CVE-2026-41568, CVE-2026-42306) were fixed upstream in Docker Engine 29.5.1. **Run Docker Engine 29.5.1 or later**, or confirm your distribution's own package has backported all three fixes.

No hard minimum is enforced by the installer, but the full stack (Core, WebApp, Updater, AdGuard Home, Unbound) already runs comfortably at 1 vCPU / 2 GB RAM under light load. Practical recommendation: **2 vCPU, 2 GB RAM** as a comfortable floor for a real household network.

**Known limitations:** single-node only, no high availability; upgrade compatibility is N-1 → N only (skipping versions isn't supported); restore is a clean-replacement operation, not an in-place merge.

## Installation

!!! info "Automated installation"
    For the fast path with automatic Docker detection/installation, auto-generated security keys, and a short username/password prompt, see the [one-command quick start](https://rootguard.foxly.de/#quickstart) on the homepage. The steps below show the manual path for reference.

```shell
mkdir rootguard && cd rootguard

curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/v1.0.0/compose.release.yaml
curl -Lo .env https://raw.githubusercontent.com/foxly-it/rootguard/v1.0.0/.env.release.example

# Generate two independent security keys
openssl rand -hex 32
openssl rand -hex 32

# Fill in .env, then start RootGuard
docker compose -f compose.release.yaml up -d
```

Store two independently generated random values as `ROOTGUARD_API_TOKEN` and `ROOTGUARD_RECOVERY_TOKEN`, and set your own strong `ROOTGUARD_ADMIN_PASSWORD` in `.env`. RootGuard pulls versioned amd64/arm64 images from GHCR; no component checkout or local build is required.

```text
http://localhost:8080/login
```

## First setup

1. **Sign in** - Use `ROOTGUARD_ADMIN_USER` and `ROOTGUARD_ADMIN_PASSWORD`. The session is protected server-side and expires after twelve hours. "Forgot password?" also provides local recovery with a separate recovery key.
2. **Choose host address** - Choose an existing LAN IP. `0.0.0.0` binds all host addresses; a specific LAN IP limits exposure more narrowly.
3. **Run preflight** - RootGuard checks the address, port, Docker Engine, and Compose before changing containers. Occupied DNS ports are detected in two stages, including local resolvers such as systemd-resolved or dnsmasq.
4. **Deploy** - Unbound is started and verified, then AdGuard Home is configured internally with Unbound as its only upstream.

## Router & clients

Enter the fixed host IP shown by Setup as DNS server in your router. Never use `127.0.0.1` or the internal Docker address `172.29.53.2` on other devices. Port 53 must be reachable over TCP and UDP.

```shell title="Check from a client"
dig @192.168.178.10 example.com A
dig +dnssec @192.168.178.10 dnssec-failed.org A
```

The first query must return an address. The second must end with `SERVFAIL`, proving that an invalid DNSSEC chain is rejected.
