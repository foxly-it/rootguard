# RootGuard project state

Last updated: 2026-08-24

This file is the persistent handover for future development sessions. Read it
before repeating repository-wide discovery.

## Repository layout

`foxly-it/rootguard` is a monorepo coordinating deployment, documentation,
CI, and website, alongside five independently buildable component
directories (each with its own Dockerfile and path-filtered CI workflow):

- `rootguard-core/` owns privileged orchestration and configuration.
- `rootguard-webapp/` owns the authenticated backend proxy and React UI.
- `rootguard-unbound/` is also usable as a standalone resolver image.
- `rootguard-updater/` is the internal control-plane updater helper.
- `rootguard-blockpage/` is the landing page shown for AdGuard-blocked
  requests; also usable standalone (see its own README).

These were four separate repositories included as Git submodules until the
monorepo migration (see "Delivered and verified" below); their full commit
history was preserved via `git subtree` merges. The formerly-separate repos
are now archived on GitHub, read-only, for history and old issue/PR links.

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
- Unbound 1.25.2 is compiled from the SHA-256-verified official NLnet Labs
  source on a digest-pinned Debian 13 Slim multi-stage base. Direct packages are
  version-pinned and verified through Debian's signed repository metadata; the
  `amd64`/`arm64` publication includes SBOM and provenance attestations. The
  component image passed configuration, recursive DNS, DNSSEC, trust-anchor,
  health, read-only, capability-free, and multi-architecture build checks.
  The Core-managed update from the prior image to the published source build
  also passed on Docker Desktop/arm64: update history recorded success, the
  container stayed healthy as `100:101`, AdGuard resolved through it, signed
  answers carried the `ad` flag, the broken DNSSEC test returned `SERVFAIL`,
  and startup logs were clean. A Debian 13 `amd64` LXC then passed the immutable
  alpha clean install, Core-managed update from the alpha image to the published
  source build, and an injected failure rollback. The rollback restored the
  exact prior image ID, kept the resolver healthy, retained recursive DNS, and
  recorded `rolled_back`; the restored image also passed source-checksum,
  DNSSEC, and clean-log checks. The host was returned to zero test containers,
  volumes, and RootGuard networks afterward.
