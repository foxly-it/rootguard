# RootGuard project state

Last updated: 2026-07-26

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
Browser --HttpOnly session--> WebApp --Bearer token--> Core --Docker API--> services
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
- AIO bootstrap split: the default Compose model starts WebApp, Core, and the
  internal control-plane updater; an authenticated Setup page performs typed
  network preflight and starts the managed Unbound/AdGuard data plane
  asynchronously.
- Persistent installation state and progress, atomic generated Compose state,
  private AdGuard administration, managed-resource labels, and controller
  network reconciliation after replacement.
- AIO deployment verified end to end on Docker Desktop/aarch64 with an explicit
  loopback bind: image pull, container creation, Unbound health, delayed
  AdGuard installer readiness, protected bootstrap, positive DNS, DNSSEC
  rejection, persistent state, and controller replacement reconciliation.
- Core handles SIGTERM with a bounded HTTP shutdown so planned container stops
  and replacements exit cleanly instead of appearing as crashes.
- Dashboard rebuilt around real installation, service, DNSSEC, bind, and
  protected-upstream state; placeholder resource metrics and synthetic trend
  data were removed.
- AdGuard page documents and visualises the RootGuard-managed official installer
  flow instead of suggesting that the native administration wizard must remain
  public.
- Authenticated native AdGuard Home UI gateway at `/adguard-ui/`; AdGuard still
  has no public administration port, Core injects its private credentials, and
  mutating browser requests are restricted to the same origin.
- Read-only live Unbound configuration endpoint reads the actual base and
  managed files from the running container; the settings UI shows the active
  file alongside directive-level explanations, and newly rendered managed
  configurations contain comments for every guided setting.
- Guided local-zone assistant in the Unbound UI creates and edits multiple
  A, AAAA, and CNAME records without requiring Unbound syntax. Its generated
  block remains visible in the expert configuration, preserves unrelated
  custom rules, detects concurrent edits, and uses the existing preview,
  effective `unbound-checkconf`, versioning, activation, and rollback pipeline.
- Guided conditional forwarding manages multiple canonical zones with ordered
  IPv4/IPv6 targets and an explicit, default-off recursive fallback. Core
  rejects root-zone, duplicate, loopback, link-local, multicast, RootGuard
  network, and expert-forwarding conflicts; a bounded authenticated probe
  requires every target to return `NOERROR` and an SOA record for the configured
  zone from the running Unbound container before WebGUI activation. Rejected
  responses expose a stable, bounded status explanation instead of raw resolver
  output.
  DNSSEC validation remains the per-zone default; trusted unsigned private
  servers require an explicit, visible `allow_unsigned` opt-in that renders a
  scoped `domain-insecure`. Rebinding protection likewise remains enabled by
  default; a separate visible `allow_private_addresses` opt-in renders a scoped
  `private-domain` for trusted RFC1918/private answers. Forwarding settings use
  the shared preview, effective `unbound-checkconf`, version history, restart
  rollback, and restore lifecycle.
- Unbound information architecture split into accessible Overview, Resolver,
  Local DNS, and Advanced tabs. The landing view now shows only configuration
  status, profile, versions, extensions, and on-demand diagnostics; cache
  tuning, live files, expert rules, and rollback use progressive disclosure.
  Tabs support arrow-key navigation, clear focus states, responsive overflow,
  and labelled tab panels without changing the validation and rollback paths.
- Extensible WebApp localisation registry with German and English catalogs,
  browser-language detection, persistent selection, document language and
  locale-aware date/number formatting. Navigation, setup, dashboard, stack
  lifecycle, AdGuard, guided Unbound settings, profiles, advisor, history and
  expert-editor controls use stable catalog keys; additional locale packages
  register without changing component or API logic.
- The former Overview entry is now the canonical Dashboard route. Setup uses a
  guided DNS-path blueprint, progress rail, endpoint preview, and compact
  deployment cards; Stack and Unbound share the Dashboard visual language.
  Subtle page, status, loading, and interaction motion is disabled automatically
  when the browser requests reduced motion.
