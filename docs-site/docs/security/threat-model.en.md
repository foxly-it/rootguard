# Threat model

## Summary

- WebApp without Docker socket or arbitrary host commands
- HttpOnly, SameSite Strict session and same-origin checks for writes
- Session inventory with targeted revocation, rate limiting, and an audit log for login and password recovery
- Core and Updater only on internal Docker networks and protected by bearer tokens
- Unbound read-only, non-root, without additional capabilities
- AdGuard administration without a public port
- Automated dependency, container, secret, and static-analysis scans in CI

!!! note "HTTPS"
    RootGuard deliberately does not terminate its own TLS - a proxy in front of it does. [HTTPS via a reverse proxy](../guides/https-reverse-proxy.md) covers the two requirements (Host-header passthrough, `X-Forwarded-Proto`) and example configurations for Caddy, Zoraxy, Nginx Proxy Manager, and HAProxy.

The sections below go into detail: who RootGuard trusts and how far, what a compromised actor can each reach, and which residual risks are deliberately left open.

As of 2026-08-08. Extends the architecture description with an explicit look at who RootGuard trusts and how far, what a compromised actor can each reach, which countermeasures already apply, and which residual risks are deliberately left open or out of scope for this project.

## Scope

This covers six actors/boundaries: Docker socket holders, the browser, internal networks, the update supply chain, backups, and the AdGuard gateway. Out of scope: compromise of the host itself (physical access, kernel exploits, compromised Debian/Alpine/Docker base images) - RootGuard trusts the host operating system and Docker engine it runs on, like any containerized application.

## Actors and trust boundaries

### 1. Docker socket holders (Core, Updater)

**Access:** `rootguard-core` and `rootguard-updater` have `/var/run/docker.sock` mounted. Whoever controls this socket effectively controls the host - a new privileged container with an arbitrary bind mount is trivially reachable from there.

**If compromised:** Full host compromise. This is by far the largest single trust boundary in the system.

**Existing countermeasures:**

- Both images currently run as `root` (see the reasoning directly in the respective `Dockerfile`) - no additional privilege-escalation step needed, but also no reduction of the attack surface within the container itself.
- Core only talks to a fixed, controlled set of images, volumes, networks, and commands; browser requests can choose neither image names nor Compose arguments nor containers freely. A compromised browser or compromised WebApp therefore cannot move Core directly to arbitrary Docker commands - only to the narrow, typed operations the Core API actually offers.
- The updater only knows `core` and `webapp` as replacement targets, only pulls configured target images, and isn't reachable from the host.
- Manual Docker cleanup accepts no resource names from the browser: Core derives candidates exclusively from its successful update history or the fixed volume label `io.rootguard.cleanup=true`, re-verifies usage before preview and execution, and never calls global prune commands.
- Digest-pinned Core/WebApp releases go through a Cosign/SLSA provenance check against the expected GitHub workflow signer before activation (see section 4).

**Known residual risks / open:**

- A bug or vulnerability *inside* Core or Updater that lets an attacker issue their own Docker API calls instead of the intended narrow operations would lead directly to host compromise - there is no second line of defense between "code bug in Core" and "full Docker access".
- Concrete planned hardening step: a dedicated `docker-socket-proxy` sidecar that holds the socket itself and only lets a narrowly allowlisted subset of the Docker API through, while Core and Updater themselves run unprivileged. Not yet implemented.

### 2. Browser / authenticated user

**Access:** HttpOnly/SameSite=Strict session against the WebApp; through it, every guided and expert function of the interface.

**If compromised (stolen session, XSS, etc.):** Full administrator access to every RootGuard function - configuration changes, updates, AdGuard management.

**Existing countermeasures:**

- PBKDF2-SHA256 with 600,000 iterations for the admin password, constant-time comparison, HttpOnly/SameSite=Strict cookies with `Secure` once HTTPS is active.
- Mutating requests must originate from the same origin (same-origin write check) - classic CSRF from a foreign origin doesn't work as a result.
- The Unbound expert editor is limited to a single file (`90-rootguard-custom.conf`); includes, listeners, remote control, container paths, and trust anchors stay locked - even a fully compromised browser session can't reach host files or arbitrary Unbound directives through it.
- The separate `ROOTGUARD_RECOVERY_TOKEN` for password reset grants neither a session nor access to Core or AdGuard by itself.
- Session inventory with targeted revocation, reachable via "Active sessions" in the user menu - a stolen session no longer has to stay valid until TTL expiry.
- Rate limiting on login and password recovery: 5 failed attempts in a 5-minute window locks out further attempts - even a subsequently correct password is rejected during an active lockout, so a lockout can't be bypassed by simply continuing to guess.
- A bounded, persisted audit log (max. 500 entries) records login success/failure, rate limiting, logout, password recovery, and session revocation - visible in the same "Active sessions" panel.
- The same rate-limit/audit principle also covers destructive actions outside authentication: a shared, per-session sliding-window budget (30 requests / 5 minutes across every protected route, not a separate budget per route) limits Unbound activation/restore/custom-config/import/diagnostic-logging, service start/stop/restart and updates, backup settings/export/restore, manual cleanup, control-plane update installation, installation deploy, and AdGuard bootstrap and filter toggle. All affected routes are plain proxies from the WebApp to Core, so the protection takes effect at the single browser-facing entry point without needing to change Core itself.

