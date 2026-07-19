# RootGuard Unbound

![Build Status](https://github.com/foxly-it/rootguard-unbound/actions/workflows/build.yml/badge.svg)
![Multi-Arch](https://img.shields.io/badge/arch-amd64%20%7C%20arm64-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Debian](https://img.shields.io/badge/base-Debian%20stable--slim-red)
![DNSSEC](https://img.shields.io/badge/DNSSEC-enabled-success)

Enterprise-grade Unbound DNS engine based on official Debian packages.

Built automatically via GitHub Actions and published to GHCR.

RootGuard Unbound is the DNS foundation of the RootGuard platform.

---

## Image

    ghcr.io/foxly-it/rootguard-unbound

---

## Available Tags

The image is automatically tagged using the official Debian Unbound package version.

Example (if Debian provides `1.22.0-2+deb13u1`):

    latest
    1.22.0-2-deb13u1
    1.22.0
    1.22

This guarantees:

- Alignment with official Debian package versions
- Transparent version traceability
- Automatic security updates via scheduled rebuilds

---

## Architecture

RootGuard Unbound follows a layered design.

### Immutable Core Configuration

Path inside container:

    /etc/unbound/unbound.conf

Contains:

- Network binding
- DNSSEC configuration
- Security hardening defaults
- Docker-compatible access control
- Modular include directive

This file is stable and not overwritten by RootGuard.

---

### Modular GUI Configuration Layer

Directory:

    /etc/unbound/unbound.d/

RootGuard will dynamically generate modular config files here, for example:

    local-zones.conf
    forwarding.conf
    performance.conf
    rpz.conf

Benefits:

- Clean separation of concerns
- Update-safe base layer
- Atomic configuration changes
- Easy rollback
- Clear auditability

---

### DNSSEC Trust Anchor Model

Debian provides a reference trust anchor:

    /usr/share/dns/root.key

On container startup, it is copied to:

    /var/lib/unbound/root.key

Unbound requires a writable trust anchor location to support RFC5011 automatic key rollover.

This ensures:

- No writes to package directories
- No dependency on unbound-anchor
- Debian compliance
- Proper DNSSEC validation

---

## Features

- Debian `stable-slim` base
- Official Debian `unbound` package
- Multi-architecture: `amd64` + `arm64`
- Automatic daily rebuild (security updates)
- Version tagging based on Debian package version
- DNSSEC validating resolver
- Modular configuration system
- Docker bridge aware access-control
- Runs as Debian packaged `unbound` system user
- Minimal default configuration
- Designed for RootGuard integration

---

## Default Configuration

The container ships with a secure Docker-compatible configuration:

- Interface: `0.0.0.0` and `::0`
- Port: `5335`
- DNSSEC enabled
- Access restricted to:
  - localhost
  - private Docker networks (`10/8`, `172.16/12`, `192.168/16`)
- Recursion enabled
- Private address protection active

It is designed for internal use, for example:

    AdGuard Home → RootGuard Unbound → Internet

It should **not** be exposed directly to WAN.

---

## Example Docker Run

    docker run -d \
      --name rootguard-unbound \
      -p 5335:5335/tcp \
      -p 5335:5335/udp \
      ghcr.io/foxly-it/rootguard-unbound:latest

---

## Example Docker Compose

    services:
      rootguard-unbound:
        image: ghcr.io/foxly-it/rootguard-unbound:latest
        container_name: rootguard-unbound
        restart: unless-stopped
        ports:
          - "5335:5335/tcp"
          - "5335:5335/udp"
        read_only: true
        cap_drop:
          - ALL
        security_opt:
          - no-new-privileges:true
        volumes:
          - unbound-config:/etc/unbound/unbound.d
          - unbound-state:/var/lib/unbound

    volumes:
      unbound-config:
      unbound-state:

---

## Testing

Test recursive resolution:

    dig @127.0.0.1 -p 5335 example.com

Expected:

- NOERROR
- ad flag present (DNSSEC validated)

Test DNSSEC validation:

    dig @127.0.0.1 -p 5335 dnssec-failed.org

Expected:

- SERVFAIL

If `dnssec-failed.org` returns `SERVFAIL`, DNSSEC validation is working correctly.

---

## Security Model

RootGuard Unbound is built with minimal attack surface:

- Runs as non-root user
- No open resolver configuration
- Explicit access-control rules
- Private network filtering
- Minimal responses
- Hidden version and identity
- Debian package tracking
- Automated rebuild pipeline

Designed to prevent:

- Open resolver abuse
- DNS amplification
- Private IP leakage
- DNSSEC downgrade attacks

---

## Build System

This image is automatically built via GitHub Actions:

- Triggered on push to `main`
- Daily scheduled rebuild
- Multi-arch build using `docker buildx`
- Debian package version automatically extracted
- Published to GHCR
- Version tags generated automatically

This ensures:

- Traceability
- Security patch propagation
- No manual image maintenance

---

## Integration with RootGuard

RootGuard will:

- Generate modular configuration files
- Validate configs using `unbound-checkconf`
- Activate validated settings atomically and restart the resolver
- Provide GUI explanations for:
  - Local Zones
  - Conditional Forwarding
  - Split Horizon DNS
  - Performance tuning
  - Security parameters
  - RPZ management

RootGuard Unbound is intentionally minimal and deterministic,
so the RootGuard application layer controls the dynamic logic.

---

## Roadmap

- Live config reload via `unbound-control` without restart
- Remote-control integration
- Metrics endpoint integration
- Config validation pipeline
- SBOM generation
- Cosign image signing
- Trivy security scanning
- Network auto-detection
- Advanced capability hardening

---

## License

MIT License
