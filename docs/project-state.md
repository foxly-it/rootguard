# RootGuard project state

Last updated: 2026-08-09

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
- Service diagnostics are loaded only after an explicit user action and are
  limited to 100 lines from the previous 30 minutes and 64 KiB. Core removes
  control characters and redacts common authorization, token, password,
  secret, and API-key patterns. The WebGUI explains the window and reminds
  operators to review diagnostic text before sharing it.
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

## Current development slice

Trustworthy Stack Center and production visibility:

- real service state, health, image reference, immutable image ID, start time,
  restart count, and published ports, presented with plain-language guidance;
- completed data-plane updates and paired Core/WebApp updates through the
  separate helper; the public alpha Compose now records and pins every
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
  allowlisted image references; the public alpha Beta reference is pinned to
  its verified multi-architecture manifest.
- guarded public-alpha clean-install verifier shared by Docker Desktop and
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
and volume cleanup. The Stack Center exposes recognized count and logical bytes
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
data-plane updates cannot overlap export creation. Clean-install restore and
transactional snapshot verification remain the next separate 0.4 steps
([rootguard#193](https://github.com/foxly-it/rootguard/pull/193)).

Backup retention and portable export have their own authenticated Backups page
and sidebar entry; Stack & Updates remains focused on service lifecycle,
updates, rollback status, and safe Docker cleanup
([rootguard#195](https://github.com/foxly-it/rootguard/pull/195)).

## Remaining production milestones

Cross-referenced against `ROADMAP.md` on 2026-08-09 (supersedes the previous
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
- **0.4 operations/backup/recovery** - 4 of 14 items open: full restore into a
  clean install, pre-update snapshot/restore verification,
  power-loss/interrupted-write tests, and a disaster-recovery runbook.
  Logging/versioning/history, the automatic cleanup-safety work, and a
  network-addressing fix ([rootguard#182](https://github.com/foxly-it/rootguard/pull/182))
  have landed - AdGuard had no pinned address on the internal `rootguard-dns`
  network (an intentional choice at the time, see the 0.3 note above), which
  let it grab Unbound's reserved static address whenever that happened to be
  free; an Unbound image update could then fail with "Address already in
  use" *and* have its automatic rollback fail with the exact same error,
  leaving Unbound down until manually repaired. The controller's own
  `docker network connect` to that network had the identical gap. Both now
  get pinned, non-conflicting addresses.
- **0.5 security/HTTPS/accessibility** - 3 items open, all close to done:
  destructive-action rate limits/audit beyond the authentication surface,
  a full keyboard/screen-reader workflow re-verification, and a WCAG
  labels/errors review.
- **0.6 beta release engineering** - 8 of 10 items open (issue templates
  landed as a stale-checkbox fix; the digest-pin automation from #165 is
  now also checked off, proven in production by the `v0.1.0-alpha.7`
  release on 2026-08-10). This is the actual gate to cut `0.6.0-beta.1`.

**Working order: top-to-bottom through the roadmap document**, per explicit
user direction (2026-08-09) - not "closest-to-done first" as this section
previously recommended. 0.1 and 0.3 are fully closed, so the order is:

1. **0.4 operations/backup/recovery** - current, in document order: full
   restore, pre-update snapshot verification,
   power-loss tests, the DR runbook.
2. **0.5 security/HTTPS/accessibility** - 3 items open, in document order:
   destructive-action rate limits/audit, the full keyboard/screen-reader
   workflow audit, WCAG labels/errors review.
3. **0.6 release engineering** - 8 items open, in document order: automated
   semantic versioning, signed multi-arch manifest digests (property already
   holds, automating the update after each release is the open part), SBOM/
   provenance, image signing/verification, compatibility matrix, upgrade
   tests, migration framework, changelog generation, website/Wiki CI check.

**Immediate next item when resuming:** 0.2's conflict-detection checkbox
stays unchecked, but the only concrete gap left under it is a guided surface
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

`v0.1.0-alpha.6` was published on 2026-08-06 as a GitHub pre-release with
public, digest-pinned `amd64`/`arm64` images for all four RootGuard components.
Release workflow run `31081554868` built and attested all four components
directly from the monorepo (the first release since the migration) before
installing the released stack and verifying recursive DNS plus DNSSEC
rejection. RootGuard remains in active alpha development:
update safety, backup/restore, broader authentication hardening and roles, and
bare-metal support are not yet production complete.