**Known residual risks / open:**

- The shared budget protects against mass abuse of a single (e.g. compromised) session, not against a single targeted destructive call by a legitimately authenticated user - that is expected behavior, not a bug: auth-backed authorization remains the actual access control, the rate limit is an additional damage-limitation layer.

### 3. Internal networks (control, edge, egress, DNS network)

**Access:** Four Docker networks separate responsibilities: `edge` (WebApp host port), `control` (WebApp↔Core↔Updater, `internal: true` - no route to the internet at all), `egress` (real internet access, but only `rootguard-attestation-proxy` sits here), and the internal DNS network (AdGuard↔Unbound). Only the WebApp and the DNS port get host ports; AdGuard's native administration stays reachable only internally.

**If compromised (e.g. another container on the same Docker host/network):** Access to internal interfaces that aren't meant to be public (AdGuard admin API, Core's bearer-token API).

**Existing countermeasures:**

- Network segmentation as above; Core only reaches Unbound over a fixed internal address (`172.29.53.2:5335`), no public fallback.
- The Core API is bearer-token protected (`ROOTGUARD_API_TOKEN`), not just network-isolated.
- `control`'s own internet isolation stays total except for one narrow, auditable path: `rootguard-attestation-proxy`, a CONNECT-only forward proxy with a hardcoded, 4-host allowlist (`ghcr.io`, `pkg-containers.githubusercontent.com`, `tuf-repo-cdn.sigstore.dev` - exactly what cosign's own attestation verification needs, empirically confirmed, nothing more - plus `api.github.com` for Core's GitHub Releases self-update-discovery check). It's defense-in-depth, not an authentication boundary - Core and the Updater, the only two callers that can reach it, already hold the Docker socket and run as root, i.e. already have full host privilege; the point is keeping `control` itself provably internet-isolated while making the one legitimate egress path explicit rather than reopening internet access wholesale.

**Known residual risks / open:**

- Whoever can already start arbitrary containers *on the same Docker host* can generally also attach to `control`/the internal DNS network (absent additional Docker network policies/host-level firewalling) - network segmentation protects against external attackers and against other, non-privileged workloads on the same host, not against an attacker who already has Docker access on the same host. This overlaps with actor 1: whoever controls the Docker socket also controls the networks.

### 4. Update supply chain

**Access:** GHCR images for all six components; the updater helper pulls and activates new Core/WebApp/AdGuard/Unbound images.

**If compromised (e.g. stolen GHCR publish credentials, a compromised CI runner):** A malicious image could be distributed as a legitimate RootGuard release and automatically adopted by existing installations - ultimately equivalent to actor 1 (host compromise), just via the update path instead of directly.

**Existing countermeasures:**

- Digest-pinned Core/WebApp/Updater/Unbound/Blockpage releases are all checked via Cosign against the signed SLSA provenance before activation: expected GitHub repository and workflow signer, expected GitHub Actions OIDC issuer, verification of the Sigstore transparency data. The embedded Cosign verifier itself is pinned by digest - no moving dependency at this point.
- Local builds, mutable tags (`:latest` etc.), and third-party images explicitly receive no RootGuard trust approval.
- An update failure (health check after swap) automatically pins the previous image ID back and re-verifies.
- On the CI side: `trivy` checks all six component images/Dockerfiles for known vulnerabilities and misconfigurations, `govulncheck` and `staticcheck` run against every Go module, `gitleaks` against the entire git history - reduces the risk of a known vulnerability or an accidentally committed secret reaching a published release unnoticed.

**Known residual risks / open:**

- AdGuard Home (a third-party image) is the one component that doesn't go through a Cosign provenance check - trust here is based on digest pinning and the upstream signature, not a RootGuard-owned signature chain. `rootguard-attestation-proxy` itself joined self-update management on 2026-09-03 and is verified through the identical Cosign policy as every other RootGuard-built component; verification of a new candidate proxy image runs through the *currently running* proxy instance, the swap only happens after that succeeds - no bootstrapping gap.
- No SBOM/provenance for every release used to be an open gap - now delivered.
- Image signing beyond Cosign, applied consistently across all six self-update-managed components, used to be an open gap too - now delivered as well.

