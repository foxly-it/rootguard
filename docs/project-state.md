# RootGuard project state

Last updated: 2026-07-20

This file is the persistent handover for future development sessions. Read it
before repeating repository-wide discovery.

## Repository layout

- `foxly-it/rootguard` coordinates deployment, documentation, CI, and website.
- `foxly-it/rootguard-core` owns privileged orchestration and configuration.
- `foxly-it/rootguard-webapp` owns the authenticated backend proxy and React UI.
- `foxly-it/rootguard-unbound` is also usable as a standalone resolver image.

The component repositories are included in the main repository as Git
submodules. Component PRs are merged before their revisions are updated here.

## Runtime architecture

```text
Browser --Basic Auth--> WebApp --Bearer token--> Core --Docker API--> services
Network clients --TCP/UDP 53--> AdGuard Home --> Unbound --> DNS hierarchy
```

- Only the WebApp and DNS port are published.
- AdGuard administration, Core, and the Docker socket remain internal.
- AdGuard forwards exclusively to Unbound at `172.29.53.2:5335`.
- Router clients use the fixed LAN address of the RootGuard host, not localhost
  or the internal Docker address.

## Delivered and verified

- Secure AdGuard Home first-time setup through RootGuard.
- Generated AdGuard credentials remain in Core's persistent data volume.
- Unbound runs read-only, non-root, without additional capabilities.
- Authenticated Unbound settings for QNAME minimisation, prefetch,
  serve-expired, cache TTLs, and resolver threads.
- Side-effect-free change preview and generated configuration display.
- `unbound-checkconf` validation before activation.
- Atomic activation, automatic restart rollback, 20-version history, and
  confirmed restore through the WebGUI.
- Fixed diagnostics for configuration, recursive resolution, and DNSSEC.
- Four validated Unbound operating profiles (balanced, privacy, resilience,
  and performance) that load only as drafts.
- Deterministic draft advice for privacy, availability, cache efficiency, and
  resource usage, including explanations and recommended value ranges.
- Separate `90-rootguard-custom.conf` expert editor with generic syntax
  highlighting, templates, prefix completion, contextual documentation, risk
  labels, deterministic advice, and a 64 KiB limit.
- Custom configuration policy blocks file includes, listeners, remote control,
  trust-anchor changes, DNSSEC bypasses, and values owned by guided settings.
- Candidate and effective `unbound-checkconf` validation plus atomic activation;
  guided settings and custom content share history and rollback.
- Integration CI starts the complete stack and verifies AdGuard bootstrap,
  Unbound preview/apply/history/diagnostics/restore, positive DNS, and DNSSEC
  rejection.
- Bilingual project website deployed with enforced HTTPS at
  `https://rootguard.foxly.de`.

## Current development slice

Stack Center and production visibility:

- trustworthy service state and actual component versions;
- read-only update availability without automatic mutation;
- safe start, stop, and restart controls with clear impact;
- bounded, redacted service logs and actionable failure states.

## Remaining production milestones

1. Cohesive responsive UI shell and real dashboard metrics.
2. Stack Center with service health, versions, logs, and safe controls.
3. Read-only update checks, followed later by backup-backed updates/rollback.
4. DNS security advisor and production preflight checks.
5. AdGuard filter lists, exceptions, clients, and query statistics.
6. Local zones, conditional forwarding, custom diagnostics, and cache tools.
7. Runtime-provider abstraction for Docker and future bare-metal/systemd.
8. HTTPS for the appliance UI, sessions, roles, backup/restore, and installer.

## Tracked editor follow-ups

- Generate and version the completion/documentation catalog for every directive
  supported by the installed Unbound release; the current catalog covers the
  common, safe RootGuard use cases.
- Expand semantic Advisor rules beyond the current security-, forwarding-,
  access-control-, and local-zone checks to cover more directive combinations.

## Release status

RootGuard remains in active alpha development. The DNS and configuration paths
are end-to-end tested, but update safety, backup/restore, UI authentication,
and bare-metal support are not yet production complete.