- Browser Basic Auth has been replaced by a bilingual responsive login screen
  backed by expiring server-side sessions. The session cookie is HttpOnly and
  SameSite Strict, administration APIs and the AdGuard gateway reject missing
  sessions, same-origin checks cover login/logout, and the header exposes an
  explicit sign-out action. Sessions persist in a dedicated restricted volume
  so a controlled WebApp replacement does not force an unnecessary login.
- Lightweight vector RootGuard shield replaces the former embedded bitmap
  asset. The global header now identifies the current page next to the brand;
  the application shell adds a keyboard skip link, labelled main navigation,
  focusable content target, and responsive page-name treatment.
- Stack & Updates page for allowlisted AdGuard Home and Unbound lifecycle
  control. Update checks compare pulled and running image IDs; installation is
  asynchronous and persistent, creates protected data backups, replaces one
  service, performs DNS-chain health checks, and automatically restores the
  previous image and backup on failure.
- Separate internal control-plane updater for allowlisted Core and WebApp
  images. It remains alive while both containers are replaced as a pair,
  persists status outside either target, verifies exact image IDs and both
  health endpoints, and pins both previous images if either check fails.
  Browser requests cannot supply images, services, or Compose arguments.
- Bilingual project website deployed with enforced HTTPS at
  `https://rootguard.foxly.de`.
- Dedicated bilingual documentation at `/docs.html` covering installation,
  first setup, router and client configuration, the WebGUI, Unbound, AdGuard
  Home, updates and rollback, security, operations, troubleshooting, and
  environment configuration. The WebApp header now links there instead of the
  unrelated Foxly corporate site.
- Bilingual Wiki hub at `/wiki.html` and public release roadmap at
  `/roadmap.html`. Root `ROADMAP.md` is the canonical checklist through 1.0,
  with release gates for public alpha, useful Unbound administration, DNS
  policy, recovery, security, beta engineering, release candidate, and stable
  Docker appliance. Website status and dashboard mock now describe delivered
  capabilities instead of unavailable query statistics.
- Detailed Unbound configuration ownership and priority plan in
  `docs/unbound-configuration-roadmap.md`: fixed secure base, typed guided
  values, guarded expert directives, and permanently blocked browser controls.
- Repository-wide AGPL-3.0-or-later licensing for the main project, Core,
  WebApp, Unbound, and Updater. A separate trademark notice keeps RootGuard and
  Foxly IT names and visual identity outside the software license and asks
  modified distributions to use distinct branding.

## Current development slice

Stack Center and production visibility:

- trustworthy service state and actual component versions;
- completed data-plane updates and paired Core/WebApp updates through the
  separate helper; immutable release digests and retention policies remain;
- safe start, stop, and restart controls with clear impact;
- bounded, redacted service logs and actionable failure states.
- production-hardening and end-to-end CI for the new AIO installer, including
  immutable release image digests, host port conflict diagnostics, and failed
  deployment recovery.

## Remaining production milestones

1. Cohesive responsive UI shell and real dashboard metrics.
2. Complete Stack Center logs, richer health details, update history, and
   signed/immutable Core/WebApp release metadata.
3. Harden backup retention, export/restore, and immutable release digests.
4. DNS security advisor and production preflight checks.
5. AdGuard filter lists, exceptions, clients, and query statistics.
6. Private domains, access networks, custom diagnostics, and cache tools.
7. Runtime-provider abstraction for Docker and future bare-metal/systemd.
8. HTTPS for the appliance UI, sessions, roles, backup/restore, and installer
   hardening.

## Tracked editor follow-ups

- Extend the guided-assistant pattern to private domains and tightly scoped
  access-control rules.
- Generate and version the completion/documentation catalog for every directive
  supported by the installed Unbound release; the current catalog covers the
  common, safe RootGuard use cases.
- Expand semantic Advisor rules beyond the current security-, forwarding-,
  access-control-, and local-zone checks to cover more directive combinations.

## Release status

RootGuard remains in active alpha development. The DNS and configuration paths
are end-to-end tested, but update safety, backup/restore, authentication
hardening and roles, and bare-metal support are not yet production complete.