### 5. Backups

**Access:** Persistent RootGuard volumes (configuration history, sessions, AdGuard credentials, installation state) and their safeguards before every update/swap.

**If compromised (access to a backup/volume snapshot):** Disclosure of every credential and configuration it contains - AdGuard admin credentials live with owner-only permissions in the persistent volume, sessions live server-side in the session volume.

**Existing countermeasures:**

- Backups/snapshots are created exclusively server-side before a controlled swap, not retrievable from the browser.
- Internal update backups are limited per service to a configurable 2-50 restore points (default 5); storage usage and unrecognized data stay visible on the Backups page.
- Automatic cleanup only accepts canonical timestamp/service paths with a manifest matching the allowed service and container. Unknown data and symlinks are never deleted.
- Password hashes are PBKDF2-SHA256 salted, never stored in plaintext.
- Portable full backups are encrypted, authenticated, and interoperable with age-v1 and a scrypt-derived passphrase identity before download. A versioned manifest contains SHA-256 checksums; sessions and external `.env` secrets aren't part of the export. Fixed sources, symlink rejection, and private, always-removed plaintext staging bound path and leftover-data risks.
- The guided restore validates schema, required files, allowed paths/types, the exact manifest, sizes, and SHA-256, plus hard upload/expansion/file-count limits before any change. Apply validates again, requires confirmation, and fails closed if the installation or managed Docker resources aren't clean.

**Known residual risks / open:**

- The export passphrase isn't stored and must be re-entered for every preview and restore. Over plain HTTP, the finished archive is encrypted, but the passphrase isn't transport-protected on its way from the browser to the local WebApp; the interface warns visibly - see [HTTPS via a reverse proxy](../guides/https-reverse-proxy.md) for the supported HTTPS operation.
- Live data can change during individual file copies. Updates are excluded, but a transaction-like service snapshot and its restore verification were, until recently, still separate open items - now delivered.

### 6. AdGuard gateway

**Access:** RootGuard proxies the native AdGuard Home interface under the fixed path `/adguard-ui/`; Core supplies the internal AdGuard credentials for it.

**If compromised (a vulnerability in AdGuard Home itself or in the proxy path):** Access to AdGuard's filter rules, DNS query logs (if enabled), and the AdGuard configuration - but not to Core, Unbound, or the Docker socket, since AdGuard itself has none of that access.

**Existing countermeasures:**

- AdGuard's native administration is reachable only internally, never via a host port.
- Only the fixed, authenticated proxy path exists; freely chosen targets and a public administration port are excluded.
- The reverse proxy discards client-supplied `X-Forwarded-*` headers before forwarding and sets them again server-side instead of passing them through unchanged - closes a header-spoofing path that was previously open.
- Mutating requests via the proxy path likewise must originate from the same origin.

**Known residual risks / open:**

- AdGuard Home itself lives outside the RootGuard codebase - a vulnerability in AdGuard Home directly affects RootGuard installations through this path. Mitigated only by digest pinning and promptly following up on AdGuard releases, not by an additional RootGuard-owned protection layer.

## Deliberately out of scope (non-goals)

- **A malicious administrator:** whoever holds legitimate admin credentials is trusted by definition - RootGuard doesn't protect against an authorized but maliciously acting operator.
- **Physical host access:** whoever has physical or hypervisor access to the host can bypass any containerized application.
- **Compromised base images/registries themselves** (Docker Hub, GHCR, Debian/Alpine package sources) - RootGuard verifies what it references (digest pinning, Cosign where available), but ultimately trusts the same roots as any other containerized application.
- **A multi-user/roles model and external identity providers** - only if real demand requires it.
- **HTTPS/TLS termination by RootGuard itself** - a deliberate scope decision: documented operation behind an established reverse proxy (Caddy, Zoraxy, Nginx Proxy Manager, HAProxy) instead of a RootGuard-native TLS implementation, see [HTTPS via a reverse proxy](../guides/https-reverse-proxy.md).

## References

- [Architecture description](https://github.com/foxly-it/rootguard/blob/main/docs/architecture.md){: target="_blank" rel="noopener" } - detailed description of the mechanisms referenced here.
- [SECURITY.md](https://github.com/foxly-it/rootguard/blob/main/SECURITY.md){: target="_blank" rel="noopener" } - the vulnerability reporting path.