- `ci-unbound.yml`'s functional checks (`unbound-checkconf`, the DNSSEC/
  identity/version smoke tests, the trust-anchor volume-compatibility check)
  now run as a native matrix on both `amd64` and `arm64` (`ubuntu-latest`,
  `ubuntu-24.04-arm`) before the multi-arch push, gated behind both legs
  passing. Previously these only ever ran on the implicit `amd64` runner;
  the separate `docker/build-push-action` step that produces and pushes the
  actual `linux/amd64,linux/arm64` manifest built `arm64` under QEMU
  emulation but never functionally verified it - a successful cross-arch
  build was silently treated as proof the image worked
  ([rootguard#121](https://github.com/foxly-it/rootguard/pull/121)).
- Unbound keeps a stable non-root identity (`100:101`) across image rebuilds,
  uses the system socket send buffer, and Core migrates the persistent resolver
  state ownership with a network-isolated, capability-restricted helper before
  image replacement. The WebApp update path from 1.22.0 to 1.25.2 was verified
  with a writable DNSSEC trust anchor, clean startup logs, positive resolution,
  and DNSSEC rejection.
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
  protected-upstream state. It now adds live aggregate CPU and memory usage for
  the five allowlisted RootGuard containers plus aggregate AdGuard Home query,
  blocked-query, and filter-rate values without exposing query names or clients.
- AdGuard page documents and visualises the RootGuard-managed official installer
  flow instead of suggesting that the native administration wizard must remain
  public.
- WebApp text actions now share explicit primary, secondary, and destructive
  button variants across Dashboard, Setup, Stack, Unbound, and AdGuard while
  tabs, selectable cards, completion entries, and icon-only controls retain
  their interaction-specific styling. The AdGuard overview reduces its status
  list to the three checks an operator actually acts on (configuration, DNS
  baseline, active filter lists), each with a right-aligned button deep-linking
  into the matching native AdGuard Home page; its on-demand local filter check
  opens immediately in a focused, scroll-safe dialog with progress, retry,
  error, summary, and per-host results, and now also carries a master
  filtering on/off toggle so disabling filtering doesn't require a separate
  AdGuard admin session.
- On-demand AdGuard filtering diagnostics use the local authenticated
  `check_host` API for advertising and tracking probes without opening test
  websites. Legitimate services and AMTSO/Wicar portals remain explicitly
  informational. RootGuard also reconciles a conservative DNS baseline through
  AdGuard's validated API: exclusive Unbound upstream, no fallback, filtering,
  DNSSEC signalling, disabled ECS, refused ANY requests, bounded cache and
  response TTLs, and daily filter refreshes.
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
- Guided private DNS manages canonical `private-domain` entries and an explicit
  NXDOMAIN-or-transparent policy for each RFC1918 reverse range. NXDOMAIN is the
  safe default; transparent fallback carries a visible leakage warning. Guided
  A/AAAA records can generate `local-data-ptr` only when the address is unique
  across the complete guided draft. Core validation, expert conflict checks,
  preview, effective `unbound-checkconf`, history, activation, and rollback all
  cover the new settings. AdGuard Home's own private reverse-resolver routing
  remains an integration responsibility rather than duplicated resolver logic.
- Conflict detection now covers every Unbound surface RootGuard guides before
  1.0: local hosts and zones, forwarding, private domains, and expert text.
  Access rules remain available to advanced operators through the expert editor
  and are not a pre-1.0 guided feature. A guided access-rules surface may later
  serve as a reference use case for the explicitly post-1.0 extension
  architecture ([rootguard#186](https://github.com/foxly-it/rootguard/issues/186)).
- Typed backend foundation for the planned zone-centred host inventory: new
  `LocalZone`/`LocalHost` types in `rootguard-core/internal/unbound/settings.go`
  (hostname plus IPv4 and/or IPv6, optional PTR), validated (canonical zone and
  hostname syntax, per-zone and total host limits, address family and
  reserved-address checks, and at most one PTR claim per address across all
  zones) and rendered as `local-zone`/`local-data`/`local-data-ptr` directives -
  verified against the real Unbound binary's `unbound-checkconf`, not just
  parsed. Reuses the existing `Preview`/`Apply`/`History`/`Restore` lifecycle
  and the expert-config conflict check unchanged; no new activation path or API
  route was needed since Core's settings endpoints already decode the whole
  `Settings` struct
  ([rootguard#129](https://github.com/foxly-it/rootguard/pull/129), closes
  [#127](https://github.com/foxly-it/rootguard/issues/127)).
- FRITZ!Box TR-064 host discovery adapter (`rootguard-core/internal/routerimport`):
  speaks the standard `Hosts:1` service - `GetHostNumberOfEntries` followed by
  a bounded (256-entry) `GetGenericHostEntry` loop - and answers an HTTP
  Digest challenge (RFC 7616, MD5, `qop=auth`) when the router requires one,
  while tolerating one that answers `200` directly without ever challenging
  (both paths verified live against a real FRITZ!Box 6690 Cable, firmware
  267.08.25: 47 hosts discovered, correct active/inactive and field mapping).
  Submitted credentials are used for exactly one discovery request and never
  persisted, returned in any response, or written into generated Unbound
  config. A Core endpoint runs discovery and returns the normalized,
  unselected draft list only - it does not touch `LocalZone`/`Settings` yet,
  the same read-only-probe shape as the existing forward-zone reachability
  check. The preview/selection UI and conflict detection against existing
  guided/expert records are tracked separately
  ([rootguard#133](https://github.com/foxly-it/rootguard/pull/133), closes
  [#132](https://github.com/foxly-it/rootguard/issues/132)).
- Router-independent reverse-DNS host discovery through the same normalized
  draft contract: operators provide canonical private IPv4 or unicast IPv6
  CIDR prefixes, Core performs bounded PTR lookups for at most 256 unique
  addresses with 16 workers and a shared 15-second deadline, and the WebApp
  reports partial lookup failures while keeping all successful results
  unselected. IPv4 and IPv6 results merge into typed local zones only through
  the existing preview, validation, activation, history, and rollback path
  ([rootguard#184](https://github.com/foxly-it/rootguard/issues/184)).
- Router import UI ("Router import" card on Unbound's Local DNS &
  forwarding tab): the first UI for the typed `LocalZone`/`LocalHost`
  model from #129 - discovered hosts start unselected, hostnames are
  editable per row before import, and selection merges into a named zone
  through the same preview → validate → activate lifecycle as every other
  guided Unbound setting. Live-verified end to end against a real
  FRITZ!Box: discovered 47 real hosts, renamed one, added two to a new
  `home.lab.` zone, and confirmed the generated `local-zone`/`local-data`
  directives through a real server-side `unbound-checkconf` pass. The
  frontend `UnboundSettings` type never got `local_zones` added when #129
  shipped (backend-only scope at the time) - added here along with
  `UnboundLocalZone`/`UnboundLocalHost`. Duplicate-hostname detection is
  scoped to the target zone itself
  ([rootguard#137](https://github.com/foxly-it/rootguard/pull/137)).
- `UnboundGuidedZones.tsx` migrated off its old JSON-in-a-comment scheme
  (parsed out of a marker block inside the custom expert config) onto the
  same typed `LocalZone`/`LocalHost` model and `fetchUnboundSettings`/
  `previewUnboundSettings`/`updateUnboundSettings` lifecycle as router
  import - both guided surfaces now write the same field, so the
  cross-system duplicate-hostname gap noted above no longer exists. Per
  the decision in
  [issue #131](https://github.com/foxly-it/rootguard/issues/131), CNAME
  records and per-host TTL are dropped from the guided UI entirely - the
  typed model was never scoped to support them; that rare case routes to
  the unrestricted expert editor instead. Live-verified end to end:
  created a zone with an IPv4+IPv6 PTR-enabled host through the wizard,
  previewed, activated, and confirmed real `dig` resolution for the A,
  AAAA, and both PTR records against the running resolver
  ([rootguard#167](https://github.com/foxly-it/rootguard/pull/167)).
- Guided resolver protocol mode defaults to IPv4 and offers dual-stack or
  IPv6-only operation only after the running Unbound container reaches an
  authoritative root server over IPv6. Core repeats this decision server-side
  during activation, renders `do-ip4`, `do-ip6`, and `prefer-ip6`, blocks expert
  duplication, and keeps preview, checkconf, history, and rollback intact.
- Client CIDR policy intentionally remains owned by AdGuard Home: Unbound is an
  internal resolver reached by AdGuard, so a separate end-client access editor
  in RootGuard would be ineffective duplication.
- Unbound information architecture split into accessible Overview, Resolver,
  Local DNS, and Advanced tabs. The landing view now shows only configuration
  status, profile, versions, extensions, and on-demand diagnostics; cache
  tuning, live files, expert rules, and rollback use progressive disclosure.
  Tabs support arrow-key navigation, clear focus states, responsive overflow,
  and labelled tab panels without changing the validation and rollback paths.
- Guided Unbound text, number, and select controls share the Setup page's
  consistent field height, surface, border, typography, and visible focus
  treatment across local zones, forwarding, private domains, and resolver
  tuning. Stack runtime badges center their pulsing indicator independently of
  browser text baselines for stable Safari rendering.
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
- The bilingual login includes local password recovery through a separate
  installation recovery key. Successful resets persist only a salted
  PBKDF2-SHA256 verifier in the restricted session volume and invalidate every
  active session. The recovery key neither creates a session nor crosses the
  WebApp/Core trust boundary; installations without one receive the explicit
  local `.env` recovery procedure instead of a misleading email flow.
- The logged-in operator can rename their account and/or set a new password
  from the WebGUI itself (`POST /api/auth/account`, user menu → "Account
  settings"), confirming with the current password rather than the recovery
  key. This reuses the recovery flow's existing PBKDF2/`credentials.json`
  persistence (previously only ever written by password recovery), now
  extended with a `Username` field so a rename also survives a restart.
  Unlike recovery - a locked-out escape hatch that always wipes every
  session - this keeps the calling session alive (its stored username
  updated in place) and only invalidates every *other* session
  ([rootguard#329](https://github.com/foxly-it/rootguard/issues/329)).
- Lightweight vector RootGuard shield replaces the former embedded bitmap
  asset. The global header now identifies the current page next to the brand;
  the application shell adds a keyboard skip link, labelled main navigation,
  focusable content target, and responsive page-name treatment. On desktop the
  primary sidebar can now collapse to its Lucide symbols, retains accessible
  labels and hover/focus tooltips, and remembers the local preference; mobile
  navigation remains fully labelled.
- The next WebGUI development slice is specified with verifiable acceptance
  criteria for a consistent utility header, local bilingual settings search,
  System/Light/Dark appearance, a bottom-anchored sticky sidebar control, and
  keyboard, screen-reader, zoom, responsive, and WCAG 2.2 AA validation.
- RootGuard Core now publishes a contributor-facing reference for every current
  internal API route. It identifies `/api/health` as the sole unauthenticated
  endpoint, documents the bearer-token boundary and route groups, and uses only
  representative sanitized request and response shapes.
- The standalone Unbound image now documents a rootless Podman quick start with
  loopback-only TCP/UDP publication and persistent configuration and trust-anchor
  volumes. The instructions were verified on macOS/arm64 with rootless Podman
  6.0.2: signed recursion carried the `ad` flag, broken DNSSEC returned
  `SERVFAIL`, and the same volumes remained valid across container recreation.
- Stack & Updates page for allowlisted AdGuard Home and Unbound lifecycle
  control. Update checks compare pulled and running image IDs; installation is
  asynchronous and persistent, creates protected data backups, replaces one
  service, performs DNS-chain health checks, and automatically restores the
  previous image and backup on failure.
- Stack Center runtime cards expose bounded Docker metadata for each DNS
  service: plain-language state guidance, health result, image version and
  immutable ID, start time, restart count, and actually published ports.
  Raw daemon output remains outside this endpoint.
- Container health reporting distinguishes an absent Docker healthcheck from
  a genuinely unknown inspection result. The official AdGuard Home image
  therefore appears as a normally running service without a configured Docker
  healthcheck instead of producing a misleading operator warning; starting,
  unhealthy, stopped, and indeterminate states retain their warning severity.
- The dedicated Logs & Diagnostics sidebar page loads the selected allowlisted
  service and offers local text/severity filters, optional ten-second refresh,
  and a downloadable privacy-safe report. Every request remains limited to 100
  lines from the previous 30 minutes and 64 KiB. Core removes control
  characters and redacts common authorization, token, password, secret, and
  API-key patterns; the browser cannot choose arbitrary containers or paths.
  Log reads use their own fixed five-container allowlist. The lifecycle-action
  allowlist remains separately restricted to AdGuard Home and Unbound, so
  exposing Core/WebApp/Updater diagnostics does not permit browser-controlled
  lifecycle actions against the control plane
  ([rootguard#201](https://github.com/foxly-it/rootguard/pull/201),
  [rootguard#210](https://github.com/foxly-it/rootguard/pull/210)).
- Separate internal control-plane updater for allowlisted Core and WebApp
  images. It remains alive while both containers are replaced as a pair,
  persists status outside either target, verifies exact image IDs and both
  health endpoints, and pins both previous images if either check fails.
  Browser requests cannot supply images, services, or Compose arguments.
- Updater source and history live in `rootguard-updater/`, a monorepo
  directory since the [monorepo migration](#delivered-and-verified) (formerly
  an independently versioned component repository pinned as a Git submodule;
  full history preserved). Its path-filtered CI (`ci-updater.yml`) runs
  tests, vetting, and `amd64`/`arm64` image builds on every change under that
  directory; `release-alpha.yml` builds and pushes its versioned release
  image alongside the other three components under one shared version. Its
  `latest` and commit tags were republished successfully as a
  multi-architecture manifest on 2026-07-29.
- The Updater CI also runs two real Docker scenarios with old and new
  Core/WebApp fixture images. It verifies both running image IDs after a paired
  update, then introduces an HTTP-503 WebApp candidate and proves that both
  previous image IDs are restored with a persisted `rolled_back` outcome.
- Bilingual project website deployed with enforced HTTPS at
  `https://rootguard.foxly.de`.
- GitHub-backed project overview on the website with current version, open
  pull requests, recent commits, and a release update log. The Pages workflow
  refreshes its public data during every deployment and every six hours, with
  a checked-in local fallback.
- Reduced public landing page focused on a plain-language product explanation,
  a Compose quick start, three user benefits, and a compact project status.
  Technical depth stays in the documentation, whose table of contents highlights the
  section currently in view. The dashboard header preview mirrors the live
  WebApp's Lucide symbols for endpoint, services, DNSSEC, filter chain, CPU,
  memory, query, blocked-query, and filter-rate cards instead of substituting
  unrelated text glyphs.
- Public alpha `v0.1.0-alpha.6` is published around a single
  `compose.alpha.yaml` that contains no local build contexts and pulls one
  named RootGuard version for Core, WebApp, Updater, and Unbound.
  `release-alpha.yml`'s publish matrix builds and pushes `amd64`/`arm64`
  images for all four components directly (previously only Core; WebApp,
  Updater, and Unbound required separately tagged pushes to their own
  repositories before the monorepo migration), after which the same workflow
  smoke-tests the published Compose through the real guided AIO installation,
  DNS resolution, and DNSSEC rejection path.
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
  capabilities, including the delivered privacy-preserving live stack and
  AdGuard Home metrics.
- Consistent public-site header navigation keeps Docs and quick start directly
  reachable and groups Wiki, project status, and roadmap under Project.
  Keyboard-friendly native dropdowns close on outside click or Escape, and the
  language switch is integrated into the navigation surface. A dedicated
  GitHub icon beside the navigation links directly to the repository.
  RootGuard now carries its own bilingual imprint and privacy notice
  tailored to the static GitHub Pages deployment instead of redirecting legal
  links to the Foxly blog. The off-topic "Foxly Tools" dropdown (MOTD, AdGuard
  Home Updater) was later dropped from the primary nav on every page -
  `tools.html` itself is untouched and still reachable via the footer link.
  The hero dashboard mockup was rebuilt to match the shipped WebGUI chrome
  (collapsed icon-rail sidebar, appbar search/theme/user controls, a Runtime
  services panel next to the data-flow panel), and a fixed jump-to-top button
  (bottom right, appears past a scroll threshold, `prefers-reduced-motion`
  aware) was added site-wide.
- Detailed Unbound configuration ownership and priority plan in
  `docs/unbound-configuration-roadmap.md`: fixed secure base, typed guided
  values, guarded expert directives, and permanently blocked browser controls.
- Repository-wide AGPL-3.0-or-later licensing for the main project, Core,
  WebApp, Unbound, and Updater. A separate trademark notice keeps RootGuard and
  Foxly IT names and visual identity outside the software license and asks
  modified distributions to use distinct branding.
- Formal keyboard/screen-reader re-verification of every WebGUI workflow,
  closing 0.5's last two accessibility items. The 2026-08 baseline (PR
  #110, closing #109) axe-core-scanned exactly 5 pages in one session and
  explicitly scoped out error states and the full Setup wizard; three
  pages shipped since then were never scanned at all (Login, Backups,
  Logs & Diagnostics). This pass: automated axe-core (wcag2a/aa,
  wcag21a/aa, wcag22aa) across all 8 current routes x light/dark (30 scan
  combinations, plus interactive states - the user menu, the
  session/audit panel, the login recovery sub-flow), scripted keyboard
  verification of the full login/recovery flow, three independent
  `ContentModal` focus traps (session/audit panel, AdGuard's filter-test
  dialog, the existing search modal), and the Zones tab's four guided
  workflow wizards. Zero violations remain after fixes. Three real bugs
  found and fixed, beyond re-confirming what already worked:
  - Collapsed-sidebar sub-nav links (`SidebarLayout.tsx`'s `nav-subitems`,
    used by 3 of Unbound's 4 tabs) had no accessible name once the label
    span was hidden via `.sidebar-collapsed .nav-subitem-label{display:none}`
    - the top-level nav items already had an `aria-label` fallback for
    this exact state, the nested sub-items never got the same treatment.
  - The Overview tab's own two sectioned panels (`unbound-section-overview-diagnostics`,
    `-path-diagnostics`) never appeared in the sidebar sub-nav at all -
    `sectionsFor()` had cases for `resolver`/`zones`/`advanced` but no
    `overview` case, silently falling through to an empty list.
  - Closing the session/audit-log modal (`SessionsModal`) dropped focus to
    `<body>` instead of the user-menu trigger: selecting "Active sessions"
    closes the dropdown (`setOpen(false)`) in the same click handler that
    opens the modal, so by the time `ContentModal`'s focus-effect runs,
    the dropdown item it would have auto-captured is already unmounted.
    The exact same async-unmount race `returnFocusTo` was built for in
    PR #110's own follow-up (Stack's log viewer) - that prop just had no
    live caller left once Logs became its own page (PR #201). Wired
    `SessionsModal` through to the trigger ref `UserMenu` already holds.
  - Two further WCAG AA contrast regressions, the same class of bug as
    PR #110's own fixes but never scanned since the affected components
    didn't exist yet: `ContentModal`'s shell is intentionally always dark
    "like a code block" regardless of page theme (an explicit, documented
    decision) - but content rendered inside it that reaches for the
    shared semantic tokens (`SessionsModal`'s session/audit lists) still
    theme-flipped with the page, reading unreadable once the page itself
    was in light theme. Fixed by re-pinning the tokens - including the
    legacy `--rg-*` aliases, which are declared once at `:root` and don't
    recompute for descendants just because a nested element overrides
    the token they reference - to their dark-theme values inside
    `.content-modal`'s own scope, keeping the shell exactly as
    theme-invariant as designed while making token-based content honor
    that same invariance. Separately, two guided-workflow "active" state
    buttons (`.private-domain-add`, the router-import method selector)
    dropped to 3.9:1 because they sit inside their own `--info-soft`
    tinted card, compositing two translucent layers instead of one -
    switched their active-state text to the high-contrast text token,
    since the border/background already signal the active state.
  Also translated 6 hardcoded German fallback error strings found along
  the way (AdGuard/Stack error paths, an Unbound restore confirmation)
  that surfaced in English-locale sessions
  ([rootguard#221](https://github.com/foxly-it/rootguard/issues/221)).
- Rate limits and audit events for destructive actions beyond the
  authentication surface: `rootguard-webapp/backend/internal/httpapi/
  destructive.go`'s `guardDestructive` wraps the ~16 mutating webapp routes
  that proxy to Core (Unbound settings/history/custom/import/diagnostic-
  logging, service start/stop/restart and updates, backup settings/export/
  restore, cleanup, control-plane update install, installation deploy,
  AdGuard bootstrap and filtering) with the same `SessionAuth`-owned
  sliding-window limiter and audit log the login/recovery surface already
  used - a single shared budget (30 requests / 5 minutes) across every
  destructive route rather than one per route, since the goal is bounding
  what a single session can do overall. All destructive routes are thin
  proxies to `rootguard-core` (which itself has no rate-limit/audit
  infrastructure), so guarding the webapp's one browser-facing entrypoint
  covers every case without needing any Core-side change. Live-verified on
  the Debian LXC test stack: the 31st request in a 5-minute window to any
  guarded route returns `429` while unrelated GET routes stay unaffected,
  and the shared budget blocks a *different* guarded route once exhausted -
  confirming it's a per-session budget, not per-endpoint. `GET /api/auth/
  audit` gained `<action>_success`/`_failure`/`_rate_limited` events with a
  new optional `detail` field (method + path) alongside the existing auth
  events ([rootguard#219](https://github.com/foxly-it/rootguard/issues/219)).

## Current development slice

Trustworthy Stack Center and production visibility:

- real service state, health, image reference, immutable image ID, start time,
  restart count, and published ports, presented with plain-language guidance;
- completed data-plane updates and paired Core/WebApp updates through the
  separate helper; the public beta Compose now records and pins every
  RootGuard and AdGuard multi-architecture manifest digest;
- bounded update and rollback history survives restarts and is shown in the
  Stack Center together with each automatic cleanup result;
- post-update cleanup retains the active and previous successful image, removes
  only older image IDs recorded by RootGuard, and considers only unused volumes
  carrying the explicit `io.rootguard.cleanup=true` label;
- the same server-side allowlist powers an optional manual preview with
  per-resource and total reclaimed-space estimates; a confirmed cleanup
  recomputes eligibility and records the result in bounded history;
- safe start, stop, and restart controls with clear impact;
- bounded, redacted on-demand service logs and actionable failure states;
- Stack Center runtime inspection now covers Core, WebApp, the independent
  Updater helper, AdGuard Home, and Unbound through Core's fixed allowlist. It
  exposes OCI version, revision, creation time, source, and manifest-digest
  pinning, while the bilingual UI visibly distinguishes immutable references,
  mutable tags, local builds, and complete, partial, or unavailable metadata.
  OCI labels remain provenance hints, while immutable Core/WebApp releases are
  now checked cryptographically with pinned Cosign against SLSA provenance,
  the exact GitHub repository/workflow identity, GitHub Actions OIDC issuer,
  and Sigstore transparency data. Verified, missing, invalid, unavailable, and
  non-applicable results remain distinct and are cached for ten minutes.
  Alpha 4 predates the signed publication step and is therefore expected to
  report a missing attestation; RootGuard does not retroactively claim build
  provenance. The next coordinated release is the first verifiable release.
- typed, bilingual AIO installation diagnostics for invalid host addresses,
  missing Compose, occupied DNS ports, failed image pulls, and interrupted
  deployment recovery; raw technical details remain available on demand.
- guided AdGuard Home release-channel selection with Stable as the
  backward-compatible default and an explicit bilingual Beta warning. Core
  accepts only the `stable`/`beta` enum and resolves both to administrator-set,
  allowlisted image references; the public beta's AdGuard Beta reference is
  pinned to its verified multi-architecture manifest.
- guarded public clean-install verifier shared by Docker Desktop and
  native GitHub-hosted Linux `amd64`/`arm64` jobs; it refuses existing
  RootGuard resources, validates login, AIO deployment, recursive DNS and
  DNSSEC rejection, then removes only resources created by the test. Docker
  Desktop `arm64` and both native Linux jobs passed on 2026-07-28; the Linux
  evidence is Actions run `30353823582`.
- fixed Dashboard service KPI: the active/total count now derives from the
  live five-service allowlist response instead of a hardcoded value, so a
  fully healthy stack shows `5 / 5` rather than a stale `X / 2`
  ([rootguard-webapp#50](https://github.com/foxly-it/rootguard-webapp/pull/50)).
- semantic design tokens and a System/Light/Dark theme system
  (`localStorage`-persisted, zero-FOUC) for the app shell and first screen -
  header, sidebar, buttons, cards, status indicators, and login. Fixed 8
  previously-referenced-but-undefined CSS custom properties and a dead
  duplicate `.sidebar` rule along the way. Every light-theme text/background
  pair is WCAG AA contrast-verified. Dashboard, Setup, Stack Center,
  AdGuard, and the Unbound settings pages are also migrated
  ([rootguard-webapp#54](https://github.com/foxly-it/rootguard-webapp/pull/54),
  [#56](https://github.com/foxly-it/rootguard-webapp/pull/56),
  [#58](https://github.com/foxly-it/rootguard-webapp/pull/58)). Code/config
  viewers (the Unbound expert editor, live-config viewer, log and
  diagnostic panels) intentionally stay dark under both themes, like a
  code block. The theme-tokens roadmap item is now fully delivered.
- header reworked as a coherent utility bar: language, theme, sign-out,
  GitHub, and Docs now share one visual language (height, border, radius,
  icon size, hover, focus-visible), including a fix for GithubIcon/DocsIcon
  rendering unconstrained at their raw 22px SVG size
  ([rootguard-webapp#60](https://github.com/foxly-it/rootguard-webapp/pull/60)).
- language, appearance, and sign-out consolidated into an accessible user
  menu (trigger + panel with explicit System/Light/Dark options, the
  language select, and sign-out). GitHub/Docs stay direct on desktop and
  fold into the same menu on narrow viewports
  ([rootguard-webapp#62](https://github.com/foxly-it/rootguard-webapp/pull/62)).
- global, local-only search: a hand-curated index (~45 entries) covering
  all pages, the 4 Unbound tabs, and the meaningfully distinct guided
  settings/actions across Unbound, Setup, Stack Center, and AdGuard,
  including technical directive keywords (`qname-minimisation`,
  `prefetch-key`, `local-zone`, `forward-zone`,
  `90-rootguard-custom.conf`, ...). Opens from the header search chip,
  `S` (ignored while typing), or `Ctrl`/`Cmd`+`K`; full keyboard
  navigation, Escape restores focus to the trigger. Selecting a result
  navigates to the matching page only - landing on the exact
  originating tab/section is the next roadmap item, since Unbound's tab
  state isn't currently URL-addressable
  ([rootguard-webapp#64](https://github.com/foxly-it/rootguard-webapp/pull/64)).
  Unbound's active tab is now URL-addressable (`/unbound/:section`,
  derived from a route param, falls back to Overview on an invalid or
  missing value, deep-linkable and reload-safe); every Unbound search
  entry's route now points at its actual tab instead of the bare page,
  so selecting e.g. "expert editor" or "forward-zone" lands directly on
  the Advanced or Local DNS & forwarding tab. Sidebar active-state
  highlighting needed no change, since `NavLink` already does
  non-exact prefix matching
  ([rootguard#106](https://github.com/foxly-it/rootguard/pull/106)).
  Every major sub-section across the Resolver, Local DNS & forwarding,
  and Advanced tabs (plus the Overview diagnostics block) now has an id;
  search entries carry a `#unbound-section-...` hash alongside their tab
  route, and landing on one scrolls to and focuses that element, opening
  it first if it's a collapsed panel like the version history. A sticky
  `UnboundSectionNav` bar under the tab strip lists the current tab's
  sections (only rendered when a tab has 2+ of them) with scroll-spy
  highlighting of whichever one is currently in view. Getting the
  scroll-spy right needed two fixes beyond the initial version:
  IntersectionObserver callbacks report only the elements whose state
  *changed*, not a full snapshot of everything being observed, so the
  latest state per element has to be tracked across calls rather than
  read fresh from each individual callback; and the observer must not
  react at all until the user has scrolled themselves, since a section
  can be positioned exactly where a deep link or search result put it
  and *still* not count as "intersecting" a percentage-based trigger
  band - not a race condition, just the real geometry for whichever
  section happens to be last on a tab, since nothing follows it to make
  room to scroll it any further up. Fixed at the layout level instead of
  purely in JS: tab panels now carry generous `padding-bottom`, the
  standard technique for making every anchor on a page - including the
  last one - reachable at the top of the viewport
  ([rootguard#107](https://github.com/foxly-it/rootguard/pull/107)).
- removed the four decorative bordered-circle pseudo-elements on the
  Dashboard/Setup/Unbound/Stack hero cards (user-reported: too
  prominent in light mode, distracting in dark mode) and fixed the
  search control's header integration - `vdev` no longer splits the
  Search/GitHub/Docs/account button group, and the search trigger now
  shows a visible `S` key badge instead of only a hover tooltip
  ([rootguard-webapp#66](https://github.com/foxly-it/rootguard-webapp/pull/66)).
- sidebar collapse control moved to the sidebar's bottom edge; new desktop
  sessions now default to the collapsed icon view, and an existing
  explicit local preference always wins
  ([rootguard-webapp#68](https://github.com/foxly-it/rootguard-webapp/pull/68)).
- fixed app-shell layout: only the main content pane scrolls now, so the
  sidebar (and its collapse control) stay visible on long pages
  regardless of scroll position; the nav item list gets its own internal
  scroll if it ever outgrows the viewport. A same-PR follow-up fixed a
  regression this introduced (a CSS overflow-axis coupling quirk had
  clipped the collapsed-sidebar hover tooltips and shown a stray
  scrollbar); tooltips now render via a `document.body` portal instead of
  a clipped CSS `::after`
  ([rootguard-webapp#70](https://github.com/foxly-it/rootguard-webapp/pull/70)).
- fixed washed-out light mode (user-reported): every `box-shadow` was a
  literal near-black `rgba(0, 0, 0, X)` hand-tuned against the dark
  theme, which read as a muddy grey halo instead of a crisp lift once
  reused unchanged against light mode's near-white surfaces. Added
  theme-aware `--shadow-ink`/`--shadow-scale` tokens (dark: unchanged;
  light: cooler ink at ~40% intensity) and rewrote the 15 affected
  declarations across 11 stylesheets to reference them, keeping each
  component's hand-tuned offset/blur/spread exactly
  ([rootguard-webapp#72](https://github.com/foxly-it/rootguard-webapp/pull/72)).
- roadmap audit: four already-shipped items were unchecked because they
  predate this file's current level of detail. Re-verified directly
  against code/config rather than trusting the earlier prose: bounded/
  redacted logs (`rootguard-core/internal/stack/logs.go` - 100 lines/30
  minutes/64 KiB, common-credential redaction, all five allowlisted
  services); real component versions/digests/uptime/health (Stack Center
  runtime cards - version, image ID, build revision, start time, restart
  count, provenance, per service); admin credential recovery
  (`rootguard-webapp` `/api/auth/recovery`, PBKDF2-hashed token compare);
  and the security policy (`SECURITY.md` - GitHub Private Vulnerability
  Reporting link, report contents, supported-versions policy). Left
  unchecked as ambiguous rather than guessing: "shared guided workflow"
  (the pattern repeats per guided area, not confirmed as one reusable
  component), "show AdGuard state together" (the information exists but
  spread across Dashboard/AdGuard page/Stack Center, not one place), and
  "cross-service diagnostics Client→AdGuard→Unbound→DNSSEC" (Dashboard's
  Data Flow card shows this exact chain, but as status display rather
  than diagnostics).
- Unbound expert editor: added a fullscreen mode and an inline panel
  showing the immutable base configuration's already-active directives, so
  experts no longer need a separate popup to cross-reference them while
  writing overrides. Fullscreen renders through a `document.body` portal
  rather than a plain CSS toggle - the parent `.unbound-page` carries a
  permanent `animation: ... both` that leaves a non-`none` transform
  applied after the entrance animation ends, which turns it into a
  containing block that would otherwise trap a `position:fixed` panel
  inside the page instead of the viewport
  ([rootguard-webapp#74](https://github.com/foxly-it/rootguard-webapp/pull/74)).
  The base-config panel was initially a collapsed `<details>` disclosure;
  feedback was that a power user needs to see what's already active at a
  glance, without an extra click, so it was changed to a permanently
  visible, height-capped, scrollable panel
  ([rootguard#101](https://github.com/foxly-it/rootguard/issues/101)).
- Monorepo migration: merged the full commit history of `rootguard-core`,
  `rootguard-webapp`, `rootguard-unbound`, and `rootguard-updater` into this
  repository as top-level directories via `git subtree`, removed
  `.gitmodules`. Motivation: as a solo maintainer, every change previously
  needed two PRs (component repo, then a submodule-bump follow-up here) and
  cutting a full alpha release required manually pushing matching version
  tags to four separate repos - `release-alpha.yml` already built all four
  images under one shared version number, just sourced from separately
  tagged repos instead of one commit. Path-filtered CI workflows
  (`ci-core.yml`, `ci-webapp.yml`, `ci-unbound.yml`, `ci-updater.yml`) replace
  the four repos' own workflows, each triggering only on changes under its
  own directory; `release-alpha.yml`'s publish matrix now builds and pushes
  all four images directly from one workflow instead of only Core, with the
  other three verified to already exist. The four original repositories are
  archived (read-only, history and issue/PR links preserved) rather than
  deleted.
- RootGuard Blockpage: a new `rootguard-blockpage/` monorepo directory - a
  static, AGPL-3.0-or-later nginx image. Guided setup configures AdGuard
  Home's `blocking_mode: custom_ip` automatically, optional and enabled by
  default with its own preflight check and a dedicated Setup toggle for
  users who explicitly want it disabled; `blocking_ipv6` is set to `::1`
  since AdGuard's API requires a valid IPv6 value even for RootGuard's
  IPv4-only configuration. The container runs `read_only: true`,
  `cap_drop: [ALL]` with only `CHOWN`/`SETUID`/`SETGID` added back for
  nginx's own root-master-to-non-root-worker privilege drop, and a fixed
  internal network address (auto-assigned addresses raced with Unbound's
  static one). End-to-end verified on a live stack: a blocked domain
  resolves to the blockpage's address, and an HTTP request with that
  domain's Host header renders the actual page
  ([rootguard#98](https://github.com/foxly-it/rootguard/pull/98)). The
  first design leaned on the WebGUI's own dashboard visual language
  (hero/KPI grid, red "danger" framing), which read poorly and didn't fit
  a landing page shown to end users rather than an admin. Rebuilt around
  AdGuard Home's own brand green as a positive "protection working"
  signal, with a restrained, high-contrast, single-accent, non-nested-card
  layout; verified in both themes plus the `/info/` explainer page
  ([rootguard#100](https://github.com/foxly-it/rootguard/pull/100)).
- Accessibility verification pass (the 0.1 navigation slice's exit gate):
  automated axe-core scans (wcag2a/aa, wcag21a/aa, wcag22aa) across every
  page, both themes, both languages, and several interactive states (search
  modal, user menu, expert editor, expert editor fullscreen, a
  `ContentModal`) - 28+ combinations, all clean after fixes, plus the same
  at a 375px mobile viewport. Found and fixed: four design tokens
  (`--info`/`--accent`/`--success`/`--warning`) that failed WCAG AA 4.5:1
  once composited over a real card background even though they read as
  passing against the flat `--surface` color alone; a dark-mode-only code
  label whose background wasn't opaque enough; a blanket `opacity: 0.72` on
  a whole card that silently broke its descendant text's contrast (CSS
  opacity composites a subtree as one unit - a child can't restore full
  contrast with its own `opacity: 1`); eight scrollable `<pre>` blocks
  (generated-config previews, the Stack log viewer, Setup's diagnostic
  detail) with no keyboard access; and `ContentModal` not trapping focus,
  letting Tab walk out of an open dialog into the covered background page.
  Also scripted-verified: logical Dashboard focus order, `S`/`Ctrl`+`K`
  open search (ignored while typing in a field), `Escape`
  closes-and-restores-focus, Unbound tabs respond to Arrow/Home/End, the
  user menu opens with `Enter`, sidebar tooltips appear on keyboard focus
  (not just hover), and no horizontal overflow at 640px (200%-zoom
  equivalent) or 320px (WCAG 1.4.10's 400%-zoom reflow width) on any tested
  page ([rootguard#109](https://github.com/foxly-it/rootguard/pull/109)).
- Blockpage preview + a real contrast bug fix: Setup's blockpage toggle now
  links to a static preview of the page (bundled into the WebApp at
  `public/blockpage-preview/`, realistic example domain/time/IP instead of
  the real page's dynamic `meta.js` fetch - it's a manual snapshot copy,
  not an automated cross-component build step, so it needs a matching
  manual update if the real blockpage's design changes later). Running an
  axe-core scan against that copy surfaced a genuine WCAG AA contrast
  failure in the actual shipped `rootguard-blockpage` itself, not just the
  preview: light-theme `--accent` (`#39ac31`) was too light for both
  white-text-on-green buttons (2.94:1) and green-text-on-white links
  (2.82:1), against a 4.5:1 requirement. Neither the original blockpage
  work above nor the separate WebGUI-wide accessibility audit had scanned
  `rootguard-blockpage` itself, since it's a separately deployed component
  with its own design tokens - each had verified its own surface only.
  Darkened `--accent`/`--accent-border`; dark theme was already passing
  and is untouched. Re-verified with axe against the real (rebuilt, not
  copied) Docker image on both light and dark
  ([rootguard#112](https://github.com/foxly-it/rootguard/pull/112)).
- Blockpage reason-lookup backend (part 1 of 3 toward showing the real block
  reason instead of a static always-checked list, see ROADMAP): the
  blockpage's own nginx now proxies `/api/reason` to AdGuard's
  `check_host`, using `$host` only - never a client-supplied parameter, so
  it can't become a free "is domain X blocked" probe against the reader's
  own AdGuard instance - and a hardcoded upstream path, so a compromised
  request here can't reach any other AdGuard admin endpoint. Rate-limited
  (`limit_req`) and short-TTL cached per host to bound both single-client
  abuse and duplicate load from many clients hitting the same blocked
  domain. AdGuard requires Basic-Auth for `check_host`; the blockpage never
  holds the raw admin username/password - Core derives a `base64(user:pass)`
  token after AdGuard bootstrap succeeds and publishes it to a dedicated
  `rootguard-adguard-auth` volume (same external-volume pattern as
  `rootguard-unbound-config`), read-only, then re-runs the blockpage's
  self-contained entrypoint script in place (not the stock `envsubst` hook -
  see script comment for the fork/export pitfall that motivated a custom
  one) and reloads nginx - deliberately not a container restart, which
  would tear down and re-create blockpage's network endpoint and race
  AdGuard's dynamically-assigned address for the static IP blockpage needs
  (found by testing this on a live deploy: the restart intermittently lost
  that race with "address already in use"). Any
  upstream failure (AdGuard down, bad/missing token, timeout) collapses to
  one uniform `{"available":false}` response
  ([rootguard#117](https://github.com/foxly-it/rootguard/pull/117)).
- Blockpage real reason display (part 2 of 3): the four "why blocked" cards
  are now five, reflecting what AdGuard's `check_host` can actually
  distinguish - the previous four conflated two reasons (`FilteredBlackList`
  covers both ad/tracking and generic threat-list hits, indistinguishable
  from `check_host` alone) into a single honest "Filterliste" card, and
  added two that had no card at all: "Gesperrter Dienst"
  (`FilteredBlockedService`) and "Jugendschutz" (`Parental`). `meta.js`
  fetches `/api/reason` (2.5s client-side timeout on top of the proxy's own
  2s) and highlights the one matching card, dimming the rest; any failure -
  offline, AdGuard down, timeout - leaves every card in its plain default
  state, which is also exactly what a JS-disabled or pre-fetch page already
  renders, so there's no separate fallback path to maintain
  ([rootguard#118](https://github.com/foxly-it/rootguard/pull/118)).
- Blockpage visual redesign (part 3 of 3): brings back cards and motion
  without repeating what PR #100 already rejected (a heavy, admin-dashboard-
  like hero/KPI treatment) - the *mechanics* are borrowed from the main
  WebApp (individual cards with a colored top-accent bar instead of one
  bordered box, a two-token `--shadow-ink`/`--shadow-scale` pattern instead
  of flat per-theme rgba() shadows, a staggered fade-free rise-and-scale
  entrance animation with a `prefers-reduced-motion` kill-switch that didn't
  exist here before), but not its *color language* - AdGuard green stays
  the only accent, no WebApp teal or danger red. The matched reason card
  (from the previous entry) gets the raised shadow and colored top border;
  the rest stay flat. The flat filled shield mark and the single reused
  checkmark icon are now stroke-based, with a distinct icon per category,
  closer to the WebApp's lucide-react line-icon language, while the
  brand mark (`rootguard-icon.svg`) is untouched. `rootguard-webapp`'s
  manually-synced preview copy (`public/blockpage-preview/`) updated to
  match, including a hardcoded example of one matched card since that copy
  has no live request to describe
  ([rootguard#119](https://github.com/foxly-it/rootguard/pull/119)).
- Blockpage narrative redesign: replaced the five-card "why blocked" grid
  with a single-sentence headline + lead ("Diese Seite ist blockiert -
  {reason}." / "RootGuard hat den Zugriff auf {domain} blockiert, weil
  ..."), reusing the same `/api/reason` data from the previous change but
  presenting it as a sentence instead of a highlighted card among four
  dimmed ones. Added a commissioned fox mascot illustration (AI-generated,
  background-removed and cropped locally) in the hero, matching RootGuard's
  green accent and carrying a root-vein motif on its tail/shield to tie
  "fox" (Foxly IT) to "root network" without being literal. `meta.js`'s
  `REASON_INFO` map now holds a short label plus a full sentence clause per
  AdGuard reason, replacing the previous five-entry DOM-attribute lookup.
  Unmatched or failed lookups keep the original generic headline/lead - the
  same text that already ships as the default render, so there's still no
  separate fallback path to maintain. `rootguard-webapp`'s manually-synced
  preview copy updated to match, including the mascot asset
  ([rootguard#125](https://github.com/foxly-it/rootguard/pull/125)).
- Four small live-testing findings fixed in one session (2026-08-22):
  the AdGuard Home update-check pin (`ROOTGUARD_ADGUARD_IMAGE`/
  `ROOTGUARD_ADGUARD_UPDATE_IMAGE`) had been frozen at `v0.107.78` since
  alpha.5 - unlike RootGuard's own four component images, nothing
  refreshes this third-party pin automatically, so every install's
  AdGuard "check for updates" silently stayed up to date regardless of
  real upstream releases; bumped to the current stable `v0.107.79`
  ([rootguard#306](https://github.com/foxly-it/rootguard/issues/306),
  [rootguard#307](https://github.com/foxly-it/rootguard/pull/307)).
  The local dev `compose.yaml`'s `updater` service hardcoded
  `image: rootguard-updater:dev` with no env interpolation, unlike
  `core`/`webapp`, so a real `ROOTGUARD_UPDATER_IMAGE` pin in `.env` had
  no effect
  ([rootguard#304](https://github.com/foxly-it/rootguard/pull/304)).
  `.gitignore` had `.graphify-out/` (leading dot) instead of the tool's
  actual `graphify-out/`, so it was never ignored
  ([rootguard#305](https://github.com/foxly-it/rootguard/pull/305)).
  `compose.alpha.yaml`/`.env.alpha.example` renamed to
  `compose.release.yaml`/`.env.release.example` - phase-neutral, so it
  won't need to change again at the next phase transition; the
  updater container's own `ROOTGUARD_COMPOSE_FILE`/bind-mount
  self-reference was updated too, since that one drives the real
  control-plane update mechanism, not just docs. `README.md`/
  `site/index.html`/`site/docs.html`'s quick-start commands are
  deliberately left pinned to the old name under the already-published
  `v0.1.0-beta.3` tag, which doesn't contain the new one - they
  self-correct once the next release ships
  ([rootguard#308](https://github.com/foxly-it/rootguard/issues/308),
  [rootguard#309](https://github.com/foxly-it/rootguard/pull/309)).
- Stack Center's per-component attestation check now actually runs for the
  RootGuard Updater and Unbound, not just Core/WebApp: `attestationPolicies`
  has held real signing policies for all five components since #230, but
  `CheckStackAttestations` - the function `/api/services` actually calls -
  only ever invoked the check for `core`/`webapp`, hardcoding
  `updater`/`adguard`/`unbound` to `not_applicable` unconditionally.
  `adguard` correctly stays hardcoded (third-party image, no RootGuard
  policy); `updater`/`unbound` now go through the same live `cosign`
  verification as `core`/`webapp`. A new regression test confirmed to fail
  against the pre-fix code and pass against the fix
  ([rootguard#317](https://github.com/foxly-it/rootguard/issues/317),
  [rootguard#318](https://github.com/foxly-it/rootguard/pull/318)).
- "Auf Updates prüfen" now discovers real new RootGuard releases live
  instead of requiring an administrator to hand-pin the next image in
  `.env` first. Core (which has real outbound internet access) queries the
  public GitHub Releases API for `foxly-it/rootguard` (the full,
  newest-first `/releases` list, since every release here is created with
  `--prerelease` and is therefore invisible to `/releases/latest`), picks
  the newest tag matching RootGuard's own `v0.1.0-(alpha|beta).N`
  convention, and resolves it to
  `ghcr.io/foxly-it/rootguard-<component>:<version>` for Core, WebApp, and
  Unbound. Unbound's own updater (inside Core) resolves this directly;
  Core/WebApp's updater is the separate, network-isolated
  `rootguard-updater` component (`control` network, `internal: true` - no
  outbound internet by design), so Core resolves those two itself and
  passes the result through as a per-request `target_images` override on
  the existing check/update call - live verification on the dev/test LXC
  is what surfaced this network boundary, since the first implementation
  attempt had `rootguard-updater` try to query GitHub directly and failed
  with a DNS lookup error. The explicit "Control Plane
  aktualisieren"/per-service update button click remains the real
  authorization gate - only how the target image string is determined
  changed, from a static pin to live discovery; the existing pull +
  inspect + cosign-attestation-checked install/rollback machinery is
  unchanged. AdGuard (third-party, channel-based) and the RootGuard
  Updater's own image (self-update is a separate, harder problem - a
  process can't gracefully replace its own running container) stay on the
  static-pin path. A failed GitHub Releases lookup degrades silently to
  the existing static `ROOTGUARD_*_UPDATE_IMAGE` `.env` pin for that
  component rather than blocking the check/update - those pins become
  unused, not obsolete
  ([rootguard#319](https://github.com/foxly-it/rootguard/issues/319)).
- The RootGuard Updater can now update its own image, closing the one
  remaining gap #319/#320 explicitly left open: a running process can't
  gracefully replace its own container, so Core - which already holds the
  Docker socket and, since #320, real outbound internet access - manages
  this the same way it already manages AdGuard/Unbound, via a second
  `*updater.Manager` instance scoped to the control-plane `compose.yaml`/
  project `rootguard` (not Core's own generated data-plane compose file).
  New Core routes (`/api/updater-updates`, mirroring the existing
  `/api/updates` shape) are proxied by the WebApp backend the same way as
  every other update surface; the Stack Center "Control Plane" panel's
  updater row, previously a static "HELPER" label with no action, now
  carries a real update button. A busy-guard refuses to start the swap
  while the updater is itself mid a core/webapp check or update - not
  required for correctness (an interrupted operation already recovers
  cleanly on next start, the same recovery path proven for a real process
  kill in #225) but avoids an easily-avoidable, confusing failure. Live
  verification on the dev/test LXC surfaced a real filename mismatch that
  unit tests couldn't: Core's generic update manager always looks for a
  file literally named `compose.yaml` in whichever directory it's pointed
  at, but the public release stack's file is `compose.release.yaml` -
  fixed by mounting it at a renamed target path (`/opt/rootguard/
  compose.yaml`) for Core specifically, while the updater's own
  self-mount keeps its real name since it uses a literal full-path field
  instead of this fixed-filename join
  ([rootguard#321](https://github.com/foxly-it/rootguard/issues/321)).
- Three more real bugs found live while actually exercising updater
  self-update against published `v0.1.0-beta.5` images (the renamed-mount
  fix above was necessary but not sufficient):
  (1) Docker Compose resolves a *relative* bind-mount source against the
  directory of whatever `-f` file is being parsed, not the file's own
  location - so when Core (or the Updater) re-invokes `docker compose -f
  /opt/rootguard/compose.yaml ...` from inside its own container to
  recreate a sibling service, `core:`'s/`updater:`'s own `./compose.yaml`
  self-mount re-resolved against `/opt/rootguard` (a container-internal
  path, meaningless on the real host) and silently became an empty
  directory - corrupting the mount for every future recreation of that
  service, including a normal core/webapp control-plane update (#320),
  not just self-update. Fixed by switching to `${PWD}/compose.yaml` (a
  literal env-var substitution, immune to filesystem-relative
  resolution) with an explicit `PWD: "${PWD}"` passthrough on both
  services, in both `compose.yaml` and `compose.release.yaml`.
  (2) Live-discovered update targets are bare `repo:tag` references by
  design (#320); cosign attestation verification requires an explicit
  `@sha256:...` reference and reports `not_applicable` without one - so a
  self-updated (or live-discovery-updated) service only ever showed
  "Verified" by coincidence, never because the live-discovery mechanism
  itself produced a verifiable reference. Fixed with a `digestQualify`
  helper (mirrored in both updater implementations) that resolves the
  real digest via `docker image inspect` right after a successful pull
  and upgrades the target to `repo@sha256:...` before it's used.
  (3) The already-published `v0.1.0-beta.5` release-pin-refresh commit on
  `main` contained genuinely broken YAML: `release-alpha.yml`'s
  digest-pinning `sed` pattern didn't exclude `}` from its greedy match,
  and every `*_UPDATE_IMAGE` line except the new
  `ROOTGUARD_UPDATER_UPDATE_IMAGE` (intentionally unpinned to `:latest`,
  so its default had no pre-existing `@sha256:...` to stop the match at)
  happened to avoid the bug by luck. `docker compose -f
  compose.release.yaml config` failed outright as a result - fixed the
  regex and the already-broken committed line
  ([rootguard#323](https://github.com/foxly-it/rootguard/issues/323)).

The storage safety slice persists successful image history before deleting
anything. Cleanup retains the active and previous successful image and removes
only older IDs recorded by RootGuard. Global `docker system prune`, `docker
image prune`, and `docker volume prune` remain prohibited. Configuration,
AdGuard data, Unbound state, sessions, backups, and every unlabeled or foreign
volume remain protected. A cleanup that has nothing safely eligible records a
visible no-op instead of widening its scope.

The Stack Center also exposes this exact allowlist as a manual cleanup preview.
It shows Docker's rounded unique image-layer and volume-size estimates, keeps
unverifiable or in-use resources visibly protected, and accepts no browser-
supplied resource IDs. Confirmation triggers a fresh server-side scan before
removal and writes the result into bounded history
([rootguard#191](https://github.com/foxly-it/rootguard/pull/191)).

Core's pre-update AdGuard/Unbound backups are now bounded separately from image
and volume cleanup. The Backups page exposes recognized count and logical bytes
in total and per service, newest timestamps, and separately measured
unrecognized data. Operators can retain 2–50 restore points per service
(default 5). A lower value requires confirmation and pruning accepts only a
canonical timestamp/service directory with a manifest matching the allowlisted
service and container; unknown entries and symlinks remain untouched
([rootguard#189](https://github.com/foxly-it/rootguard/pull/189)).

Operators can now download a separate portable full backup as an interoperable
age-v1/scrypt encrypted archive. It packages checksummed RootGuard state plus
live AdGuard and Unbound persistent data from fixed server-side sources while
excluding sessions, external environment secrets, update restore points, and
temporary exports. Private plaintext staging is removed on every path and
data-plane updates cannot overlap export creation
([rootguard#193](https://github.com/foxly-it/rootguard/pull/193)). Clean-install
restore now repeats strict bounded archive/manifest verification, gates apply
on clean installation and Docker state, permits a re-preflighted target
address/port, restores into stopped fresh volumes, normalizes Unbound
ownership, and health-verifies the DNS chain. Failure cleanup removes new
Docker resources and restores prior local contents
([rootguard#199](https://github.com/foxly-it/rootguard/pull/199)). Transactional
snapshot verification remains the next 0.4 step.

Backup retention and portable export have their own authenticated Backups page
and sidebar entry; Stack & Updates remains focused on service lifecycle,
updates, rollback status, and safe Docker cleanup
([rootguard#195](https://github.com/foxly-it/rootguard/pull/195)).
The Backups page now distinguishes full encrypted recovery, RootGuard Unbound
bundle transfer, and traditional `unbound.conf` import, linking and indexing
each existing validation workflow directly
([rootguard#203](https://github.com/foxly-it/rootguard/pull/203)). Stack keeps
routine state and actions visible, collapses low-frequency release metadata,
aligns service actions, places history before maintenance, and no longer loads
the Docker cleanup inventory until requested
([rootguard#206](https://github.com/foxly-it/rootguard/pull/206)).

## Remaining production milestones

Cross-referenced against `ROADMAP.md` on 2026-08-11 (supersedes the previous
narrative version of this section, which had drifted from the actual
per-item roadmap state). Read `ROADMAP.md` itself for full item text and PR
references - this is the sequencing/prioritisation layer on top of it, kept
here specifically so a session can resume with "Mache weiter mit der
Entwicklung" without re-deriving what's next.

Milestone completion snapshot:

- **0.2 Unbound administration** - complete at the pre-1.0 scope. Access rules
  remain expert-only and a possible guided surface is deferred with the
  post-1.0 extension architecture. Import/export of the complete logical resolver
  configuration is done (`UnboundConfigTransfer.tsx`,
  `/api/unbound/export`+`/api/unbound/import*`). Hand-written `unbound.conf`
  import is done at full roadmap scope (`UnboundConfImport.tsx`, `import.go`'s
  classifier): fixed-base filtering/conflict detection, all scalar guided
  settings, forward-zone/local-zone/RFC1918-reverse-zone-policy/network-mode/
  resource-profile reverse-mapping, and expert-config adoption for everything
  else all work end to end. Scenario tests (home network, VLANs, split DNS,
  IPv6-only local records, broken upstreams, DNSSEC failures) are done too -
  a build-tag-gated Go integration suite
  (`scenario_integration_test.go`) that renders real `Settings` values
  through production `Settings.Render()` and verifies them against a live
  `rootguard-unbound` container with real `dig` queries, wired into CI as a
  new `scenario-tests` job in `ci-unbound.yml`
  ([rootguard#183](https://github.com/foxly-it/rootguard/pull/183)).
  The guided-zones frontend migration
  ([#131](https://github.com/foxly-it/rootguard/issues/131)), the read-only
  fixed-base display, and the shared draft→preview→activate workflow
  abstraction (`useUnboundDraftWorkflow`, migrated onto by four of the five
  places that pattern existed - main settings stays separate, a genuinely
  different shape) are all done now.
- **0.3 AdGuard integration** - **complete**. The last item, cross-service
  Client → AdGuard → Unbound → DNSSEC diagnostics, landed as a second "Path
  diagnostics" card on the Unbound Overview tab
  ([rootguard#160](https://github.com/foxly-it/rootguard/pull/160)): AdGuard
  is reachable by container name (`rootguard-adguard`) on the
  `rootguard-dns` network, and only `rootguard-unbound`'s own container has
  both DNS tooling and network line-of-sight to it, so the check reuses that
  container the same way Unbound's own resolution/DNSSEC checks already do -
  just targeting `rootguard-adguard:53` instead of Unbound's own port.
  AdGuard being unpinned on that network at the time was later found to be a
  real bug - see the 0.4 network-addressing fix below.
- **0.4 operations/backup/recovery** - **complete**. Clean-install full
  restore landed in
  [rootguard#199](https://github.com/foxly-it/rootguard/pull/199).
  Pre-update snapshot and post-update restore verification landed
  ([rootguard#223](https://github.com/foxly-it/rootguard/issues/223)): the
  separate *internal, automatic* pre-update snapshot Core takes before an
  AdGuard/Unbound image replacement (distinct from the operator-triggered
  portable backup above) now checksums every backed-up file, and a failed
  update's automatic rollback refuses a corrupted or partial snapshot
  instead of silently restoring it - proven against a real Docker container
  and a real `docker compose` image swap in a new `ci-core.yml` job, under
  its own isolated Compose project so it never touches a real deployment.
  Power-loss and interrupted-write tests landed
  ([rootguard#225](https://github.com/foxly-it/rootguard/issues/225)): fixed
  the one remaining non-atomic write (`writeBackupManifest`), then proved -
  via a real child-process SIGKILL mid-operation, restarted against the same
  on-disk state, since Go has no per-goroutine kill - that both
  `updater.Manager` (real Docker, killed between the pre-update backup and
  the candidate image swap, and again between the swap and verify/rollback)
  and `installer.Manager` (Docker-free, since `deploy()` hardcodes
  production container/network names with no isolated-project escape hatch
  the way `updater.Manager` now has) recover cleanly: an accurate
  interrupted-operation diagnostic, whatever was already captured before the
  kill stays intact, and a retried operation afterward completes
  successfully rather than leaving the appliance stuck.
  The disaster-recovery runbook landed
  ([rootguard#227](https://github.com/foxly-it/rootguard/issues/227)):
  `docs/disaster-recovery.md` covers total host loss, a failed update that
  didn't roll back cleanly, lost administrator credentials, a stuck
  deployment/update after a crash, and DNS-outage triage, each mapped to an
  existing mechanism rather than new capability. The total-host-loss
  scenario was drilled for real - an encrypted backup from a live
  installation restored on an entirely separate, freshly provisioned host,
  reaching `installed` through all seven restore steps, with the
  replacement host's own resolver independently verified (`ad`-flagged
  recursive resolution, `SERVFAIL` for a broken DNSSEC chain). The drill
  originally surfaced a real gap, since closed: the backup/restore feature
  landed on `main` after the published `v0.1.0-alpha.7` release but is
  included starting with the current `v0.1.0-beta.1` release.
  Logging/versioning/history, the automatic cleanup-safety work, and a
  network-addressing fix ([rootguard#182](https://github.com/foxly-it/rootguard/pull/182))
  have landed - AdGuard had no pinned address on the internal `rootguard-dns`
  network (an intentional choice at the time, see the 0.3 note above), which
  let it grab Unbound's reserved static address whenever that happened to be
  free; an Unbound image update could then fail with "Address already in
  use" *and* have its automatic rollback fail with the exact same error,
  leaving Unbound down until manually repaired. The controller's own
  `docker network connect` to that network had the identical gap. Both now
- **0.5 security/HTTPS/accessibility** - **complete**. Destructive-action
  rate limits/audit beyond the authentication surface
  ([rootguard#219](https://github.com/foxly-it/rootguard/issues/219)) and
  the formal keyboard/screen-reader re-verification plus WCAG labels/
  errors review
  ([rootguard#221](https://github.com/foxly-it/rootguard/issues/221)) both
  landed - see "Delivered and verified" above for both.
- **0.6 beta release engineering** - **complete**, all 10 items delivered
  and live-verified (see the 2026-08-14 update below for the final two).
  This was the actual gate to cut `0.1.0-beta.1` (continuing the existing
  `0.1.0-alpha.N` series - "0.6" is this roadmap's own milestone label, not
  the software's version number).
  SBOM/provenance for every image
  ([rootguard#229](https://github.com/foxly-it/rootguard/issues/229)) and
  release-image attestation verification extended from core/webapp to all 5
  components
  ([rootguard#230](https://github.com/foxly-it/rootguard/issues/230)) both
  landed and are now verified live: `v0.1.0-alpha.8`, a real tagged release
  cut specifically to prove this, published all 5 images with a fetchable
  SPDX SBOM and SLSA provenance predicate, and `cosign verify-attestation`
  - run from inside a live `rootguard-core` container using the exact
  binary and policy Core itself ships with - reported `verified` for all 5.
  Both flow from the same discovery: `release-alpha.yml` never set `sbom:`/
  `provenance: mode=max` on `docker/build-push-action` for any of its 5
  matrix components (only `ci-unbound.yml` did, for unbound specifically,
  outside the actual release path), and `attestation.go`'s policy map only
  ever covered core/webapp even though updater/unbound/blockpage are
  published by the identical signer.
  Automated semantic versioning
  ([rootguard#233](https://github.com/foxly-it/rootguard/issues/233)) and
  changelog generation
  ([rootguard#234](https://github.com/foxly-it/rootguard/issues/234)) are
  implemented together and now verified live, since both flow from the same
  mechanism: RootGuard follows Conventional Commits (documented in
  `CONTRIBUTING.md`), a manually-triggered `release-version-bump.yml`
  computes the next `v0.1.0-alpha.N` tag, refuses an empty release, and
  pushes it, and `cliff.toml` ([git-cliff](https://git-cliff.org/)) generates
  that release's `CHANGELOG.md` section plus a real GitHub Release from the
  commits since the last tag. `CHANGELOG.md` is seeded with the full history
  across all 8 pre-existing alpha releases, scoped to the real
  `v0.1.0-alpha.N` tag pattern rather than a bare `v*` glob - the repository
  also carries a few stray, non-release tags (`v1.0.0`, `v0.1.0`,
  `v0.2.0-service-discovery`) that would otherwise interleave into the
  changelog out of order. First live run (`v0.1.0-alpha.9`) got version
  computation, changelog generation, and the GitHub Release all correct, but
  surfaced a real bug: GitHub Actions never auto-triggers another workflow's
  `on: push: tags` from a push made with the built-in `GITHUB_TOKEN` (an
  anti-recursion safeguard undocumented enough to miss writing the original
  workflow), so the pushed tag alone never started `release-alpha.yml` - a
  "phantom" release existed (tag + GitHub Release, no images) until fixed to
  explicitly dispatch `release-alpha.yml` with the computed version, the
  same `workflow_dispatch` path already used by hand for every alpha release
  through `v0.1.0-alpha.8`. Confirmed working by manually running that exact
  dispatch command, which also retroactively completed the phantom
  `v0.1.0-alpha.9` release.
  Migration framework
  ([rootguard#237](https://github.com/foxly-it/rootguard/issues/237))
  landed, scoped deliberately to schema-version + fail-closed consistency
  rather than a full transform-function framework: every persisted JSON
  file across `rootguard-core`/`rootguard-updater` was audited, and only
  two genuine gaps were found (the rest already had adequate cheap
  validity checks). The updater's per-backup `manifest.json` now carries a
  `SchemaVersion`, matching its sibling `backupexport.Manifest`. Unbound's
  `settings.json` gets a `schema_version` envelope kept off the `Settings`
  type itself (also the guided-settings HTTP API response shape); `Load`
  refuses only a version *newer* than the build knows, leaving the
  existing additive-field migration (`jsonFieldExists`) untouched for an
  absent or older version.
  Website/Wiki CI check
  ([rootguard#240](https://github.com/foxly-it/rootguard/issues/240))
  landed, scoped to hard verifiable facts rather than content review:
  `scripts/check-site-facts.sh` (new `ci-site-facts.yml`, running on every
  push/PR to `main`, not path-filtered to `site/**`) compares every
  `0.1.0-alpha.N` mention in `site/*.html` against the latest real release
  tag and verifies every local link/asset reference resolves to a real
  file. Building it surfaced real drift, fixed in the same change:
  `site/*.html` still said `0.1.0-alpha.7` in the status badge, quick-start
  `curl` commands, pinned `.env` image digests, and version labels, even
  though `v0.1.0-alpha.8`/`v0.1.0-alpha.9` had already shipped.
  Upgrade tests
  ([rootguard#242](https://github.com/foxly-it/rootguard/issues/242)) and
  the compatibility matrix
  ([rootguard#243](https://github.com/foxly-it/rootguard/issues/243)) are
  now live-verified passing (`v0.1.0-alpha.16`) - a new `upgrade-test` job
  in `release-alpha.yml` deploys the previous published release exactly as
  it shipped, completes guided setup, verifies DNS, then upgrades
  Core/WebApp in place through the real control-plane updater (never a
  synthetic fixture) to the version being published, and verifies the
  running images and DNS resolution afterward.
  `docs/compatibility-matrix.md` consolidates this RootGuard-version axis
  with three that already had independent CI proof (Docker platform/engine,
  AdGuard channel, Unbound - RootGuard pins its own build, not an operator
  choice). Building the upgrade test surfaced two classes of real bug:
  - The release tag is created *before* the pin-update commit that follows
    it, so every existing tag pointed at a `compose.alpha.yaml` still
    pinned to the *previous* release's images - the documented quick start
    fetched the wrong images for every past release.
    `update-alpha-pins` now moves the tag to the pin-update commit it just
    created; `v0.1.0-alpha.7` through `.9` were retroactively force-moved to
    their correct commits (`.1`-`.6` predate the automation and have no
    correct target).
  - The job's first live runs failed six times in a row, each fix
    uncovering the next real problem: silently swallowed curl errors (twice
    - the second time from the fix's own `>/dev/null` redirect discarding
    its own error message), a check/install race, and a poll loop that
    didn't tolerate the WebApp being briefly unreachable while the
    control-plane updater swaps its container. The sixth and final fix
    surfaced a genuine, independent, previously-untested product bug: after
    a real successful control-plane update, `GET /api/control-plane-updates`
    reported success with no `history` entry at all -
    `rootguard-core/internal/controlplane/client.go`'s `Status` struct had
    no `History` field, so Core silently dropped it while proxying the
    updater's response. Fixed with a matching type and a new regression
    test (that package had zero test coverage before).

**Working order: top-to-bottom through the roadmap document**, per explicit
user direction (2026-08-09) - not "closest-to-done first" as this section
previously recommended. 0.1 and 0.3 are fully closed, so the order is:

1. **0.4 operations/backup/recovery** - **complete**, see above.
2. **0.5 security/HTTPS/accessibility** - complete, see above.
3. **0.6 release engineering** - **complete**, see above. This closes the
   gate to cut `0.1.0-beta.1`.

**2026-08-12 update:** 0.5 was picked up directly, ahead of 0.4 in the
working order above, per explicit user direction to resume specifically in
0.5 - both remaining items landed the same session: destructive-action
rate limits/audit
([rootguard#219](https://github.com/foxly-it/rootguard/issues/219)) and
the formal keyboard/screen-reader re-verification plus WCAG labels/errors
review ([rootguard#221](https://github.com/foxly-it/rootguard/issues/221)).
**0.5 is now fully complete.**

**2026-08-13 update:** 0.4's pre-update snapshot/restore verification
([rootguard#223](https://github.com/foxly-it/rootguard/issues/223)) and
power-loss/interrupted-write tests
([rootguard#225](https://github.com/foxly-it/rootguard/issues/225)) landed,
followed by the disaster-recovery runbook
([rootguard#227](https://github.com/foxly-it/rootguard/issues/227), see
above) - all worked through autonomously per explicit user direction to
complete 0.4. **0.4 is now fully complete.** 0.6 release engineering is next
in the recommended top-to-bottom order if not otherwise directed.

**2026-08-14 update:** 0.6's last two items, the compatibility matrix
([rootguard#243](https://github.com/foxly-it/rootguard/issues/243)) and
upgrade tests ([rootguard#242](https://github.com/foxly-it/rootguard/issues/242)),
are live-verified passing as of `v0.1.0-alpha.16` (see above for the six
rounds of `upgrade-test` fixes this took, including the real
`controlplane.Client` history-dropping bug the job surfaced). **0.6 is now
fully complete - the gate to cut `0.1.0-beta.1` is clear.** The public site
(`site/*.html`) also got a pass this session: version references brought
current, a compact single-dropdown mobile header replacing one that
silently failed to open on real iOS Safari (`.links`' `backdrop-filter`
turned it into a containing block for the dropdown's `position:fixed`,
which a `overflow-x:auto` fix then clipped), and the roadmap page's
"Release-Zug" status labels corrected to match this document (0.4/0.5 were
showing stale "in progress" copy despite being complete for a session).

**Superseded (2026-08-14):** milestones 0.1 through 0.6 are now ALL complete
and verified per `ROADMAP.md` (every checkbox in each of those sections is
`[x]`); `v0.1.0-beta.1` is the current public release. The "immediate next
item" pointer below, describing 0.2 as still open, is historical narrative
from when that was true - kept for context, not as current direction.
**0.9 status as of 2026-08-15**: six of eight checklist items are done, see
`ROADMAP.md`'s own 0.9 section for the full evidence citations per item
(PRs #274/#276/#277/#279/#280/#281). Summary:

- [x] Fresh install/upgrade/rollback/backup/restore matrix is green - the
  backup/restore CI gap (previously only mocked-Docker unit tests) is now
  closed with a real live export/teardown/restore/verify cycle
  (`scripts/verify-backup-restore.sh` + `backup-restore.yml`).
- [x] Performance/memory baseline (`docs/performance-baseline.md`).
- [x] Final accessibility/security review (`docs/
  accessibility-security-review.md`) - also where this cycle's auth
  hardening landed: rate limiting no longer trusts `X-Forwarded-For`,
  password recovery is now failure-safe, logout surfaces persistence
  errors, the static-file guard no longer misreads `[ ] * ?` as glob
  patterns.
- [x] Documentation walkthrough - caught a leftover "Alpha" mention the
  earlier grep-based sweep missed (inside a `<pre><code>` block, not a
  `data-de` string) plus a confusing unset-variable-looking placeholder.
- [x] Platform/support policy frozen (`docs/platform-support.md`).
- [ ] **In progress**: the 30-day endurance test itself
  (`scripts/soak/*.sh` + systemd timers on a dedicated host, started
  2026-08-14, 100% probe pass rate so far) - closes around 2026-09-13
  ([rootguard#271](https://github.com/foxly-it/rootguard/issues/271)).
- [ ] **Deferred on purpose**: release-blocking-defect review (do it once,
  right before cutting `1.0.0-rc.1`, not now) and the 1.0 migration
  guide (the mechanism is already built/tested/documented, but 1.0.0's
  own 10 checklist items aren't final yet - writing version-specific
  instructions now would risk describing a release that doesn't exist).

Also found and fixed along the way, not originally part of any checklist:
`.env.alpha.example` had been silently stale since `v0.1.0-alpha.5` (PR
#70) - `update-alpha-pins` only ever updated `compose.alpha.yaml`'s own
pins, so every release from alpha.6 through the current beta.1 shipped a
documented quick start that deployed alpha.5 images regardless of the
release name. Fixed going forward plus a retroactive correction and tag
move for the already-published `v0.1.0-beta.1` tag (PR #270). The LICENSE
file was also a hand-written paraphrase of AGPL-3.0 instead of the actual
license text, which is why GitHub's license detector showed "unknown" -
replaced with the full canonical text (PR #273).

**2026-08-15:** real end-user feedback (first-ever Docker Compose user)
surfaced two more issues, tracked and fixed together
([rootguard#286](https://github.com/foxly-it/rootguard/issues/286)). First,
a deploy failed with "address already in use" on port 53 even though the
`dns_port_available` preflight check re-runs server-side immediately before
`compose up -d` - a `docker ps`-based check can't see a just-stopped
container's port-53 socket (or a lingering `docker-proxy` process) that
hasn't fully released yet, a transient race no point-in-time check can rule
out. `deploy()`/`restoreDeploy()` now retry `compose up -d` up to 3 times
(2s apart) specifically for that error class instead of failing outright.
Second, the user read the Backups page's HTTP-transport warning as an
outright refusal to export ("das verweigert er mir, wegen fehlendem https
Protokoll") - there is no such block anywhere in the stack (checked
frontend button state, backend handlers, and the session cookie's `Secure`
flag), so the warning text was reworded to lead with "note, not a blocker."
Also fixed stale `site/docs.html` copy describing the occupied-port check
and collapsible technical-details UI as "next release" when both already
shipped in `v0.1.0-beta.1`.

**2026-08-16:** the `dns_port_available` preflight check itself had a
matching blind spot - it's `docker ps`-based, so it can't see a non-Docker
process bound to the requested port (most plausibly `systemd-resolved`'s
stub listener on Debian/Ubuntu hosts, a very common port-53 conflict for
exactly this project's target audience). Added `probeHostPortBusy`: when
the cheap `docker ps` scan finds nothing, RootGuard now also runs a
throwaway container that actually publishes the requested address/port -
the same mechanism `compose up` itself uses - reusing Core's own
already-pulled image so no extra pull is needed. Live-verified on the `.7`
test LXC against a real non-Docker occupant (`nc -l` on a test port
outside Docker): the probe correctly failed while occupied and passed once
freed, with the identical error text `deploy()`'s own retry logic already
recognizes ([rootguard#288](https://github.com/foxly-it/rootguard/issues/288)).
A second audit pass on that same change caught that its port-bind-conflict
matching (and `runComposeUp`'s) relied on `err.Error()` alone, which only
works because the production `CommandRunner` happens to fold
`CombinedOutput()` into the error text - not a guarantee the interface
itself makes. Both now match against output and `err.Error()` together.

Also that day: the Setup wizard's DNS bind address field defaulted to
`0.0.0.0` even though the browser reaches the page over the exact LAN IP
RootGuard's own docs already tell people to use
(`http://<host-LAN-IP>:8080`, never `localhost`). `Setup.tsx` now defaults
`dns_bind_address` to `window.location.hostname` when it's a usable IPv4
address (not loopback, not a hostname, not IPv6 - Core only accepts IPv4),
falling back to `0.0.0.0` otherwise. Pure client-side default, no backend
changes; the help text under the field now explains the pre-fill and that
`0.0.0.0` remains available to bind every host address instead. Suggested
directly by foxly-it
([rootguard#290](https://github.com/foxly-it/rootguard/issues/290)).
A follow-up audit pass caught the help text overclaiming - it read as if a
LAN address is always pre-filled, when a hostname/`localhost`/IPv6 access
still falls back to `0.0.0.0` - reworded to "if opened over an IPv4
address" in both languages. `detectDefaultBindAddress` also moved out of
`Setup.tsx` into `src/utils/network.ts` with its first real unit tests -
the frontend had no test runner at all until now; added a minimal Vitest
setup (`npm test`, wired into `ci-webapp.yml`) rather than growing a
second, uncoordinated test convention alongside the existing Playwright/
axe-core E2E suite.

**2026-08-16, later the same day:** publishing `v0.1.0-beta.2` and
redeploying it live on the `.7` test host (real end-user request: "bring
the LXC up to date") surfaced a genuine, previously-undetected
release-blocking defect. Deploying the DNS stack with the blockpage
enabled - the Setup wizard's own default - failed outright with `external
volume "rootguard-adguard-auth" not found`. Core's internally-rendered DNS
stack compose declares that volume `external: true`, expecting the outer
app-layer compose to have already created it - which the dev-only
`compose.yaml` does, but the *public* `compose.alpha.yaml` never did. Any
real first-time user following the actual public Quick Start with no
pre-existing Docker state would hit this on every attempt with the
blockpage left enabled. Root cause of the gap going undetected:
`scripts/verification-common.sh`'s `install_stack` config never set
`blockpage_enabled` at all, which Go unmarshals to `false` - so
`clean-install.yml`'s CI job has been silently testing the
blockpage-*disabled* path only, never the wizard's real default. Fixing
this properly took three commits, not one: declaring the volume in
`compose.alpha.yaml` alone wasn't enough - Compose only creates a
top-level named volume when some service in that same file actually
mounts it, so `core` needed the same `adguard-auth` mount and
`ROOTGUARD_BLOCKPAGE_AUTH_DIR` env var `compose.yaml` (dev) already had,
confirmed by a second live redeploy attempt failing identically. Fixing
`install_stack`'s shared config then exposed the exact same gap a third
time, in `verify-backup-restore.sh`'s own separate `deploy_config` for the
`/api/backups/restore` call - the primary instance now installs with the
blockpage on, so a restore config that still defaulted it to off
disagreed with what the exported archive actually contained. Fixing that
still left `Backup and restore` CI failing at the restore call itself with
a bare 409 and no visible reason - reproduced by hand on the `.7` host: a
leftover `rootguard-blockpage` container survived
`teardown_managed_resources` (its `managed_containers`/`managed_volumes`
lists predate blockpage ever being exercised by these scripts, so neither
it nor `rootguard-adguard-auth` were ever included), so the "fresh"
restore-target instance wasn't actually clean and `RestorePreflight`
correctly refused via `Restore()`'s hard `ErrNotClean` gate. Added both to
the managed-resource lists. Live-verified end to end on the `.7` host that
surfaced all of this, running the real `verify-backup-restore.sh`
unmodified: install with blockpage enabled -> export -> full teardown ->
fresh deploy -> restore -> DNS + DNSSEC verification, every step passing
([rootguard#294](https://github.com/foxly-it/rootguard/issues/294)).

The same redeploy also surfaced two independent findings: the Stack Center
showed `0.1.0-beta.2` as Unbound's version instead of the actual resolver
version (`1.25.2`) - `release-alpha.yml`'s shared image labels
unconditionally set `org.opencontainers.image.version` to the RootGuard
release tag for all five images, overwriting the one Dockerfile
(`rootguard-unbound`) that deliberately sets it from its own
`UNBOUND_VERSION` build-arg instead; core/webapp/updater/blockpage already
set the same label themselves from the passed `VERSION` build-arg, so the
shared override was pure redundancy for four images and silent data loss
for the fifth - removed. Separately, `scripts/check-site-facts.sh` (a
required CI check) had been failing unnoticed since the `v0.1.0-beta.2`
tag was cut: `site/index.html`, `site/docs.html`, and `site/wiki.html`
still named `0.1.0-beta.1` as current. Updated all three, including
`docs.html`'s update-target image digests; reworded two genuinely
historical `site/roadmap.html` claims ("superseded by 0.1.0-beta.1",
"completed with the release of 0.1.0-beta.1") to use the checker's own
recognized "starting with"/"ab " marker phrasing instead of just bumping
their version number, since those facts are correctly pinned to beta.1
regardless of which release is current.

All of #294's fixes landed in `v0.1.0-beta.3` (2026-08-16), cut the same
day: full release pipeline green (all five images, signed, `upgrade-test`
beta.2 -> beta.3, `smoke-test`), live-verified again end to end on the
`.7` host with the real images - install with blockpage enabled, DNS
resolution, DNSSEC rejection, and the Unbound version label now correctly
reading `1.25.2` instead of the release tag.

**2026-08-18:** a routine check ("why does RootGuard Unbound have errors
on GitHub, and the site looks stale again") found two more independent,
unrelated issues. First, the scheduled `Unbound CI` run started failing
with `E: Version '2.8.2-1~deb13u1' for 'libexpat1' was not found` -
`rootguard-unbound`'s Dockerfile pins exact Debian package versions for
reproducibility, and Debian's `libexpat1`/`libexpat1-dev` received a
security-patch bump to `2.8.3-1~deb13u1` that dropped the old version from
the mirror entirely (Debian repos only ever serve the latest point
release of a package). Checked every other pinned package in both build
stages against the real `debian:13-slim` base image on `.7` - only expat
had drifted; bumped both references, rebuilt the image on `.7` to confirm
(Unbound 1.25.2, `unbound-checkconf` clean). Second, `site/*.html` had
gone stale again the same way as before - the release pipeline updates
`compose.alpha.yaml`/`.env.alpha.example` automatically but never touches
the hardcoded site version strings, so cutting `v0.1.0-beta.3` without a
manual site pass left it one release behind within two days. Updated the
same three files (plus `roadmap.html`'s current-version badge) again.

Both of that day's findings recur by nature (a live upstream repository
drifting under a pin; a release shipping without a site pass), so instead
of just fixing them again, built prevention for each:

- `scripts/bump-site-versions.sh` mechanically applies exactly what
  `scripts/check-site-facts.sh` checks for - verified idempotent (a no-op
  against an already-current site) and, by reverting `site/*.html` to the
  known-stale beta.2 state and rerunning it, byte-identical to the manual
  beta.3 fix. Wired into `release-alpha.yml`'s `update-alpha-pins` job,
  right after the digest-pin update and committed in the same commit; a
  `check-site-facts.sh` run immediately after verifies it, since the pin
  commit itself carries `[skip ci]` and would otherwise never be checked.
- `scripts/check-debian-pins.sh` (with a `--fix` mode) extracts every
  `package=version` pin from `rootguard-unbound/Dockerfile`, checks each
  against the pinned base image's real `apt-cache policy` output, and
  reports or rewrites whatever's drifted - verified against both states on
  the `.7` host: reports nothing on the current (fixed) Dockerfile, and
  against the known-drifted pre-fix one, detects and `--fix`es both
  `libexpat1`/`libexpat1-dev` occurrences to the exact version the manual
  fix used. New scheduled workflow
  (`.github/workflows/debian-pin-freshness.yml`, daily) runs it and, on
  drift, force-pushes a fix to one reused branch and opens or updates a PR
  - `ci-unbound.yml`'s own `pull_request` path filter already covers
  `rootguard-unbound/**`, so the real build tests the fix before anyone
  merges it, same as any other PR.

**2026-08-18, same day:** `site/docs.html`'s preflight step and "Port 53
is already in use" FAQ entry only described the old `docker ps`-based
port check, predating the two-stage real-bind-test `probeHostPortBusy`
added for [rootguard#288](https://github.com/foxly-it/rootguard/issues/288) -
reworded both to explain the actual current
mechanism (published-Docker-ports check first, then a real host-level
bind attempt that also catches non-Docker occupants like
`systemd-resolved`/`dnsmasq`), and that the FAQ case is now detected
automatically rather than something the user has to go check by hand.

**2026-08-18, later the same day:** renamed "Control Plane" to "Control
Panel" everywhere it names the WebGUI Dashboard eyebrow and Stack page
section (`Overview.tsx`, `Stack.tsx`'s history-row `scope`, the
`stack.controlPlane*` i18n strings, and the matching public-site Dashboard
mockup on `index.html`). Deliberately scoped to WebGUI-visible text only,
confirmed with foxly-it first: `architecture.md`/`threat-model.md`/
`README.md`/`ROADMAP.md`/`docs.html`'s quickstart and manual/`wiki.html`
all keep "control plane" as the precise architectural term (the
privileged Core/WebApp/Updater layer, contrasted with the data-plane DNS
services) - renaming those would make that established distinction less
accurate, not more. Also untouched: env var names
(`ROOTGUARD_CONTROL_PLANE_UPDATER_*`), the `/api/control-plane-updates*`
routes, and internal Go/TS identifiers (the `controlplane` package,
`ControlPlaneUpdateStatus` type) - all outside the public UI surface and
would otherwise break already-running installations without a migration
path.

---

0.2's conflict-detection checkbox
stays unchecked at the time this paragraph was written, but the only concrete gap left under it is a guided surface
for access rules (currently expert-only) - a separate, larger feature (see
"Tracked editor follow-ups" below), not a conflict-detection bug. Cross-zone
hostname uniqueness is now enforced (mirrors the existing PTR-address
check). Resolver config import/export is done
(`UnboundConfigTransfer.tsx`). Hand-written `unbound.conf` import is now
also done at full roadmap scope (`UnboundConfImport.tsx`, `import.go`'s
classifier), landed across PRs #171/#173/#174/#175 per explicit user
direction (2026-08-10) to build it completely rather than ship a leaner
classify-only version: fixed-base filtering/conflict detection, all scalar
guided settings, `forward-zone` (including the zone-scoped
`domain-insecure`/`private-domain` opt-ins), `local-zone "static"` host
inventory (CNAME/mismatched-PTR/non-static/RFC1918-name all correctly fall
back to expert adoption instead of guessing), RFC1918 reverse-zone policy,
network mode, and resource-profile reverse-mapping, plus expert-config
adoption for everything else, all work end to end. Hardened in PR #176/#177
against a real hand-written config reported live: a PTR-target
trailing-dot mismatch that silently misclassified a clean zone as expert,
a custom-config accumulation bug causing a genuine `unbound-checkconf`
syntax error on re-classification after activation, and a follow-on
duplicate-zone error on re-import (zones now merge by name, so re-import
is idempotent). PR #178 also moved the guided local-zone host list from an
inline chip row to a proper table, since an 11-host real-world zone made
the chip row hard to scan.

Scenario tests (home network, VLANs, split DNS, IPv6-only local records,
broken upstreams, DNSSEC failures) landed in PR #183: a build-tag-gated Go
integration suite (`rootguard-core/internal/unbound/
scenario_integration_test.go`, `//go:build integration`, so `go test ./...`
stays Docker-free) that constructs real `Settings` values the way a user's
guided WebGUI configuration would, renders them through production
`Settings.Render()`, applies them to a live `rootguard-unbound` container,
and verifies with real `dig` queries against a running resolver - not
string-matching rendered config. Wired into `ci-unbound.yml` as a new
`scenario-tests` job gating image publishing. Two design corrections only
surfaced by testing against the real resolver: the split-DNS/`AllowUnsigned`
check initially relied on a real third-party domain's DNSSEC status
(`dnssec-failed.org`), which turned out to test the wrong thing entirely -
that domain is deliberately signed-with-broken-signatures, not unsigned, so
`domain-insecure` never applied to it regardless of the setting - redesigned
around forwarding behavior instead; and the broken-upstream scenario
originally expected a fast SERVFAIL, but a forward target that's silently
dropped rather than actively refused (true of TEST-NET-1 in this
environment) has no effective ceiling in Unbound's default retry behavior
at all - still pending after a full 5 minutes in manual testing - so the
test now verifies what actually matters operationally instead: one stuck
forward zone must not block unrelated queries. Move on to
**host-discovery beyond the FRITZ!Box adapter**, the next unchecked 0.2
item in document order (conflict detection stays deliberately open, blocked
on the untracked guided access-rules surface).

## Tracked editor follow-ups

- Extend the guided-assistant pattern to tightly scoped access-control rules.
- Generate and version the completion/documentation catalog for every directive
  supported by the installed Unbound release; the current catalog covers the
  common, safe RootGuard use cases.
- Expand semantic Advisor rules beyond the current security-, forwarding-,
  access-control-, and local-zone checks to cover more directive combinations.

## Release status

`v0.1.0-beta.7` is the current public release, published with digest-pinned
`amd64`/`arm64` images for all five RootGuard components and a live-verified
`upgrade-test` job in the release pipeline. Milestones 0.1 through 0.6 are
complete and verified; the remaining gates before 1.0 are 0.9 (release
candidate) and the 1.0.0 stable-appliance checklist itself (see
`ROADMAP.md`).

Cutting beta.4 (2026-08-22) surfaced three real release-pipeline bugs, none
caught before because this was the first release since #298 added the
site-refresh/pin-consistency automation and since the alpha->beta series
transition: `update-alpha-pins`'s checkout lacked tag history (`fetch-depth`
default of 1), so `bump-site-versions.sh` died silently under
`set -e`+`pipefail` before its own error message could print
([rootguard#311](https://github.com/foxly-it/rootguard/issues/311));
`upgrade-test`'s "previous release" detection only ever listed
`v0.1.0-alpha.*` tags, silently testing an upgrade from `alpha.16` instead of
the real N-1 release for every beta release so far, until the
`compose.alpha.yaml`->`compose.release.yaml` rename turned the
already-wrong-scope lookup into a hard crash (same issue, fixed by re-deriving
via `git for-each-ref --sort=-creatordate` across both series); and a push
made with the workflow's own `GITHUB_TOKEN` never auto-triggers `pages.yml`
(the same GitHub anti-recursion rule `release-version-bump.yml` already
works around for tag pushes), so the pin-refresh commit's site changes
never deployed on their own
([rootguard#314](https://github.com/foxly-it/rootguard/issues/314)) - now an
explicit `gh workflow run pages.yml` dispatch, mirroring that same existing
pattern. A fourth, narrower bug: the rename intentionally left the public
quick start's `curl` commands pointing at the old filename under the
still-current `v0.1.0-beta.3` tag so they wouldn't 404 immediately, but
`bump-site-versions.sh` only ever substitutes the version-number substring,
not filenames - so the very next release advanced the version in that same
text without the filename, producing "new version, old filename," a
combination that never existed and did 404 live on the public site for a
few minutes before being caught and fixed.

`v0.1.0-beta.5` shipped the same day, adding live GitHub-Releases update
discovery ([rootguard#320](https://github.com/foxly-it/rootguard/pull/320))
and updater self-update
([rootguard#322](https://github.com/foxly-it/rootguard/pull/322)). Live
verification of the latter immediately surfaced three more real bugs -
a self-referential compose-mount resolution failure, a digest-less
attestation gap, and genuinely broken YAML in beta.5's own committed
pin-refresh commit - all fixed same-day
([rootguard#324](https://github.com/foxly-it/rootguard/pull/324)). That
fix's own follow-up correction for the already-published beta.5 pin
commit then hit two release-pipeline mechanics for the first time: a
squash-merged two-commit historical correction collapses into a
net-zero diff and silently undoes itself (fixed by re-merging with a
real merge commit,
[rootguard#326](https://github.com/foxly-it/rootguard/pull/326)), and a
tag-triggered workflow run stays pinned to its original checkout even
when rerun (worked around with a fresh `workflow_dispatch` against
`main` for the same version). `v0.1.0-beta.6`, cut immediately after
with all of the above already on `main`, passed its full pipeline
including `upgrade-test` cleanly on the first real (non-rerun) attempt.

**2026-08-24:** dashboard/AdGuard polish and bugfix round, released as
`v0.1.0-beta.7` ([rootguard#331](https://github.com/foxly-it/rootguard/pull/331)).
Code review of the metrics-caching work from the previous session found
two remaining perf/concurrency gaps: `dashboardHandler` still ran `docker
inspect` five times per request (only `docker stats` had been cached),
and concurrent stale-cache callers (the background ticker plus several
requests/tabs at once) could each start their own redundant refresh.
Fixed by mirroring the existing metrics cache for container status
(`CollectStatus` in `stack/status.go`) and adding a small singleflight
(`stack/refresh.go`) shared by both caches; `fetchAdGuardStatus()` (up to
four real AdGuard API calls) also moved off the 500ms poll onto its own
5s interval, since it was still hitting AdGuard directly on every tick.
Metrics now carry their real `collected_at` so the frontend can skip
re-recording an unchanged cached sample under a fresher-looking age. Live
verification on `.7` caught a real bug in this batch before it shipped:
the webapp backend round-trips Core's `/api/dashboard` response through
its own mirrored Go struct rather than forwarding raw bytes, and that
struct hadn't been given the new field - `collected_at` was silently
dropped even though Core sent it, until a direct `curl` comparison
surfaced the gap.

Also this session: removed the dashboard's "DATENFLUSS" card (both the
live app and the marketing site's hand-built mockup, which had gone
stale the moment the real card was removed and was still showing a
fourth-of-five runtime list to boot); and added an AdGuard
protection-pause toggle (Off/10 minutes/1 hour) mirroring AdGuard Home's
own native "disable protection" control via its real `/control/protection`
endpoint - AdGuard's own server-side timer re-enables it, no RootGuard
scheduling needed. A second code-review pass on that toggle found three
more real bugs before it shipped: `protectedState` (and AdGuard.tsx's own
`ready`) never actually factored in `protection_enabled`/`filtering_enabled`,
so pausing protection still showed a green "PROTECTED" dashboard; the
select derived its displayed value straight from `protection_enabled`, so
a 10-minute pause displayed identically to "off indefinitely" the instant
it was chosen, and the AdGuard page never polled again afterward, so it
stayed on "paused" after AdGuard's own timer re-enabled protection until
a manual reload; and the protection handler took a plain `bool` for
`enabled`, so an empty `{}` request body silently decoded to
`{"enabled":false,"duration_seconds":0}` and disabled protection
indefinitely with no expressed intent. Fixed: `reachable` (configured/
healthy/upstream-ready) now gates the pause control itself so pausing
protection can never hide the control needed to un-pause it, while
`ready`/`protectedState` additionally require protection and filtering
both enabled; the select is now a pure action trigger with the real
state plus a live countdown (sourced from AdGuard's own
`protection_disabled_duration` via `/control/status`, previously only
`version` was read from that response) rendered separately, and the page
polls every 5s while paused; `enabled` is now `*bool` (a missing field is
a 400, not a silent false), and `duration_seconds` is restricted to the
three values the UI actually offers - enforced identically in Core and
the webapp proxy.

`v0.1.0-beta.7`'s full release-pipeline run (test/publish/pin-refresh/
smoke-test/upgrade-test) passed cleanly on the first attempt. Used it to
verify RootGuard's own update mechanism end-to-end rather than just
trusting the pipeline: reset the `.7` test LXC to a genuine, freshly
pulled `v0.1.0-beta.6` (core/webapp/updater) plus its existing
`v0.1.0-beta.5` Unbound - i.e. what a real user still on the previous
release actually has - then drove the real in-app flows to bring every
component to beta.7: `/api/control-plane-updates/check` +
`/install` for the atomic Core+WebApp update (WebApp restarting itself
mid-update and reconnecting cleanly afterward), `/api/updates/check` +
`/api/updates/unbound` for Unbound, and `/api/updater-updates/check` +
`/install` for the Updater's own self-update. All four succeeded; the
post-update dashboard's `collected_at` field (absent on the pre-update
beta.6 response, since that fix hadn't shipped yet) confirmed the live
containers were genuinely running the new code, not just freshly
recreated with the same tag. Also re-verified the protection-pause
round trip against the real published beta.7 build (not just the local
`:dev` build used during development): paused for 600s, confirmed
`protection_disabled_duration_ms` counting down live, then restored.

**2026-08-24, same day:** added a one-command installer,
`install.sh` at the repo root, for non-technical ("DAU") users - user ask
was a one-liner alternative to the manual quick start that detects/
installs Docker, downloads the release, generates the secret tokens, and
only asks for a WebGUI username/password, without taking anything away
from the existing manual path for people who want to see every step.
Detects Docker and, if missing, installs it via the separate
[foxly-it/dockerinstall](https://github.com/foxly-it/dockerinstall)
project (Debian/Ubuntu; that script already rejects unsupported OSes
with a clear message on its own, so `install.sh` doesn't duplicate that
logic). Resolves the current release the same way `rootguard-core`'s own
updater does - GitHub Releases API, newest tag matching the release
pattern, since `/releases/latest` excludes prereleases and every
RootGuard release is one - so this script carries no version string of
its own to remember to bump at release time, unlike `site/*.html`.
Generates `ROOTGUARD_API_TOKEN`/`ROOTGUARD_RECOVERY_TOKEN`, prompts for a
username/password (or auto-generates and prints one for
`--non-interactive` use) explicitly via `/dev/tty` - a plain `read` in a
`curl | bash` script reads from the download pipe instead of the
terminal - and substitutes secrets into `.env` through `awk`+`ENVIRON`
rather than sed/awk program text, so a password containing `&`, `#`, or
`\` can't corrupt the substitution (verified live with exactly such a
password). `site/index.html`'s quick start now leads with the one-liner;
`site/docs.html`'s full manual walkthrough is untouched, with a callout
linking to the new fast path instead. Verified live end-to-end twice, not
just via shellcheck: once with a typed password, once via
`--non-interactive`, both pulling the real published beta.7 images via
the real GitHub API and ending in a successful login with the credentials
the script set.

**2026-08-24, later the same day:** four follow-ups found by the user
actually using the shipped beta.7.

- **Authenticated account settings had never been merged.** User: "Und in
  Beta 7 hast du anscheinend die Benutzerverwaltung gelöscht" - it hadn't
  been deleted, PR #330 (`feat/account-settings`, the change username/
  password work from the prior session) had simply sat open, unmerged,
  the whole time beta.7 was cut and this session's other work landed.
  Rebased it onto current main (clean merge, no conflicts across 30+
  commits of divergence), re-verified the full test/build/lint suite,
  and merged it. Live-verified the actual endpoint on `.7` afterward
  (not just CI): renamed the logged-in session's own username twice in a
  row (round-trip back to the original) with the session staying alive
  throughout, exactly as designed. Prompted by this, swept every other
  branch in the repo for the same "PR open but never merged" gap
  (checked all ~40 branches sitting ahead of `main` against the GitHub
  API's own merge state, not just git ancestry, since a squash-merged
  PR's source branch is *expected* to look "ahead" by one commit)  -
  `feat/account-settings` was the only real gap found.
- **The new AdGuard protection select didn't match the app's other
  dropdowns.** Was using ad-hoc padding/radius/background instead of the
  `--field-bg`/7px-radius/34px-padding tokens every other themed
  `<select>` in the app already uses (`.resource-profile-field` in
  unbound-polish.css, `.logs-toolbar select`). Fixed to match.
- **The marketing site's hero mockup didn't structurally match the real
  dashboard.** User: fake metrics are fine, but it should be 1:1. The
  mockup's 5 metric cards were missing the sparkline chart + Min/Max
  caption every real `SparkMetric` card has had since the metrics-history
  work, and the runtime list was a plain bullet list instead of the real
  `.service-card` treatment (icon box, restart-button corner, status
  text). Rebuilt both to match, live-verified via headless Chromium.
- **`install.sh` now checks for busy ports before installing** (user
  ask, tying back to Setup's own existing two-stage port-53 preflight
  described in docs.html). Port 8080 - what the script's own `docker
  compose up` actually binds - is a hard check with an interactive
  alternate-port prompt; 53/80 are informational only, since install.sh
  doesn't bind those itself (Core creates the DNS/blockpage containers
  later, during guided Setup). **Real incident while testing this
  live on `.7`**: `compose.release.yaml`'s containers use fixed
  `container_name` values, so running the script's actual `docker
  compose up` step against `.7` recreated the *real* deployment's
  containers regardless of `--dir` (isolating the download directory
  does not isolate the Docker container namespace on the same host) -
  and a `ROOTGUARD_WEB_PORT` override exported for that test leaked into
  the recreated real webapp's own port binding via environment
  inheritance, moving it off 8080. Both caught and fixed within the same
  session (recreated from the real `/root/rootguard-test` checkout with
  its correct `.env`, confirmed login and `collected_at` afterward) -
  no data loss (named volumes are independent of container identity),
  but a concrete reminder that `.7`/`.61` are shared persistent
  environments, not disposable ones: any `install.sh`-style test that
  actually runs `docker compose up` needs an isolated host (verified the
  rest of this feature locally on macOS Docker Desktop instead, once
  this was understood).

**2026-08-25:** a formal code-review pass on the accumulated work above
found four concrete issues, one security-critical, worked through in
priority order.

- **Login/recovery/account rate limiting had a TOCTOU race** (PR #342).
  `blocked()` (check) and `recordFailure()` (record) were two separate,
  non-atomic operations around an expensive PBKDF2 verification -
  concurrent requests could all pass the check before any of them got
  counted, so the limiter only ever bounded *sequential* guessing, not
  concurrent. Fixed with an atomic reservation pattern: `beginAttempt`
  combines the check with reserving an in-flight slot (counted as if it
  were already a failure for admission purposes, without permanently
  recording it), `endAttempt` releases that slot and optionally converts
  it into a real, window-tracked failure. Verified the fix is load-
  bearing, not just plausible: added a test that fires 30 truly
  simultaneous login attempts via a `sync.WaitGroup` release gate,
  confirmed it fails against the pre-fix code (temporarily reverted via
  `sed`, saw a 30/0 pass/block split instead of the correct 5/25) before
  restoring the fix and re-verifying green.
- **AdGuard filtering endpoint still had the `{}`-disables-filtering
  bug** the protection endpoint was already fixed for earlier (PR #343).
  Same root cause, same fix shape: `Enabled bool` → `*bool` with an
  explicit nil check, plus a `decoder.More()` check on both the
  filtering and protection handlers (Core and WebApp) to reject trailing
  JSON after the first decoded value - a bare `Decode()` call silently
  ignores anything after the first object. Live-verified on `.7`: both
  `{}` and trailing-JSON bodies now return 400, a real on/off filtering
  toggle still works.
- **`install.sh` ignored `ROOTGUARD_ADMIN_USER`.** Unlike the password
  handling, the username prompt was called unconditionally instead of
  checking the environment variable first - fixed to match the password
  pattern (check env, only prompt if empty).
- **`install.sh` accepted any string as `ROOTGUARD_WEB_PORT`**, both the
  env var and the interactive alternate-port prompt, only failing later
  and less clearly inside Docker Compose. Added a `valid_port()` check
  (numeric, 1-65535) at both points.
- **`install.sh` left an empty target directory behind on a failed
  download**, which then tripped the "already exists" abort on retry.
  Downloads now land in a `mktemp -d` scratch directory first and are
  only moved into `$TARGET_DIR` once both succeed.

Two follow-up review passes on this same work (PR #345, #346) closed the
remaining low-priority items:

- **AdGuard page polish** (screenshot feedback): the "DNS filtering"
  checkbox is now a green/red on-off switch; the "AdGuard protection"
  select was too short and its color (`--field-bg`, a near-black
  recessed-field tone designed for softer backgrounds) looked mismatched
  directly on `.adguard-panel`'s solid `--surface` - restyled to
  `--surface-soft` + a full-strength border at the app's primary field
  size. Live-verified in both themes on `.7` via Playwright screenshots,
  including toggling filtering off/on to confirm the underlying request
  still works.
- **Shared `decodeStrictJSON` helper**: login/recovery/account in
  `auth.go` each ran a bare `Decode()` with no trailing-data check,
  unlike the already-fixed AdGuard handlers. Extracted one helper
  (decode + `decoder.More()`) and switched all three over.
- **Recovery rate-limit slot released too early**: `handleRecovery`
  freed its limiter slot right after the token check, before the
  expensive PBKDF2 derivation and session-wipe/credential persistence
  that follow a valid token - anyone holding the (already
  high-privilege) recovery token could fire unbounded concurrent resets.
  Now holds the slot through the whole handler, mirroring
  `handleAccount`'s existing pattern. Regression-tested the same way as
  the earlier login race fix: reverted, confirmed the test fails (18
  accepted instead of 5), restored.
- **`install.sh`'s target-directory check ran too late** - after
  `check_ports` and a potential real Docker install. Moved to the very
  first thing `main()` does.
- **First React component tests in this repo**: added
  `@testing-library/react` + `jsdom` (this project had none before -
  only plain-function utility tests) and covered the new switch/select
  - click/keyboard toggling, busy/disabled state, `filtering_enabled`
  rendering, the select's reset-after-action behavior.

A third, full-repository review pass (PR #347, #348, #349, #350) closed
one more security-relevant finding, two low-priority bugs, unified strict
JSON decoding across every remaining handler, and shipped one user-driven
website feature:

- **Weak startup secrets accepted** (medium priority, security). WebApp,
  Core, and the Updater only checked that
  `ROOTGUARD_API_TOKEN`/`ROOTGUARD_ADMIN_PASSWORD`/`ROOTGUARD_RECOVERY_TOKEN`/`ROOTGUARD_UPDATER_TOKEN`
  were non-empty, accepting e.g. `ROOTGUARD_ADMIN_PASSWORD=a` -
  inconsistent with the 12-character minimum a password *change* already
  enforces. Added a `requireSecretStrength` check to each binary's
  `main()` (12 chars for the admin password, 32 for tokens) that also
  rejects an unedited `.env.release.example` placeholder outright, since
  those placeholder strings happen to be long enough to pass a length
  check alone. `install.sh`'s own manual/env-supplied password path got
  the matching check, moved to before any directory/download side
  effects. **Caught live**: the first CI run against this fix failed -
  two `scripts/verify-*.sh` fixtures and one hardcoded literal in
  `ci.yml`'s own recovery-flow test still used short placeholder tokens
  that the grep sweep for the env var *name* had missed (they set the
  var via `export VAR="literal"`, not a workflow `env:` block); fixed in
  a follow-up commit once CI surfaced it.
- **FritzBox IPv6 URLs malformed.** `FritzBoxClient` built its base URL
  with a bare `Sprintf`, producing an invalid `http://fd00::1:49000` for
  an IPv6 router address instead of `http://[fd00::1]:49000`. Switched to
  `net.JoinHostPort`.
- **Backup restore off-by-one.** The entry-count guard used
  `count > MaxFiles` instead of `>=`, accepting exactly one more archive
  entry than the documented limit.
- **JSON decode unification finished**: PR #346 only covered
  login/recovery/account in the WebApp's `httpapi` package. Core's
  `routes.go` still had 13 near-identical decoder blocks (none with a
  trailing-data check), and the WebApp's own proxy handler package had 12
  more. Added one generic `decodeJSON[T any]` helper per module and
  converted every call site - no behavior change for endpoints that
  already worked, every endpoint that previously accepted trailing JSON
  data now rejects it with 400.
- **Duplicate frontend API client removed.** `services/api.ts` existed
  solely for `GET /api/version`, with its own raw `fetch()` that never
  checked for a 401 - unlike the central client's `request<T>` helper,
  it never fired the `rootguard:unauthorized` event on an expired
  session. Moved into `api/client.ts`, deleted the duplicate.
- **Website: copy-to-clipboard button + animated install demo**
  (user idea, refined in conversation). The quick-start command block
  was missing a copy button entirely. Added one, plus a new "Live
  preview" section showing a faithful example `install.sh` session
  (real `log()`/`prompt()` message wording) with lines staggering into
  view via `IntersectionObserver`, respecting `prefers-reduced-motion`.
  The version line stays live via the existing `project-data.json`
  fetch; its static fallback is kept in sync for free by the existing
  `bump-site-versions.sh` release-pin step.

Two remaining code-compression suggestions from the same review (splitting
`routes.go`/`auth.go`/other large files by responsibility, and unifying
each module's own temp-file-then-rename atomic-write pattern into a
shared helper) were explicitly deferred - the user picked only the API-
client removal above; the review found no bugs in either, just organizational
improvement.

**Real bug found while cutting the resulting beta (0.1.0-beta.10), not by
review**: the release pipeline's `upgrade-test` job failed twice in a row -
"Core und WebApp verwenden bereits die aktuellen Images", skipping the
upgrade entirely, even though beta.10's images are genuinely different
from beta.9's (independently verified against the registry). Root cause
in `rootguard-updater`'s `digestQualify`: it resolved a freshly-pulled
tag's digest via `docker image inspect --format {{.RepoDigests}}`,
returning the *first* repo-prefixed match - `RepoDigests` belongs to the
local image object as a whole and can list more than one digest for a
repository whose tag recently moved between releases, so the loop had no
way to distinguish "the digest just pulled" from "some older digest this
local image happens to also carry," and reproducibly returned the stale
one on the GitHub Actions runner's Docker setup (did not reproduce on a
different host with a containerd-backed image store). **This is a real
correctness bug in the live `/api/control-plane-updates/check` path**, not
just a CI artifact - any real installation could plausibly hit the same
false "already up to date" under the right local Docker state, on any
release before this fix. Fixed by reading the digest `docker pull` itself
reports in its own output instead of round-tripping through the ambiguous
local `RepoDigests` lookup (kept as a fallback for an already-qualified
static pin or unexpected output). `beta.10`'s images themselves are fine
(fresh install verified via its own passing smoke-test) - only the
*upgrade path* was affected; `beta.11` carries the fix and re-verifies the
upgrade-test passes for real.

**That fix wasn't the whole story - two more real bugs surfaced chasing
this, across `beta.11` through `beta.13`:**

- **Core has its own separate copy of the exact `digestQualify` bug**
  (`rootguard-core/internal/updater/manager.go`, used for AdGuard/Unbound's
  own self-update checking) - a different Go module than
  `rootguard-updater`, so PR #352/#353's fix there didn't cover it. Same
  `digestFromPullOutput` fix applied here too (PR #355).
- **The GitHub Releases API doesn't reliably return releases newest-first**,
  despite `pickLatestReleaseImage`'s own doc comment promising it. Directly
  querying the live API showed `v0.1.0-beta.9` listed *ahead of*
  `v0.1.0-beta.12`, on a repository that had several releases cut in quick
  succession. Every live self-update resolution (core, webapp, updater,
  unbound) silently trusted that ordering. Fixed by ranking every matching
  release locally by `(series, build number)` parsed from the tag itself,
  ignoring API response order entirely (PR #355).

**Why the upgrade-test kept failing even after both of those landed** (root
cause found by the user, not guessed): the upgrade-test's `check`/`install`
calls run against the *previous* release's own Core - `beta.12`'s Core for
the `beta.13` test - which still has the API-ordering bug, since that fix
only ships *in* `beta.13`. Core resolves "latest core/webapp release"
itself and sends that as a `target_images` override, which *wins* over the
updater's own correctly-set `ROOTGUARD_*_UPDATE_IMAGE` env vars
(`targetImageFor` in `rootguard-updater/main.go` prefers the override).
`beta.12`'s Core kept resolving to whatever the live API listed first
(`v0.1.0-beta.9`) and pushing that as the override - so the "upgrade"
silently installed `beta.9` instead of `beta.13` every single time,
explaining why the exact same wrong digest recurred across every version
pair tested (`beta.10→11`, `11→12`, `12→13`). A fix only helps upgrades
*after* the release that first ships it; it can't fix the one jump onto it,
since the code doing the resolving at that point is still the old, buggy
version. Worked around for exactly that one CI transition (PR #356):
`release-alpha.yml` special-cases `previous == 0.1.0-beta.12` and calls the
updater's own `/api/control-plane/{check,update}` directly with an explicit
override via `docker exec rootguard-core wget ...`, bypassing only the old
Core's resolution - still exercises the real
updater/pull/digest/compose/verify/rollback path either way. Reverts to the
normal path automatically once `beta.13` (which carries the fix) becomes
"previous". **Confirmed working**: `beta.13`'s full release pipeline,
including `upgrade-test`, passed end to end.

**Real-installation impact, deliberately not mitigated further**: any
`beta.12` installation clicking "check for updates" in the WebGUI could hit
this same bug and silently resolve to an older release than `beta.13`.
Given `beta.12` was published for roughly an hour before `beta.13` and this
is pre-1.0 software with no known external adopters caught in that exact
window, building dedicated tooling or public documentation for it was
judged disproportionate. If anyone ever does report being stuck on
`beta.12`, the same manual `docker exec rootguard-core wget ... http://
updater:8082/api/control-plane/update` bootstrap documented above (with
`target_images` pointed at the desired version) is the escape hatch.

**Durable safety net added on top, for the future** (PR #357, user asked
"and fixed going forward?" after the postmortem above): the fixes so far
close the three *specific* bugs found, but nothing stopped `update()` from
applying a *future* bad resolution the same way, whatever causes it -
"different image ID" was treated as "newer," full stop. `update()` now
compares both images' `org.opencontainers.image.version` label (every
Dockerfile already sets it from its own build) using the same
`(series, build number)` ranking `pickLatestReleaseImage` uses, and refuses
with a clear error if the candidate is genuinely older - silently skipping
the check for anything that doesn't parse as a RootGuard release version
(a local `:dev` build) rather than blocking it. This can't rescue a release
that shipped before it exists (the code doing the resolving during an
upgrade is whichever version is *currently running*), but it protects
every upgrade *from `beta.14` onward* against this entire bug class.
Confirmed live: `beta.14`'s `upgrade-test` passed via the normal
WebApp-proxied path (the `beta.12`-specific bypass in `release-alpha.yml`
correctly stayed dormant, since `beta.13` - carrying the ordering fix - is
now "previous").

**SemVer/tagging generalization, part 1** (PR #359, branch
`feat/general-semver-release-tagging`): the Go-side update mechanism
(`isOlderReleaseVersion`, `pickLatestReleaseImage`) was already moved onto
`golang.org/x/mod/semver` for full generality (see above). The user's own
review of the branch before merge found three real bugs in the
*surrounding shell scripts*, none caught by CI because none of the
existing checks exercise a stable (non-prerelease) or malformed version:

- `release-alpha.yml`'s version-gate used a shell glob
  (`[0-9]*.[0-9]*.[0-9]*`) that isn't a SemVer check at all - accepted
  `1foo.2bar.3baz`, a leading-zero core like `01.2.3`, and build metadata
  (`1.2.3+build.4`, which isn't even a legal Docker tag). Replaced with a
  full SemVer 2.0 grammar via bash `[[ =~ ]]` (build metadata excluded on
  purpose, same reason it's invalid in a tag).
- `install.sh`'s `resolve_latest_tag` used `sort -V | tail -1`, which
  ranks a prerelease *above* its own stable release (`1.0.0-rc.1` above
  `1.0.0`) - verified empirically, and previously documented as an
  "accepted gap" that turned out to be a real, fixable bug. Fixed by
  rewriting each tag's version-core/prerelease separator from `-` to `~`
  before sorting (GNU `sort -V` treats `~` as sorting before everything,
  same as dpkg) and back after - correct precedence, still no jq/semver
  dependency.
- `check-site-facts.sh`/`bump-site-versions.sh` required a letter-leading
  prerelease suffix on every version match, so a bare stable tag
  (`v1.0.0`) was invisible to `latest_tag` resolution entirely - the
  scripts would keep tracking an old `rc.N` as "current" forever. Split
  into two patterns: a wider, suffix-optional `tag_version_pattern` for
  resolving the latest real git tag (safe - no prose to false-positive
  against), and the original letter-leading `version_pattern` kept
  unchanged for prose scanning, since widening *that* one was verified
  live to reintroduce the exact SVG-path-coordinate false positives the
  letter requirement was added to fix in the first place (`site/*.html`
  is full of SVG icon path data shaped like fake dotted-number
  "versions"). Originally left as a documented accepted gap (a correct
  bare mention would be invisible to the checker); superseded by a real
  fix in the same PR's second review round - see below.

All three verified directly against the user's own reported reproduction
cases (glob-fooling strings, `v1.0.0`/`v1.0.0-rc.1` ordering, tag
resolution finding a bare stable tag), plus a live run of
`check-site-facts.sh`/`bump-site-versions.sh` against the real repo
(idempotent, no site content changed) and `shellcheck` (clean).

Also restructured `ROADMAP.md`'s 30-day soak test
([rootguard#271](https://github.com/foxly-it/rootguard/issues/271)) from
a 0.9 (pre-`rc.1`) gate to a 1.0.0 (stable-promotion) gate, at the user's
suggestion: the soak test exercises the update/backup/restore machinery
generically, not one specific build, and an RC period is exactly what
longer-running validation like this is for - there's no reason to hold
`1.0.0-rc.1` itself hostage to the calendar. 0.9 is now 5/7 with only two
non-time-bound items left (final blocker sweep, migration docs).

**Same PR, second review round** (commit `17fafd1` reviewed independently
by the user, two more real gaps found):

- Different components recognized different subsets of valid SemVer:
  `install.sh`'s tag regex and the site scripts' `tag_version_pattern`
  both rejected a hyphen *inside* a prerelease identifier (`rc-one.1`),
  even though release-alpha.yml's gate and Core's `semver.IsValid` both
  accept it - a release using that shape could publish and be picked up
  by Core's updater while `install.sh` and the site silently kept
  ignoring it as "not a real release." Fixed by widening each identifier
  character class to `[0-9A-Za-z-]+` everywhere a *tag* gets recognized
  (not everywhere a *version gets minted* - release-alpha.yml's own gate
  stays strict on purpose, leading zeros and all, since that's the one
  place actually deciding what's allowed to exist).
- The "accepted gap" from the first round turned out to be a real,
  ongoing problem, not just a missing positive check: once site prose
  says a bare `1.0.0` with no letter-suffix, `check-site-facts.sh`'s old
  narrow pattern could never recognize *any* future bare mention again -
  so a later `1.0.0` → `1.0.1` patch bump would go completely undetected
  by CI forever after the first stable release, not just through the
  rc→stable transition. Fixed for real rather than re-documented: added
  `scripts/version-pattern.sh` (sourced by both site scripts;
  `install.sh` can't source it - no repo checkout exists yet on a fresh
  machine - so it keeps its own hand-synced copy of the tag grammar) with
  a `rootguard_extract_versions` helper that strips SVG `<path d="...">`
  coordinate data first (the original false-positive source), then keeps
  a dotted-number candidate only if it has exactly three groups with a
  ≤3-digit last group - cheap enough to reject a 4-group IP address and a
  `dd.mm.yyyy` date (both proven live to otherwise look exactly like a
  bare version) without needing lookahead, which check-site-facts.sh's
  own header already rules out for BSD/GNU grep portability. Verified
  live end to end: injected a fake stale `1.0.1` mention into `index.html`,
  confirmed `check-site-facts.sh` flags it, confirmed
  `bump-site-versions.sh` correctly fixes it and leaves an
  already-correct bare mention untouched, confirmed the real repo content
  still passes/no-ops afterward, all test edits reverted before
  committing.

**Same PR, third review round** (commit `9555e67` reviewed independently
by the user as an explicit RC-readiness check ahead of triggering
`1.0.0-rc.1`, three real findings plus one acknowledged-low-priority one):

- `release-version-bump.yml` (the "Cut next release" workflow a human
  runs by hand, optionally with a manual version override) accepted the
  override completely unvalidated, then immediately ran `git-cliff`,
  committed `CHANGELOG.md` to `main`, pushed a real tag, and created a
  real GitHub Release - only the separately-dispatched
  `release-alpha.yml` validated the version, by which point a typo'd
  override would already have left a half-published release needing
  manual cleanup. Exactly the risk that mattered right before triggering
  the actual RC by hand with an override. Fixed by validating with the
  same SemVer 2.0 grammar as release-alpha.yml's own gate, immediately
  after `next_version` is computed and before anything external happens -
  duplicated rather than shared (no simple way to share a bash snippet
  across two separate workflow files without a composite action).
- Same workflow's GitHub Release creation always passed `--prerelease`
  unconditionally - correct for every version so far (all had a
  suffix), silently wrong for a future bare stable release. Now
  conditional on whether the version string contains a `-`.
- `install.sh`'s `sort -V` + `-`→`~` precedence workaround (from the
  first review round) turned out not to be genuine SemVer precedence in
  every case: a prerelease identifier containing its own literal hyphen
  ("rc-one.1") sorted *below* the plain "rc.1" it should rank above,
  because the trick only ever repositions the version-core/prerelease
  separator, not every hyphen SemVer identifiers are allowed to contain.
  Doesn't affect the actually-planned `beta.N` → `rc.N` → stable series.
  Replaced with a real, from-scratch SemVer 2.0 precedence comparator in
  pure bash (`version_gt` + helpers) - numeric version-core comparison,
  then prerelease-identifier-by-identifier comparison per spec (numeric
  identifiers compare numerically and always rank below alphanumeric
  ones; more identifiers with all preceding ones equal ranks higher) -
  rather than trying to make `sort -V`'s undocumented ordering behavior
  cover a case it was never designed for. Verified against SemVer.org's
  own canonical precedence example chain, the specific reported bug case,
  and a live call against the real GitHub API (still correctly resolves
  `0.1.0-beta.14` as latest).
- Acknowledged, left as is: `scripts/version-pattern.sh`'s website
  extractor rejects a patch/build number over 3 digits (to keep excluding
  a `dd.mm.yyyy` date's 4-digit year, the whole reason that check exists)
  - flagged by the user themselves as low-priority and "practically far
  away," not something to chase now.

**Same PR, fourth review round** (commit `deb46eb`, explicit RC-readiness
check ahead of triggering `1.0.0-rc.1` - user's conclusion: no blocker
left for that specific version, branch mergeable, both findings below
non-blocking):

- `release-version-bump.yml`'s own "find the previous tag" step (used to
  pick `git-cliff`'s changelog range) still used the pre-fix, hyphen-less
  regex - the one spot that hadn't been updated when every *other*
  tag-recognition site (release-alpha.yml's gate, Core, the updater,
  `install.sh`) already accepted a hyphenated prerelease identifier like
  `rc-one.1`. Widened to match, for consistency - doesn't affect the
  actually-planned `beta.N`/`rc.N` series either way.
- `install.sh`'s new `version_gt` comparator used bash's native
  `((10#$a < 10#$b))` arithmetic for numeric-identifier and version-core
  comparisons, which silently overflows for a value beyond bash's integer
  range - SemVer itself never caps a numeric identifier's size. Replaced
  with `numeric_compare`: strips leading zeros, then decides by digit
  *count* first (no leading zeros left means length alone is decisive),
  falling back to a lexical compare only once lengths already match
  (numerically correct at that point) - arbitrary-precision, no
  conversion to a native integer anywhere. Verified with a 36-digit
  identifier and a 20-digit major-version component, plus the full
  canonical SemVer.org chain and every previously-reported case again, to
  confirm nothing regressed.

**PR #359 merged** (2026-08-26, commit `ab6b1bf`) after four independent
review rounds - final `gh pr checks 359` confirmed fully green on the
merged commit before merging.

**`v1.0.0-rc.1` cut and published same day.** Final `gh issue list` sweep
right before cutting: 4 open issues, none release-blocking (see
`ROADMAP.md`'s 0.9 section for the per-issue reasoning). Triggered via
`release-version-bump.yml`'s `version` override; tag, GitHub prerelease,
and all five component images published cleanly - but `upgrade-test`
failed on the first run, for exactly the reason `release-alpha.yml`'s own
comments already warned about: `beta.14 -> 1.0.0-rc.1` is the *first*
jump onto the release that ships PR #359's SemVer generalization, so
`beta.14`'s still-running, pre-fix Core resolved "latest" back to itself
instead of `1.0.0-rc.1` (`update_available: false`) - the identical
bootstrap class as the earlier `beta.12 -> beta.13` failure, just
triggered by a different fix this time. The existing workaround only
special-cased `previous == 0.1.0-beta.12`; generalized properly this time
(PR #360, merged same day) - `upgrade-test` now always calls the updater
with an explicit `target_images` override instead of trusting whichever
previous release's Core to resolve "latest" correctly, since this job
already knows exactly which version it's testing and doesn't need live
discovery at all. Re-ran `release-alpha.yml` for `1.0.0-rc.1` (images
already published and immutable, so only `upgrade-test`,
`update-alpha-pins`, and `smoke-test` did real work this time) - fully
green. [`v1.0.0-rc.1`](https://github.com/foxly-it/rootguard/releases/tag/v1.0.0-rc.1)
is out, correctly marked as a GitHub prerelease, site pinned to it.

RootGuard is now in the `0.9` exit state: "only bug fixes and
documentation" until the two remaining 1.0.0-gating items close (the
30-day soak test, `rootguard#271`, and the migration/rollback docs that
wait on 1.0.0's scope being final).

**The `upgrade-test` failure above isn't only a CI artifact.** Verified
directly against the `v0.1.0-beta.14` source: its Core's
`releaseTagPattern` is `^v0\.1\.0-(alpha|beta)\.([0-9]+)$` - hard-limited
to the pre-#359 scheme, so `v1.0.0-rc.1` isn't merely ranked wrong, it's
completely invisible to `pickLatestReleaseImage`. Every installation still
running `beta.14` or earlier (i.e. every installation, since that pattern
was unchanged since `alpha.2`) has the identical blind spot: its WebGUI
"check for updates" will never surface `1.0.0-rc.1` at all. Structurally
unfixable after the fact (the fix ships *in* the release that needs it),
same as the `beta.12` case earlier - except this is the actual 1.0-line
transition, not an interim beta bump, so it warranted more than "note it
and move on." Added a warning callout plus the exact one-time
`docker exec rootguard-core wget ... /api/control-plane/update` bootstrap
command to `docs.html`'s Updates & Rollback section (the same escape
hatch already used for `beta.12`, now public and documented rather than
only living in this file). Also widened `check-site-facts.sh`/
`bump-site-versions.sh`'s `historical_reference_pattern` with a "bis
einschließlich"/"up to and including" marker (mirroring the existing
"Ab"/"Starting with" one, just for the opposite direction) so the new
callout's `0.1.0-beta.14` mention doesn't get flagged as a stale
current-version claim - verified the callout's `1.0.0-rc.1` mentions
still get checked and will auto-update via `bump-site-versions.sh` on
every future release, same as everywhere else on the site.

## Full security audit against 60dc447 (2026-08-28)

User ran a complete independent audit of the codebase at commit `60dc447`
(the RC.1 tree plus the site-polish work above). Found 3 release-blocking
issues, 7 medium findings, and several minor ones - Claude verified each
high-severity finding directly against the code before agreeing, all
confirmed accurate. User chose to work the full list in priority order,
one issue/PR each, same discipline as always.

**Blocker 1, fixed: attestation was never enforced on activation, only
displayed.** `stack.CheckStackAttestations` was wired into the stack
status API only (`routes.go`'s `stackStatusHandler`/`servicesHandler`),
never into either updater's actual `update()` path - a pulled,
digest-resolved image was activated (`selectImage`/`writeOverride` +
`compose up`) the moment its post-swap health check passed, with no
attestation check anywhere in between. Directly contradicted
`docs/threat-model.md`'s explicit claim that releases are "checked via
Cosign against the signed SLSA provenance before activation" - a real gap
between documented and actual behavior, not just a missing feature.

Fixed in both updaters:
- `rootguard-core/internal/stack/attestation.go`: new exported
  `RequireAttestation(ctx, service, image) error`, wrapping the existing
  `verifyReleaseAttestation` - no-ops for a service with no RootGuard
  signing policy (AdGuard, a third-party image), demands literally
  `"verified"` and nothing else (not `"missing"`, `"failed"`,
  `"unavailable"`, or an unexpected `"not_applicable"`) for every other
  service.
- `rootguard-core/internal/updater/manager.go`: new injectable
  `Options.AttestationVerifier` (defaults to `stack.RequireAttestation`),
  called right before `selectImage`/`composeUp` - the actual point of no
  return, not just logged for display.
- `rootguard-updater/main.go` + new `attestation.go`: a separate Go
  module, can't import Core's internal package, so it carries its own
  minimal standalone cosign-verify-attestation call (core/webapp are the
  only two services it ever manages, both always requiring verification -
  no AdGuard-style exemption needed here). Injectable
  `manager.attestationVerifier`, defaults to the real `verifyAttestation`.
- `rootguard-updater/Dockerfile`: didn't carry the `cosign` binary at all
  before this - added the same digest-pinned
  `ghcr.io/sigstore/cosign/cosign:v3.0.6@sha256:de9c65...` COPY step Core's
  Dockerfile already uses. Built for real on the `.7` test host (no local
  Docker daemon) to confirm the multi-stage COPY actually works and
  `cosign version` runs inside the image - it does.

Regression tests in both modules prove the gate has teeth: a failing
verifier must stop the update before any `compose`/activation command
runs at all, not just fail afterward - verified by temporarily reverting
each fix and confirming the new test fails without it, then restoring.
Existing tests that exercise a full successful `update()` needed an
explicit no-op `AttestationVerifier` injected (they use fixture image
names that don't match a real `ghcr.io/foxly-it/...` prefix, which the
real, now-default verifier correctly refuses as `"not_applicable"` -
fail-closed, working as intended, just not what those particular tests
were testing).

**Found by PR #365's own CI, not by local testing**: `ci-updater.yml`'s
real Docker E2E integration test (`integration/run.sh`, a genuinely
separate thing from the Go unit tests above - builds real local fixture
images and runs the real compiled binary through a real paired
core/webapp update+rollback) failed the same way, for the same reason -
its fixture images (`rootguard-e2e-core:new`, no `ghcr.io/foxly-it/...`
prefix, and with `ROOTGUARD_UPDATER_SKIP_PULL=true` already set, never
even digest-qualified) correctly fail the new attestation gate every
time, so the update can never reach `idle`. Fixed with the same kind of
test-only escape hatch `ROOTGUARD_UPDATER_SKIP_PULL` already is -
`ROOTGUARD_UPDATER_SKIP_ATTESTATION`, read once in `main()`, set only in
`integration/compose.e2e.yaml`. Documented plainly in the code that this
one is a real security control, not just a convenience skip like
`SKIP_PULL` - anything setting it in production loses the whole point of
this fix. Verified the exact env-var-driven override logic in isolation
(a standalone Go snippet mirroring `main.go`'s wiring, run with and
without the variable set) since the shared `.7` test host's own
long-running containers made a real `integration/run.sh` pass there
unsafe - fixed `container_name:` values in `compose.e2e.yaml` would have
collided with (and risked disrupting) an unrelated 3-day-old deployment
already running there.

**Blocker 2, fixed: releases became publicly visible before smoke-test/
upgrade-test passed.** `update-alpha-pins` (pin commit + moved release tag)
ran with `needs: [version, publish]` - a sibling of, not gated behind,
`smoke-test`/`upgrade-test`. The GitHub Release itself was created even
earlier, by `release-version-bump.yml`, before `release-alpha.yml` had even
started. A failing `upgrade-test` (which has happened live, for exactly
this reason, during the `1.0.0-rc.1` cut) still left the final image tags,
a real GitHub Release, committed pins on `main`, and a moved release tag
all publicly visible - and since Core's/`install.sh`'s live release
discovery both query the Releases API (not raw git tags), a
published-but-broken Release was immediately eligible for auto-discovery
by any existing installation checking for updates.

Fixed: `release-version-bump.yml` no longer creates the GitHub Release (it
still tags + pushes the version-bump commit itself, a low-risk internal
history marker, not a publicly promoted artifact). `update-alpha-pins` now
`needs: [version, publish, smoke-test, upgrade-test]` and creates the
GitHub Release itself as its last step, after the tag has already been
moved to the correctly-pinned commit - notes are regenerated with
git-cliff right there (deterministic from commit history, so identical to
what the other workflow would have produced) rather than passed across the
two separate workflow runs. Both the tag-move and the new Release-creation
step stay idempotent, matching the rest of this job, since a re-run against
an already-published version (exactly what happened live for `1.0.0-rc.1`)
must not fail just because an earlier run already did the work.

Also closed the two smaller gaps the same review flagged in this job:
`docker compose -f compose.release.yaml config` (plus the same
digest-pinned/no-`:latest` assertions `ci.yml`'s own PR check already
runs) now validates the pin-sed's output before it's committed - the
`[skip ci]` pin commit's own comment already documented a *real* past
incident where a broken sed match silently shipped invalid YAML because
nothing re-checked it. And the release pipeline's own `test` job gained
`go vet` (all three Go modules) and `npm test` (the frontend unit suite,
previously not run there at all, only by `ci-webapp.yml`'s own PR checks -
a release cut has no PR).

Deliberately deferred, not folded into this fix: making the release
pipeline depend on `ci-security.yml`'s scans (trivy/gitleaks/govulncheck/
staticcheck) passing. They already run on every push to main via that
separate workflow, but - found while investigating this - `main` isn't
actually a GitHub-protected branch (`gh api .../branches/main/protection`
returns 404), so that's convention, not an enforced gate. Wiring a
security-scan dependency into the release pipeline for real means turning
`ci-security.yml` into a reusable `workflow_call` workflow, a larger,
separate change; flagged to the user rather than rushed into this PR.

**Follow-up (2026-08-29):** done - see the "release gate" entry further
down under "Follow-up review, round 2" for the actual `workflow_call`
conversion and the new `security` job in `release-alpha.yml` that
depends on it.

**Blocker 3, fixed: the one-line installer ran an unpinned, unverified
script as root.** `install.sh`'s `install_docker()` downloaded
`dockerinstall.sh` from `foxly-it/dockerinstall`'s moving `main` branch
and immediately ran it via `sudo`, with no digest, signature, or checksum
check at all - a compromised commit to that separate repository would
have been live on every fresh RootGuard install within minutes. That repo
has no tags or releases to pin to instead (confirmed via the GitHub API),
so there's no "latest stable version" to point at.

Fixed with the two-part guarantee the review itself recommended: pinned
`DOCKERINSTALL_URL` to a specific commit SHA (fixes *which* content is
expected - a real git commit's content is immutable, unlike a branch tip)
plus a hardcoded `DOCKERINSTALL_SHA256`, checked against the actual
download before it's ever `chmod +x`'d or executed (the part that
actually verifies what was *received* still matches that commit - a
commit-pinned URL alone doesn't protect against a compromised CDN edge,
DNS spoofing, or a tampered mirror serving something else at the same
URL). Verified both directions live against the real pinned URL: the
real download's checksum matches, and a deliberately wrong expected value
is correctly rejected. Documented directly in the script how to
intentionally update the pin later (bump the commit SHA, recompute the
checksum from that exact URL).

## Medium findings from the same audit

**Medium 1, fixed (the digest-pinning half): blockpage wasn't
reproducibly bound to a release.** `release-alpha.yml`'s publish job
explicitly excluded blockpage from digest capture
(`if: matrix.component != 'blockpage'`), and neither
`compose.release.yaml` nor `.env.release.example` referenced
`ROOTGUARD_BLOCKPAGE_IMAGE` at all - Core's own Go default
(`ghcr.io/foxly-it/rootguard-blockpage:latest`) was always what actually
ran. A fresh install could get a blockpage image from an entirely
different commit than the rest of the release, with no digest pin and no
attestation ever checked for it (unlike core/webapp/unbound/updater,
which already all have real signing policies in
`attestationPolicies["blockpage"]` - that policy existed already, it was
just never reachable because nothing pinned a digest-qualified reference
to check it against).

Fixed: `release-alpha.yml` now captures blockpage's digest like every
other component, `rootguard-blockpage` was added to the pin-rewrite loop,
and both `compose.release.yaml` and `.env.release.example` now pin
`ROOTGUARD_BLOCKPAGE_IMAGE` explicitly (with today's real digest,
verified live via `docker buildx imagetools inspect`). The `docker
compose config` digest-pinned assertion (both the copy in `ci.yml` and
the one added to `release-alpha.yml` for the earlier release-gate fix)
now checks it too, so the two can't drift apart.

**Deliberately deferred, not folded into this fix** (two related but
separable pieces the same finding raised):
- Full dashboard/attestation-status visibility for blockpage
  (`StackStatus` has no `Blockpage` field at all yet - adding one means
  wiring container inspection, health, and attestation status through
  the API and frontend end-to-end, a materially bigger change than
  closing the actual "unpinned mutable image" security gap above).
- The architectural fix for blockpage holding a reversible (base64,
  trivially decodable) copy of the real AdGuard admin credentials
  (`publishBlockpageAuthToken` in `rootguard-core/internal/adguard/
  manager.go`) - the review's own recommendation frames this as a
  longer-term redesign (a narrow Core-side endpoint the blockpage
  queries instead of ever holding real credentials at all), not a
  same-size fix as the others in this list.
  **Follow-up (2026-08-29): done** - see the "blockpage no longer holds
  any AdGuard credential" entry further down under "Follow-up review,
  round 2" for the actual Core-side endpoint and service-token redesign.
**Medium 2, fixed: expert config could disable DNSSEC entirely.**
`blockedDirectives` in `rootguard-core/internal/unbound/custom.go`
blocked `val-permissive-mode` and the trust-anchor directives, but not
`domain-insecure` at all - a bare `domain-insecure: "."` disables DNSSEC
validation for the whole namespace, not just one zone. Separately,
`harden-dnssec-stripped: no` (accepting a stripped-DNSSEC-data attack
instead of treating it as an error) only produced a soft "warning"
advisory, lumped in with cosmetic settings like `hide-identity` - a user
could activate it anyway. Both directly contradicted `docs.html`'s own
claim (written earlier this same session) that the expert editor
"blocks... DNSSEC bypasses".

Fixed: `domain-insecure` added to `blockedDirectives` (doesn't affect the
guided private-domain/reverse-DNS feature, which renders this directive
itself from validated Go code - a separate path from this free-text
editor entirely); `harden-dnssec-stripped: no` specifically (not the key
unconditionally - `: yes`, the recommended default, must stay accepted)
is now a hard `normalizeCustom` rejection instead of an `adviseCustom`
warning, removed from that warning's case since it's now unreachable
there. New regression test proves both refusals have teeth (reverted
each, confirmed the test fails, restored) and that `harden-dnssec-
stripped: yes` still passes.

**Medium 3, fixed: logout could be silently undone by a restart.**
`handleLogout` only cleared the browser's session cookie *after*
`persistLocked` succeeded - a persist failure returned a 500 with the
cookie still live, even though the comment right there claimed it was
"already gone" (false: the delete-cookie call was below the early
return, unreached). If a stale `sessions.json` then revived that exact
session on the next restart, the browser would still hold a valid
cookie for it, undoing a logout the user had already been told
succeeded. `handleRevokeSession` had the same durability gap for an
*admin*-revoked foreign session (no cookie involved there, but the same
"in-memory revoked, not-yet-persisted, restart brings it back" window).

Fixed: the cookie-clearing call in `handleLogout` now runs
unconditionally, before the persist attempt - the browser-side effect of
logout is durable regardless of what happens to the server-side record.
`handleRevokeSession` gained a bounded retry (3 attempts, 50ms apart)
around its persist call, closing the common transient-I/O-hiccup case;
a permanently broken write still can't be made durable by retrying, so
it still surfaces as an error rather than a silent success, and the
revoked session still stays removed from memory immediately either way.
Two new regression tests (a real broken-path filesystem trick: point
`persistencePath` through a file that was just written, so `MkdirAll`
reliably fails on any OS) prove: logout's cookie is cleared even when
persistence fails (reverted, confirmed the test fails, restored), and a
broken-persist revoke still returns a clear error while the session
stays gone from memory.

**Medium 4, fixed: a corrupted credentials.json silently reactivated the
old env password.** `loadCredentials` treated every failure identically
- a genuinely missing file, an unreadable one, corrupt JSON, an
unexpected algorithm, a malformed salt/hash - as a silent no-op that
left the env-configured initial username/password active. Correct for a
fresh install that never changed its password (file genuinely absent),
wrong for real corruption: an admin who'd deliberately changed the
password in the UI would have that change silently, invisibly reverted
to the original env password on the next restart - a real risk if that
original password was ever exposed (an old `.env` backup, a log,
whoever was present during initial setup).

Fixed: `loadCredentials` now returns an error, distinguishing
`os.ErrNotExist` (the one legitimate silent case) from every other
failure (returns a real error). `NewSessionAuth` panics on that error
the same way it already does for a password-hashing init failure -
consistent with this constructor's own existing "fail loudly at
startup" convention, not a new one. Two of this package's existing
tests deliberately made `credentials.json` a directory *before*
constructing `SessionAuth` (to force a later *persist* failure) - moved
that setup to *after* construction in both, since it would otherwise now
correctly panic during construction itself (a directory where a file
should be is exactly the kind of corruption this fix is meant to catch).
New regression test proves the panic has teeth (reverted, confirmed the
test fails, restored) alongside a companion positive test confirming a
genuinely missing file still doesn't panic.

**Medium 5, fixed: the destructive-action rate limit was bypassable under
concurrency and shared across an account's sessions.** `guardDestructive`
had the same `blocked()`-then-`recordFailure()` TOCTOU gap already fixed
for login/recovery this session - many truly concurrent requests could
all observe zero recorded uses and all be admitted before any got
counted. Separately, it keyed the limiter by *username*, so every session
the same admin account happens to have open (session inventory
explicitly allows more than one) shared a single combined budget -
directly contradicting the limiter's own documented purpose ("bound how
much a single... session can do", right there in its own construction in
`NewSessionAuth`).

Fixed: `guardDestructive` now uses `beginAttempt`/`endAttempt` (every
attempt counts, matching its pre-existing "bound request volume, not
repeated wrong guesses" semantics), keyed by a new `authenticatedSessionID`
helper (the session's own opaque `.ID`, already designed to be safe to
hand out, not the bearer token itself) instead of username, falling back
to the IP-based key only when there's genuinely no session. New
per-session regression test proves the keying fix has teeth (reverted,
confirmed the test fails with a 429 that should have succeeded,
restored). The concurrency test is honest about its own limits: unlike
login/recovery's PBKDF2-widened window, there's no expensive work between
the old two calls here, so Go's scheduler rarely interleaves into that
gap in a synchronous test even at high goroutine counts - confirmed the
race is real a different way instead (a throwaway direct probe against
the raw rate limiter, bypassing the HTTP layer, showed extra accepted
attempts in roughly 1 of 3 runs).

**Medium 6, fixed: Setup preflight didn't check the blockpage's port 80.**
`Preflight` only ever checked the DNS port; a host with something already
bound to :80 (a common case - many hosts run a web server) was reported
"ready", then failed only later, during deployment, when the blockpage
container's own port publish (`composeDNSFile`, gated on
`config.BlockpageEnabled`) collided with it.

Fixed: `Preflight` now runs the same `occupiedDockerPort`/
`probeHostPortBusy` pair it already uses for the DNS port against port 80
too, gated on `config.BlockpageEnabled` (the same flag that decides
whether the blockpage - and therefore that port publish - exists at
all), with its own `blockpage_port_occupied`/`blockpage_port_available`
check codes and a blockpage-specific action message. Two new regression
tests: one proves the occupied-port case fails preflight (reverted,
confirmed the test fails, restored), the other proves a disabled
blockpage doesn't fail preflight just because something else happens to
hold port 80.

**Medium 7, fixed: the WebApp sent no browser security headers.** Every
response - the SPA, the API, the proxied AdGuard UI - carried no
`Content-Security-Policy`, `X-Content-Type-Options`, `X-Frame-Options`,
`Referrer-Policy`, or `Permissions-Policy`. None of this is the primary
defense against anything (that's `RequireSameOriginWrites` and
`SessionAuth`), but its absence meant a same-origin script-injection bug
anywhere else in the app would have had no browser-side backstop, and the
admin UI could be framed by another site.

Fixed: a new `SecurityHeaders` middleware, wrapped outermost around the
whole router in `main.go`, sets all five. The CSP's `script-src` is
same-origin plus a hash for the one inline script the SPA actually has
(`frontend/index.html`'s theme-flash-prevention script, which can't be
externalized without reintroducing the flash it exists to avoid) -
locked down, since script execution is the directive that actually
matters for XSS. `style-src` keeps `'unsafe-inline'` deliberately: a
handful of components apply the React `style` prop, which CSS-in-JS
research shows browsers don't reliably attribute to CSP's HTML-attribute
style-src restriction the same way across versions - not worth risking a
silent rendering break in the field to tighten a directive that isn't
the injection-relevant one. The CSP is withheld specifically for
`/adguard-ui/` (AdGuard Home's own reverse-proxied admin UI, a separate
frontend whose asset layout this app doesn't control); the other four
headers still apply there.

Three new tests: the headers are actually set (and withheld for
`/adguard-ui/`, both revert-verified - reverted to a no-op middleware,
confirmed both tests fail, restored), plus a drift guard that hashes the
real `frontend/index.html` inline script at test time and fails if it no
longer matches the hardcoded CSP hash, so a future edit to that script
can't silently start breaking under the new CSP.

Not verified in a live browser (no local Docker daemon, same constraint
as the rest of this session) - worth a manual DevTools-console check for
CSP violations on the actual built SPA before the next release.

**Medium 8, fixed (audit rated it "gering"/low): router-import allowed
SSRF via redirects and arbitrary addresses.** `FritzBoxClient` only ever
gave `address` a syntax check (`normalizeRouterAddress`); it was never
restricted to plausible router locations, and its `http.Client` followed
redirects with its default behavior. An admin (or anything able to drive
this endpoint through them) could point host discovery at any reachable
address, public or otherwise, or have a redirect steer the request - with
its Digest `Authorization` header intact on a same-host redirect -
somewhere else entirely.

Fixed with the two changes the audit itself suggested: `CheckRedirect`
now refuses every redirect outright, and a custom `DialContext` checks
the *actually-dialed* remote IP - not the pre-resolution hostname, which
closes a DNS-rebinding gap a string-only check would leave open - against
private/link-local/loopback ranges only, rejecting anything globally
routable. Five new tests: the happy path still works through both new
guards (a real `httptest.Server` on loopback), redirect rejection is
revert-verified against a real HTTP redirect, and the address-range logic
is covered directly (a table test on the pure range-check function, and a
dial-level test using a fake `net.Conn` with an arbitrary `RemoteAddr` so
rejecting a public IP doesn't depend on outbound network access in CI) -
both of those also revert-verified.

**Medium 9, fixed: GitHub Actions supply-chain and permission hygiene.**
Every third-party action was pinned by a mutable major-version tag
(`@v7`, `@v4`, ...) rather than a commit SHA; two scanner installs
(gitleaks, trivy) downloaded a binary over HTTPS with no checksum
verification, and trivy's installer additionally came from the `main`
branch of a script - a genuinely moving target, re-fetched fresh (and
potentially different) on every run; `govulncheck`/`staticcheck` ran via
`@latest`, so a new upstream release could change CI behavior with zero
review; and several workflows granted `packages: write`/`id-token: write`/
`attestations: write` at the workflow level, so their test-only jobs
inherited registry-write and OIDC-attestation capabilities they never use.

Fixed: every `uses:` across all 15 workflow files now pins a full commit
SHA with the original tag kept as a trailing comment. `govulncheck` and
`staticcheck` are pinned to specific versions (`v1.7.0`/`v0.8.1`, the
versions already in use) instead of `@latest` - `go run pkg@version` also
gets Go's own module checksum verification (GOSUMDB) for free, unlike a
raw download. gitleaks and trivy are now installed by downloading the
exact release asset directly (bypassing trivy's third-party install
script entirely) and verifying its sha256 against a hardcoded hash before
extracting; git-cliff's existing download (both release workflows) gained
the same checksum check. Elevated permissions moved from workflow-level
to job-level in `ci-unbound.yml`, `ci-updater.yml`, `ci-webapp.yml`, and
`release-alpha.yml`'s `publish` job (the only jobs that actually push
images or attest), with `smoke-test`/`upgrade-test` getting `packages:
read` (they pull published images but never push). `ci-blockpage.yml` and
`pages.yml` are single-job workflows where the one job both builds and
(conditionally) publishes - permission scoping doesn't help there without
splitting into a build/publish pair with an image handoff between them, a
larger restructure left for later since the practical exposure is low
(fork-PR runs already get a read-only token from GitHub regardless of
what a workflow declares, and same-repo runs push in that same job
anyway).

**Minor 1, fixed: backup-restore's entry-limit error message overclaimed
what was found.** The guard rejects at `count >= MaxFiles`, i.e. as soon
as `MaxFiles` entries have already been read successfully - without ever
checking whether a real `MaxFiles+1`th entry exists (that's correct: see
the surrounding comment for why `>=` and not `>`, from an earlier fix
this session). But its message said "backup contains more than %d
entries", which isn't always true at that point - an archive with
*exactly* `MaxFiles` entries and no more triggers the same rejection with
the same overclaiming message. Reworded to "backup contains too many
entries (limit: %d)" - accurate regardless of which case triggered it.
New regression test builds an archive with exactly `MaxFiles` (100000)
entries and checks both that it's still rejected and that the message no
longer says "more than" (revert-verified).

**Minor 2, fixed: the AdGuard UI proxy's Set-Cookie rewrite was a fragile
string match.** `rewriteAdGuardUIResponse` repoints AdGuard's root-scoped
session cookie (`Path=/`) to this proxy's own mount point
(`/adguard-ui/`), so the browser keeps sending it back on later requests.
It did that with `strings.Replace(cookie, "Path=/;", ...)`, which missed
every one of: `Path=/` as the *last* attribute (no trailing semicolon - a
completely ordinary, spec-legal shape), different attribute-name casing
(`path=/`), and extra whitespace around the attribute.

Fixed: `rewriteAdGuardSetCookie` now parses the cookie properly via
`http.ParseSetCookie` (net/http already implements RFC 6265 for this)
and only touches the `Path` field, instead of substring-matching the raw
header text - which also means every other attribute (`Secure`,
`HttpOnly`, `SameSite`, `Expires`, ...) survives untouched regardless of
where `Path=/` appears in the string. A cookie whose path isn't exactly
`/`, or that fails to parse at all, is passed through unchanged - the
same scope the old code had, just reliably. Two new tests cover the
realistic variance the audit named (all revert-verified against the old
implementation) and confirm the "leave it alone" cases still work.

**Minor 3, fixed: the blockpage used innerHTML with location.hostname
unnecessarily.** The block-reason renderer built its "lead" paragraph via
`element.innerHTML = "...<strong>" + location.hostname + "</strong>..."`.
`location.hostname` can't practically carry HTML-special characters (DNS
hostname syntax and browser URL parsing both rule that out), so the
audit rated this low practical exploitability - but it's still needless:
nothing about this line needs raw HTML construction.

Fixed: rebuilt via `textContent`/`createElement`/`appendChild`, the same
pattern this file already uses two lines above for the headline's
reason span - no `innerHTML` anywhere in the reason-rendering path
anymore. (The one other `innerHTML` use in this component, `theme.js`'s
icon swap, stays as-is: it only ever assigns one of three hardcoded SVG
strings keyed by an internal `mode` value, never anything
request-derived.) No automated test covers this specific rendering path
- this component has no JS test harness at all (a static
Node.js-syntax-checked, curl-smoke-tested-only script), and building one
for a single low-severity hygiene fix wasn't judged worth the new CI
surface; verified by inspection and by `node --check` for syntax only.

**Minor 4, fixed: install.sh wrote the WebGUI username/password into
.env unquoted.** The awk rewrite that injects the generated tokens and
typed credentials into the downloaded `.env` file printed each value
raw. Docker Compose re-reads that file later and treats an unquoted `#`
as starting a comment - a password containing one would get silently
truncated, corrupting it without any visible error - and a literal
newline as starting a new `KEY=VALUE` line, which a value supplied via
`ROOTGUARD_ADMIN_PASSWORD` in a non-interactive install could exploit to
inject an extra variable into `.env` entirely.

Fixed two ways: every value the awk script writes is now double-quoted,
with any backslash or embedded double-quote in the value itself escaped
first so the quoting can't be broken out of (verified locally against
values containing `#`, embedded quotes, and backslashes - all come out
correctly preserved inside the quotes). Separately, `admin_user`/
`admin_password` are now rejected outright if either contains a literal
newline - not relied on the quoting alone to contain one, since whether
a dotenv-style parser treats a quoted value as continuing past a
newline varies enough across implementations that it wasn't worth
trusting for something that becomes an actual account password. A typed
value can never contain a newline in the first place (`read -r` stops
at the first one); this only matters for the non-interactive,
environment-supplied path.

No CI coverage exists for install.sh's own logic at all (the existing
`clean-install.yml` workflow exercises `compose.release.yaml`/
`.env.release.example` directly, never invokes `install.sh` itself) -
this is a pre-existing gap, not introduced or widened here, and out of
scope for this fix. Verified locally: `bash -n` syntax check, and the
awk pipeline and newline-rejection logic run standalone against a set
of adversarial values (`#`, embedded `"` and `\`, an injected newline).

**Minor 5, fixed: persistence errors were swallowed via `_ = persist...`
in many places.** ~20 call sites across `rootguard-core`'s installer and
updater managers (`_ = m.persistLocked()`) and `rootguard-webapp`'s
session/audit stores (`_ = a.persistLocked()`, `_ = a.persistAuditLocked()`)
discarded the write error entirely. On a full disk or a permissions
problem, an install step, update, cleanup, rollback, session change, or
audit entry could report success while its outcome was never actually
written down - invisible anywhere, not even in the container's own logs.

Fixed at the single choke point each package already had rather than at
every call site: `persistLocked` (both `installer` and `updater` in
`rootguard-core`) now takes an injectable `OnPersistError` hook -
defaults to a no-op, matching this codebase's existing
`CommandRunner`/`BootstrapFunc`-style option pattern - and calls it on
every failure before returning, so all ~15 existing `_ =` call sites get
visibility for free without themselves changing. `cmd/rootguard/main.go`
wires all three manager instances to log it. `rootguard-webapp`'s
`SessionAuth` doesn't have that same options-struct constructor (its
`NewSessionAuth` has a fixed positional signature used at 29+ existing
call sites, mostly in tests - not worth restructuring for this), so
`persistLocked`/`persistAuditLocked` log directly instead, at the one
place each already knows a write failed.

None of this changes what any caller does with the returned error or
what the UI reports on success/failure - it only makes an already-silent
failure mode observable in logs, closing the "invisible even to someone
troubleshooting a full disk" gap the audit described. A full fail-closed
redesign (surfacing every one of these to the UI, refusing to report
success at all) would be a much larger change or a longer-term goal,
matching how the same call was made for logout/session persistence
earlier this session.

Four new regression tests (two per repo, all revert-verified: reverted
each hook wire-up/log call, confirmed the corresponding test fails,
restored) using the same broken-path-through-a-file trick already
established this session (point a path through a file that was just
written, so `MkdirAll` reliably fails on any OS).

**Minor 6, fixed: the release tag's move to a pin-update commit
complicated forensic traceability.** An earlier RC-blocker fix this
session moved the release tag to point at the pin-update commit (the one
that records this release's own image digests in
`compose.release.yaml`/`.env.release.example`), so the documented quick
start resolves to the right files. But the published images are built
*before* that pin commit exists (the pin needs the digests the build
produces - chicken-and-egg), so each image's own
`org.opencontainers.image.revision` label and provenance attestation
reference the earlier build commit, not the tag. Someone tracing a
published image back to source without already knowing this would land
on the wrong commit.

Not fixable by skipping the tag move (that's the RC-blocker this would
revert to) or by building from the pin commit (still impossible for the
same chicken-and-egg reason) - recorded explicitly instead. The release
notes now get a "Build provenance" section naming both commits: the one
the images were actually built from (`github.sha`, matching their labels
and attestations) and the one the tag now points at (the pin-update
commit, one commit ahead). A one-line lookup instead of something to
reverse-engineer from git history.

Workflow-only change with no way to exercise a real release run in CI -
verified locally instead: the YAML parses, the appended-section shell
logic was run standalone against a fixture notes file and produces the
expected output.

**Code compression 1, done: a shared atomic-file writer.** The
audit's own lower-priority code-compression suggestions, worked through
after the full findings list: the write-temp-then-rename pattern was
hand-rolled separately in `installer`, `updater` (twice - one copy in
`manager.go`, another in `backups.go`), `unbound` (twice), and `adguard`
- six call sites within `rootguard-core` alone, and they'd genuinely
drifted: some cleaned up their temp file on a failed rename, some
didn't (a stray `.tmp` left behind on error); one used `json.Marshal`,
another `json.MarshalIndent`.

New `internal/atomicfile` package (`WriteFile`, `WriteJSON`) replaces
all six - only `adguard/manager.go`'s credentials write is left as-is,
since it deliberately delays its rename past an unrelated HTTP call
(a real two-phase-commit shape, not the same pattern). Behavior-
preserving: every existing test in the module passes unchanged, plus
three new tests for the new package itself (one a genuine regression
test - proves the failed-rename cleanup the consolidation fixed for
free, revert-verified).

`rootguard-updater` and `rootguard-webapp` keep their own small,
independent copies - they're separate Go modules from `rootguard-core`
(and from each other), and this codebase already established this
session that sharing code across that boundary means a new shared
module, real machinery for a handful of lines each already keeps
correct on its own.

**Code compression 2, done: split rootguard-updater/main.go (851 lines)
into files by concern.** Everything - HTTP wiring, the manager's whole
state machine, digest/version-resolution helpers, and raw docker-CLI
primitives - lived in one file. Split into `main.go` (156 lines: entry
point, env wiring, secret-strength check), `manager.go` (the `manager`
type, its persisted `status`, and every method - the actual update/
check/rollback state machine), `image.go` (`digestQualify`,
`digestFromPullOutput`, `isOlderReleaseVersion` - the same grouping as
the digest/SemVer logic this session's other compression item already
touched in `rootguard-core`), `http.go` (request/response glue), and
`docker.go` (the two raw `docker`/filesystem primitives) -
`attestation.go` already existed as its own file.

Scoped narrower than the audit's literal "separate packages" wording:
kept everything in `package main` rather than introducing real
sub-packages. This binary has no other consumer (a standalone
control-plane updater, not a library), so sub-packages would mean
export/visibility bookkeeping and import-cycle risk for zero reuse
benefit - multiple files in one package gets the actual readability win
this suggestion was about without that cost.

Purely a reorganization: every function's body is unchanged, byte for
byte, just relocated - confirmed by every existing test passing
unchanged (gofmt/go vet/go build/go test -race/staticcheck all clean),
which requires nothing about behavior or the public shape of any type
to have shifted.

**Code compression 3, done: shared SemVer validation between the two
release workflows.** `release-alpha.yml` and `release-version-bump.yml`
each kept a byte-for-byte identical SemVer 2.0 regex, deliberately - the
existing comment on the second copy said sharing "needs a composite
action" and wasn't worth it. That's not quite right: both jobs already
check out the full repo, so a plain shared shell script works without a
composite action. Worth fixing given the history the same comment
recorded: this exact duplication had already drifted apart for real once
- only `release-alpha.yml` validated a version at all until the second
check was added by hand, so a typo'd override had already produced a
real commit, tag, and GitHub Release before anything caught it.

New `scripts/lib/semver-validate.sh` (`require_semver "$value"`, exits
non-zero with a message on an invalid version) replaces both inline
copies; `release-alpha.yml`'s `version` job gained a checkout step it
didn't need before, purely to read this one file (a small, deliberate
trade - cheap CI overhead for one source of truth on a check with a real
past-incident history). `install.sh`'s own, much larger SemVer
comparator (real precedence comparison, not just format validation) is
untouched - it runs standalone via `curl | bash` with no repo access at
that point, so it can't source this file the same way; already
established this session as a case where duplication is the correct
trade, not an oversight.

Verified locally (no way to exercise a real release run in CI): the
regex's behavior is unchanged (same pattern, byte for byte) - checked
against every case the removed comments themselves named
(`1.0.0-rc.01`, `1foo.2bar.3baz`, `1.0.0+build`, `01.2.3`, plus valid
alpha/beta/rc/bare versions), all matching the prior inline behavior
exactly.

**Code compression 4, done: the last item on the audit's list -
cross-referenced (not merged) the digest-resolution duplication between
Core's updater and the control-plane updater.** `digestQualify` and
`digestFromPullOutput` are near-identical between
`rootguard-core/internal/updater/github_release.go` and
`rootguard-updater/image.go` - the audit's own suggestion was to share
this logic. Deliberately not done: the two are separate Go modules (this
session already established that pattern for the attestation verifier
and, again, for the atomic-file-write consolidation two items back), and
standing up a third shared module for ~30 lines of stable logic that
hasn't meaningfully drifted in practice wasn't judged worth the ongoing
versioning/import overhead.

What actually closes the audit's real concern (silent drift between the
two copies) without that cost: each function's doc comment on both sides
now names the other file explicitly and says to check it too on any
change - previously only loose, non-actionable prose ("a different Go
module, so its own copy") pointed at the duplication's existence at all,
with no precise pointer either way.

With this, all four of the audit's code-compression suggestions are
addressed - three with real consolidation (atomic-file writer, the
updater's file split, shared SemVer validation) and this one with an
explicit, deliberate "not worth a new module" call plus the
cross-referencing that keeps the trade-off honest going forward. Every
finding in the original audit, from the three RC-blockers down to this
lowest-priority suggestion, has now been worked through.

## Follow-up review, round 2 (2026-08-29)

A second, independent review of the round-1 fixes found genuine gaps in
several of them plus a few new issues - worked through the same way as
round 1: verify directly in code, scope the fix, test with real teeth,
document, PR, merge.

**Medium, fixed: the guided setup's first-ever DNS stack deploy skipped
attestation entirely.** `RequireAttestation` was wired into both updater
packages (round 1's first RC-blocker), but never into `installer.Manager`
- so a fresh install's very first Unbound/Blockpage activation, often the
only deployment event most installations ever have, pulled and started
those images with zero attestation check, contradicting
`docs/threat-model.md`'s claim exactly the way the original finding did
for updates.

Fixed with the identical pattern already used for both updater packages:
`installer.Manager` gained an injectable `AttestationVerifier` (defaults
to `stack.RequireAttestation`), called right before `compose up` in both
`deploy()` (fresh install) and `restoreDeploy()` (backup restore) - after
`pull`, at the actual point of no return. AdGuard stays unchecked
(no RootGuard signing policy, same as everywhere else); Blockpage is
checked only when `config.BlockpageEnabled`. `unbound`/`blockpage`
already had real signing policies registered in `stack.attestationPolicies`
from round 1's blockpage-pinning work - this was purely a missing call
site, not a missing policy.

Two new regression tests (`TestDeployRefusesActivationWhenAttestationFails`,
`TestRestoreRefusesActivationWhenAttestationFails`), both revert-verified
- reverting the call in either `deploy()` or `restoreDeploy()` made the
corresponding test hang until Go's test timeout rather than failing
cleanly, itself proof the gate is what unblocks the flow. Also added a
dedicated `attestation_failed` diagnostic code/message (previously an
attestation failure fell into the generic "could not be deployed"
classification, same bucket as an unrelated Docker error) and fixed four
existing tests that now correctly hit the real `stack.RequireAttestation`
default and need the same noop-verifier injection round 1's updater
tests already use.

**Follow-up to the fix above:** this PR's own CI caught what it broke -
`backup-restore.yml`'s "Verify backup export and restore" builds
`rootguard-core` locally from the checkout under test and deploys it via
the real installer path, so the new attestation gate correctly (if
inconveniently, for that test) rejected the unattested local build.
`cmd/rootguard/main.go` gained `ROOTGUARD_SKIP_ATTESTATION`, the same
shape and purpose as `rootguard-updater`'s own `ROOTGUARD_UPDATER_SKIP_ATTESTATION`
- disables every attestation gate this binary enforces (installation
deploy, service updates, updater self-update) uniformly, wired through
`compose.release.yaml` and set only in that one E2E job.
`clean-install.yml` (the other workflow deploying via `compose.release.yaml`)
needed no change - it deploys the real, published, genuinely-attested
images, not a local build, so it's an actual live check of the
attestation chain working end-to-end rather than something that needs
to bypass it.

**Unrelated cleanup, needed to unblock this PR's own CI:** `trivy`
flagged CVE-2026-56854 in `golang.org/x/crypto` (v0.52.0, an indirect
dependency, fixed in 0.55.0) - present in `main` already, not something
this PR introduced, but it fails `main`'s own security-scan gate for any
PR touching `rootguard-core` right now. Bumped via `go get
golang.org/x/crypto@v0.55.0 && go mod tidy` (pulled `golang.org/x/sys`
along with it); `rootguard-updater`/`rootguard-webapp/backend` don't
depend on it at all, so this is scoped to `rootguard-core` alone.

**Medium, fixed: build-time base images weren't digest-pinned.**
`golang:1.26-alpine`/`docker:29-cli` (`rootguard-core`, `rootguard-updater`)
and `node:22-bookworm`/`golang:1.26-bookworm` (`rootguard-webapp`'s two
builder stages) were still tag-only - `rootguard-unbound`,
`rootguard-blockpage`, cosign, and the webapp runtime's own
`gcr.io/distroless` base were already digest-pinned, this closes the
remaining gap. Matters even for a builder stage that never ships in the
final image: it's what compiled the shipped binary, so a repointed tag
(a compromised or re-pushed upstream image) could tamper with the build
toolchain itself without any change to this repo.

Digests resolved live against the real registry (`docker buildx
imagetools inspect <image> --format '{{.Manifest.Digest}}'`) and
independently re-verified a second time before committing, after an
early copy-paste slip in this same change corrupted one digest and was
caught by a `diff` between the two Dockerfiles' identical cosign pins
before it ever reached a commit.

**Medium, fixed: the release pipeline could build from, tag, or attribute
provenance to the wrong commit under a race.** Two distinct races, both
from trusting a mutable git ref re-resolved at a *later* moment than
when the intent to release was actually captured:

1. `release-version-bump.yml` dispatched `release-alpha.yml` with
   `--ref main` and no pinned commit. `github.sha` for that dispatched
   run resolves from whatever `main` is *when its own checkout actually
   runs* - not necessarily the commit `release-version-bump.yml` just
   committed and tagged, if anything else landed on `main` in the
   window between the two. Every job's checkout, the published images'
   `org.opencontainers.image.revision` label and `COMMIT` build-arg, and
   the release notes' own provenance section all inherited this
   ambiguity.
2. `update-alpha-pins`'s "Point the release tag" step re-fetched
   `origin/main`'s tip at that later point, independent of what its own
   preceding "Commit updated pins" step had just pushed - a second,
   narrower race window with the same shape.

Fixed by threading an explicit commit through instead of re-resolving a
ref: `release-version-bump.yml` now pushes commit+tag atomically
(`git push --atomic`, closing the window between the two ref updates
too) and passes the exact resulting SHA as a new `source_sha`
`workflow_dispatch` input; `release-alpha.yml`'s `version` job resolves
`source_ref` once (`inputs.source_sha || github.sha` - a manual dispatch
with no `source_sha` keeps today's behavior) and every other job's
checkout, the image labels, and the release notes all reference that
same value. The tag-move step now uses the exact commit
"Commit updated pins" just pushed (captured as that step's own output)
instead of re-fetching `origin/main`, falling back to the fetch only on
its pre-existing idempotent "nothing to commit" path, where there's no
"just pushed" commit to reference. Both workflows also gained a shared
`concurrency: group: release-pipeline` lock (the audit's own
recommendation) - previously absent entirely, so two overlapping release
attempts could have raced each other's pushes directly, a strictly worse
version of the same problem.

Workflow-only change with no way to exercise a real release run in CI -
verified locally: YAML parses on both files, `git push --atomic` tested
against a real local bare repo (both refs land in one transaction), and
the new conditional shell logic (prefer the captured commit, fall back
to a fresh fetch only when nothing was pushed) checked standalone.

**Medium, fixed: install.sh's double-quoted .env values still let
Compose expand `$` references in them.** Round 1 double-quoted every
value the awk block writes to `.env`, closing the `#`-comment-truncation
and newline-injection gaps - but double-quoted dotenv values still get
`$VAR`/`${VAR}` expansion from Docker Compose's own parser. Reproduced
live with `docker compose config`: a password of
`abc$HOME${MISSING}` came back out with `$HOME` expanded to the invoking
user's real home directory - the actually-running admin password could
silently differ from the one typed in.

Fixed by switching to single quotes (`q()` now just wraps the value in
`'...'`, no escaping needed) - single-quoted dotenv values are fully
literal, no expansion or escape processing at all, confirmed the same
way: `docker compose config` against a single-quoted
`abc$HOME${MISSING}` now returns it completely unchanged. The one
consequence of true literalness: a value can't contain an embedded `'`
at all (no escape mechanism exists in single-quoted dotenv syntax) - so
`admin_user`/`admin_password` are now rejected outright if they contain
one, alongside the existing newline rejection (which also now covers a
bare `\r`, the audit's own additional recommendation). Deliberately not
attempting a quote-concatenation escape trick for `'` (`'\''`-style) -
its correctness would depend on parser behavior across dotenv
implementations this script has no way to verify, unlike the
single-vs-double-quote literalness question, which was checked directly
against the real `docker compose config`.

No regression test added (this script has no test harness - the
existing gap noted in round 1's install.sh fix) - verified instead the
same way round 1's install.sh fix was: the awk pipeline run standalone
against adversarial values, and this time also against a *real*
`docker compose config` invocation (works daemonless, since it's pure
config resolution) proving both the single-quote fix and the
previously-double-quoted bug it fixes are real.

**Medium, fixed: the FritzBox SSRF guard checked the dialed address after
connecting, not before.** `dialPrivateOnlyWith` dialed first and inspected
`conn.RemoteAddr()` afterwards - real protection against the client
speaking TR-064 to a public address, but a genuine `connect(2)` to the
disallowed address still happened before the connection was torn down,
letting a caller distinguish an open port from a closed one on a host
this client should never have touched at all (a classic SSRF port-scan
oracle, even though the actual TR-064 request itself never got sent).

Fixed by moving the check into a `net.Dialer.Control` hook
(`rejectNonPrivateControl`), which the standard library calls after DNS
resolution but before the `connect(2)` syscall - a disallowed address is
now never actually contacted. DNS rebinding is still closed the same way
as before: `Control` receives the concrete, already-resolved `ip:port`
about to be dialed, never the pre-resolution hostname. The old
`dialPrivateOnlyWith` test faked a `net.Conn`'s `RemoteAddr()` to exercise
the post-dial check without real network I/O; the new test calls
`rejectNonPrivateControl` directly with a `nil syscall.RawConn` instead,
since the function never touches it - simpler and no longer needs a fake
connection at all. Revert-verified: reverted the range check to
always-allow, confirmed the public-address test cases fail, restored.

Loopback stays allowed in the production path (the audit's own secondary
recommendation was to restrict it there too, "not only for testability")
- left as-is deliberately: this package's own tests bind to loopback via
`httptest.Server`, and a fully clean test/production split would need
materially more test scaffolding for a client whose blast radius (an
unauthenticated TR-064 host-discovery call) loopback doesn't meaningfully
worsen beyond what the private ranges above it already allow.

**Releasekritisch, fixed: final image tags were published before the
release's own E2E tests ran, and a re-run could silently reuse an image
built from the wrong commit.** Two compounding gaps in
`release-alpha.yml`'s `publish` job:

1. It pushed straight to the FINAL version tag
   (`ghcr.io/.../IMAGE:VERSION`), before `smoke-test`/`upgrade-test` had
   run - so a release whose E2E tests then failed still left a real,
   publicly pullable image behind under that tag.
2. The "does this need building" check keyed on that same final tag's
   mere *existence* - so a re-run after fixing whatever broke the tests
   could not correct it: the tag already "existed", build+attestation
   were silently skipped, and the stale (or, on a re-dispatch against a
   newer commit, actually-wrong-commit) image was reused - while the
   release notes' provenance section still claimed it was built from the
   current `source_ref`. Not theoretical - this happened on a real
   release re-run.

Fixed by publishing to a commit-scoped CANDIDATE tag first
(`VERSION-candidate-<12-char-sha>`, computed once in the `version` job so
every job agrees on it) instead of the final tag - nothing under the
final version tag is ever touched by `publish` itself. `smoke-test` and
`upgrade-test` now run against these candidate images. Only after both
have actually passed does `update-alpha-pins` (already gated on
`needs: [..., smoke-test, upgrade-test]` from an earlier round) run a new
"Promote candidate images to the final release tag" step:
`docker buildx imagetools create` republishes the exact already-tested
manifest under the final tag - not a fresh build, so the promoted image
is byte-for-byte what was actually tested (same digest, same
attestation, same `org.opencontainers.image.revision`) - followed
immediately by re-inspecting the promoted tag's digest and asserting it
matches the recorded one exactly, as defense-in-depth against promoting
from a stale digest file.

This also closes finding #2 as a side effect, not just #1: because the
candidate tag encodes the exact commit, "does this candidate already
exist" and "was this the right commit" collapse into the same question -
a genuinely new commit always gets its own, different candidate tag,
so there's no tag-existence check left that a wrong-commit re-run could
satisfy.

Also wired `gitleaks`/`trivy`/`govulncheck`/`staticcheck` directly into
the release gate (the audit's own additional recommendation) rather than
continuing to trust that `ci-security.yml`'s separate `push: branches:
[main]` trigger happened to run against the right commit at some
earlier, untimed point: `ci-security.yml` gained a `workflow_call`
trigger (with a `ref` input, since a workflow_dispatch release run's
`source_ref` can differ from its ambient `github.sha` - see the earlier
fix for that), and a new `security` job in `release-alpha.yml` calls it
pinned to the release's own `source_ref`, with `publish` now requiring
`needs: [version, test, security]`.

Workflow-only change with no way to exercise a real release run in CI -
verified locally: YAML parses on both files (`yaml.safe_load`), the
`docker buildx imagetools create`/`inspect` command shapes match the
pattern already verified live in an earlier round's base-image pinning
work, and every image-reference expression that needed to move from
`needs.version.outputs.value` to the new `needs.version.outputs.candidate_tag`
was swept via `grep` before and after to confirm none were missed (the
handful of remaining `value` references are all genuinely about the git
tag/GitHub Release name, not a Docker image reference, and were left
alone deliberately).

**Fixed (not deferred - see the correction above): blockpage no longer
holds any AdGuard credential.** Round 1 explicitly deferred this as a
"longer-term redesign"; the user's own correction on this second review
was direct: a mandate to fix a review's findings doesn't leave room for
unilaterally downgrading one to "document only" because it looks bigger
than the others, even when the review itself calls it a long-term
recommendation. Implemented as originally scoped:

`publishBlockpageAuthToken` (`base64(adguard_username+":"+adguard_password)`,
written to the volume shared with the blockpage container) is gone.
`Manager.publishBlockpageServiceToken` now writes a 32-byte random,
AdGuard-unrelated token there instead, and a new `GET
/api/blockpage/reason` endpoint in `rootguard-core/internal/api/routes.go`
performs the actual AdGuard `check_host` call server-side - only Core
ever holds the real AdGuard credentials now, and only returns the block
`reason` string (the only field blockpage's own frontend, `meta.js`,
actually reads). Registered on the root mux (not the bearer-token-gated
`apiMux` subtree) with its own, much narrower auth check
(`Manager.VerifyBlockpageServiceToken`, constant-time-compared) - putting
it behind Core's own admin token (shared with the WebApp and the updater)
would have been almost as bad as the problem being fixed. Host input is
validated against an RFC-1123-style hostname pattern
(`ErrInvalidBlockpageHost`) before ever reaching AdGuard's admin API.

`rootguard-blockpage`'s nginx template and its `19-render-blockpage-conf.sh`
entrypoint script both updated to match: `/api/reason` now proxies to
`http://rootguard-core:8081/api/blockpage/reason?host=$host` with `Authorization:
Bearer ${BLOCKPAGE_SERVICE_TOKEN}`, resolved the same Docker-embedded-DNS
way the old direct-to-AdGuard proxy already was. Reachability confirmed by
reading the actual network topology, not assumed: blockpage only joins the
dynamically-rendered `rootguard-dns` network (see `renderCompose` in
`installer/manager.go`), and Core is already `docker network connect`ed to
that same network at deploy time (for its own DNSSEC-adjacent reasons,
predating this fix) - so `rootguard-core:8081` was already reachable from
there, no new network wiring needed.

Regenerating the token on every `Bootstrap` call (rather than persisting
and reusing one) is intentional, not an oversight: `installer.Manager`
already re-renders blockpage's nginx config and reloads it
(`docker exec rootguard-blockpage sh /docker-entrypoint.d/19-render-blockpage-conf.sh`
+ `nginx -s reload`) immediately after `Bootstrap` returns, in the same
deploy step that existed before this fix - so there's no window where
blockpage would be left holding a stale token.

Tests: `adguard` package - the token is 32 random bytes (64 hex chars),
provably unrelated to the AdGuard credentials, and round-trips through
`VerifyBlockpageServiceToken` (accepts the real one, rejects a tampered
one, rejects empty, fails closed before any `Bootstrap` has ever run);
`ReasonForHost` proxies correctly and rejects malformed hosts. `api`
package - the handler rejects a missing/wrong service token, rejects a
malformed host with a valid token, and - the routing half of the fix,
proven separately - a request bearing the service token (never Core's
admin token) reaches past `requireBearerToken`, revert-verified: removing
the route's own `root.HandleFunc` registration and re-running that exact
test flips it to a 401, confirming the route precedence is what the test
depends on, not something it would pass by accident either way.

**Small, fixed: `atomicfile.WriteFile` wrote to a fixed, predictable temp
name (`path+".tmp"`) via a plain `os.WriteFile`.** Three compounding
gaps in that: not concurrency-safe (two overlapping callers for the same
`path` shared that one temp name, racing each other's writes); `os.WriteFile`
opens-and-truncates an *existing* file rather than refusing one, so a
pre-existing `path+".tmp"` - a stale leftover from an earlier failed run,
or a symlink someone else in the same directory could plant - would be
written through rather than replaced; and since `os.OpenFile` only applies
the requested permission bits when it actually *creates* a file, an
existing leftover at that name silently donated its own (possibly wrong)
mode to every future write, ignoring whatever mode the caller asked for.

Fixed by switching to `os.CreateTemp(dir, ...)` in the target directory:
it always creates a brand-new, uniquely-named file, so there's never an
existing name (symlink or otherwise) to collide with, and this code now
explicitly `Chmod`s it to the requested mode regardless of what any
earlier leftover had. Also added `Sync()` on both the file (before the
rename that makes it visible) and the parent directory (after the rename
- a rename is itself a directory-entry change that needs its own fsync to
survive a crash, which the previous version never did either).

`TestWriteFileIgnoresStaleLegacyTempFile` is the regression test:
pre-plants a stale `path+".tmp"` at world-writable mode `0777`, then
asserts a fresh `WriteFile(path, ..., 0600)` call is completely unaffected
by it - both in the final file's content and in its mode actually being
`0600`, not the leaked `0777`. Revert-verified: reverting to the old
fixed-name implementation makes exactly the mode assertion fail (got
`0777`, wanted `0600`), confirming the test targets the real bug, not
just the file-content half of it.

**Medium, fixed: the DNSSEC-bypass check still had reachable bypasses.**
Round 1's fix for `harden-dnssec-stripped: no` matched a literal `": no"`
suffix against the raw config line - real, but still bypassable with any
of `harden-dnssec-stripped:    no` (extra internal whitespace),
`harden-dnssec-stripped: no # allow it` (a trailing comment - the *key*
extraction already stripped comments, the value check never did), or
`harden-dnssec-stripped:no` (no space after the colon) - all ordinary,
spec-legal Unbound config shapes.

Fixed by parsing the value the same principled way the key was already
parsed: new `directiveValue` (comment-stripped, colon-split, trimmed -
mirrors the existing `directiveKey`) replaces the raw-line suffix match,
compared via `strings.EqualFold` instead of a pre-lowercased literal
match. Four new cases added to the existing
`TestCustomConfigRejectsDNSSECBypasses` (the three real bypasses plus a
non-regression casing case, kept as a companion now that the value
comparison is its own explicit step rather than inherited from a
whole-line lowercase) - revert-verified: reverted to the old suffix
match, confirmed the whitespace case fails, restored.

**Small, fixed: an archive with exactly `MaxFiles` entries was still
rejected.** Round 1 fixed this same off-by-one's *message* (it used to
claim "more than %d entries", not always true at that point) but not the
underlying semantics: the entry-count guard checked `count` - starting at
0, incremented per loop iteration before `archive.Next()` was even called
- against `MaxFiles` *before* reading each entry. Once exactly `MaxFiles`
entries had already been read successfully, the guard fired on the next
iteration without ever looking far enough to confirm a genuine
`MaxFiles+1`th entry actually exists - rejecting an archive precisely at
the limit, not over it.

Fixed by restructuring the loop to count (and check) only after a real
entry has actually been read: `count++` moved to immediately after a
successful `archive.Next()`, with the guard now `count > MaxFiles` -
`MaxFiles` entries read is no longer over the limit, only the
genuine `MaxFiles+1`th read trips it.

`TestExtractAcceptsExactlyTheEntryLimit` builds an archive with exactly
`MaxFiles` entries and confirms the entry-count guard no longer rejects
it (deliberately narrower than "the whole restore succeeds" - a
100000-entry archive's manifest would need matching hashes for every one
of them, unrelated overhead for what this specifically targets; whatever
Extract fails on afterward, here the always-expected missing manifest, is
a separately-covered path). `TestExtractRejectsEntryCountOverTheLimit`
is the counterpart: one entry over the limit is still correctly rejected,
with the message a prior round already corrected staying accurate.

**Small, fixed: the AdGuard UI cookie rewrite silently dropped unknown
`Set-Cookie` attributes.** `rewriteAdGuardSetCookie` parses each cookie
with `http.ParseSetCookie`, rewrites `Path`, and re-serializes with
`Cookie.String()`. Verified live: `Cookie.Unparsed` holds any
attribute-value pair `ParseSetCookie` couldn't map to one of its own
known fields (a vendor-specific or not-yet-standard attribute - a
`FutureAttr=xyz` pair lands there in practice), but `Cookie.String()`
never serializes it back - so the rewrite was silently dropping anything
it didn't specifically recognize, not just leaving it untouched.

Fixed by appending `cookie.Unparsed` back onto the serialized result
after `Cookie.String()` runs - order doesn't matter for Set-Cookie
attributes (RFC 6265), so this round-trips correctly regardless of where
the unknown attribute originally sat.

`TestRewriteAdGuardSetCookieKeepsUnknownAttributes` is the regression
test: a cookie carrying a `FutureAttr=xyz` pair must still carry it after
the rewrite. Revert-verified: removing the `Unparsed` re-append makes the
rewritten cookie silently lose it, exactly reproducing the finding.

**Small, fixed: the WebApp's audit log and session inventory trusted a
freely-settable `X-Forwarded-For` header.** Rate limiting already used
the real TCP peer address (`rateLimitKey`, fixed in an earlier round) -
but `clientAddress`, used for the audit log and session inventory shown
to an operator, still trusted `X-Forwarded-For` unconditionally. Not just
a display quirk: an operator reviewing "who logged in from where" for
incident response would see attacker-controlled garbage instead of the
real peer address, since anyone who can reach this container at all could
set an arbitrary value and have it recorded as their own session's
address. RootGuard's own documented reverse-proxy setups
(`docs/https-reverse-proxy.md`) never ask an operator to forward
`X-Forwarded-For` either - only `X-Forwarded-Proto` - so there's no
legitimate deployment this header could be trustworthy in.

Fixed by making `clientAddress` delegate to `rateLimitKey` directly -
kept as its own named function (rather than every caller switching to
`rateLimitKey`) purely to preserve the distinct display-vs-access-control
intent already documented at each call site.

`TestAuditLogIgnoresSpoofedForwardedForHeader` is the regression test: a
login with a spoofed `X-Forwarded-For` must still record the real peer
address in every resulting audit event. Revert-verified: reverting
`clientAddress` to trust the header again makes the audit log record the
spoofed value instead.

**Small, fixed: `theme.js` still had an `innerHTML` sink.** The blockpage
theme toggle set `btn.innerHTML = icons[mode]`, where `icons` is a
hardcoded, developer-written object of three fixed SVG strings - not
exploitable today, since nothing attacker-influenced ever reaches it, but
the audit's point stands: a sink that happens to be safe today is still a
sink, and removing it is cheap here.

Fixed by building each icon as real DOM nodes
(`document.createElementNS`/`setAttribute`) instead of an HTML string, and
swapping the button's child via `removeChild`/`appendChild` instead of
`innerHTML`. Applied identically to both copies of this file -
`rootguard-blockpage/web/theme.js` (the real blockpage) and
`rootguard-webapp/frontend/public/blockpage-preview/theme.js` (the
WebApp's live preview of it, a manually-kept-in-sync copy, confirmed
byte-identical to the original before and after this change).

No existing test harness for this file (no Playwright/DOM test
infrastructure in this repo yet) - verified instead with a real `jsdom`
instance (already a frontend devDependency): loaded the actual file,
confirmed initialization produces exactly one `<svg>` child, and that
cycling through all three modes (three simulated clicks) toggles
`data-theme` correctly and never leaves more than one child node behind.
`grep` confirms no `.innerHTML =` assignment remains in either file;
`npm run lint` stays clean.
