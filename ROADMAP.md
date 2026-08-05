# RootGuard roadmap to 1.0

Last reviewed: 2026-08-05

This is the canonical product and engineering roadmap. The public website
summarises it; implementation decisions and release readiness are tracked here.
Items are completed only when their acceptance criteria are verified.

## Status and scope

RootGuard is in **pre-release alpha development**. The end-to-end DNS path,
guided setup, authenticated WebGUI, Unbound configuration lifecycle, AdGuard
bootstrap, and guarded update paths exist. Use in a production environment requires
recovery, immutable releases, broader system tests, and operational hardening.

The 1.0 scope is a **single-node Docker appliance**. Bare-metal/systemd and
multi-node management are explicitly post-1.0 so they cannot delay a reliable
Docker release.

Status symbols:

- `[x]` delivered and verified
- `[ ]` required for the milestone
- `Later` useful, but not a gate for that milestone

## Release rules

Every release candidate must satisfy all of these rules:

1. No browser request can choose container names, images, commands, mounts, or
   arbitrary configuration paths.
2. Every mutable DNS configuration path supports preview, validation,
   versioning, activation health checks, and rollback.
3. Every supported upgrade has a tested recovery path.
4. Documentation and the public website are updated in the same change as the
   feature they describe.
5. Images use immutable digests; version, commit, license, SBOM, and supported
   architectures are published.
6. Known limitations are explicit. A feature is never presented as delivered
   before its release gate passes.

---

## 0.1.0-alpha.2 — reproducible public alpha

Goal: an external tester can install a named RootGuard version and report a
reproducible problem without relying on local `dev` images.

### Already delivered

- [x] Control plane split into WebApp, Core, and isolated Updater
- [x] Guided AIO deployment of Unbound and AdGuard Home
- [x] HttpOnly sessions, same-origin write checks, and internal bearer token
- [x] DNSSEC-validating AdGuard → Unbound chain without public fallback
- [x] Unbound preview, `unbound-checkconf`, history, diagnostics, and rollback
- [x] Guided local A/AAAA/CNAME zones and guarded expert configuration
- [x] German and English WebGUI
- [x] Data-plane and paired Core/WebApp update foundations
- [x] AGPL-3.0-or-later licensing and separate trademark notice

### Delivered release gate

- [x] Move Updater into a versioned component repository and pin it from the
      main repository
- [x] Commit clean component revisions and a reproducible root Compose model
- [x] Publish versioned `amd64` and `arm64` images to GHCR
- [x] Replace release `latest` references with readable version tags plus
      recorded immutable multi-architecture manifest digests
- [x] Run clean-install tests on Linux `amd64`, Linux `arm64`, and Docker Desktop
      ([issue #39](https://github.com/foxly-it/rootguard/issues/39))
- [x] Test an actual paired Core/WebApp update and a deliberately failed rollback
- [x] Add typed diagnostics for occupied DNS ports, invalid host addresses,
      missing Compose, failed pulls, and interrupted deployment recovery
- [x] Publish release notes, upgrade notes, known limitations, and checksums
- [x] Run the complete integration workflow from a clean GitHub runner

Published: Git tag and GitHub pre-release `v0.1.0-alpha.2`. The remaining
unchecked items are alpha hardening work and stay ahead of beta readiness.

### 0.1.0-alpha.3 — visibility and interface quality

- [x] Add privacy-preserving live Stack and AdGuard Home dashboard metrics
- [x] Refresh dashboard and Stack Center state automatically
- [x] Move long logs, configurations, and directive details into large,
      scrollable views
- [x] Align guided Unbound actions and fix stale Advisor profile results
- [x] Update the responsive public website and bilingual documentation

Published: Git tag and GitHub pre-release `v0.1.0-alpha.3`. The clean
GitHub-runner release smoke test passed login, guided AIO installation,
recursive DNS resolution, and DNSSEC rejection.

### 0.1.0-alpha.4 — resolver safety and transparent filtering

- [x] Upgrade Unbound to 1.25.2 and preserve writable state across updates with
      a stable non-root volume identity
- [x] Add guided serve-expired, DNSSEC cache, EDNS buffer, resource-profile,
      and temporary diagnostic-log controls
- [x] Apply privacy-oriented AdGuard Home DNS defaults during guided setup
- [x] Show local AdGuard filter diagnostics without visiting test websites or
      exposing client/query details
- [x] Add a collapsible symbol-only navigation sidebar and align public preview
      icons with the live dashboard
- [x] Publish and smoke-test digest-pinned `amd64` and `arm64` component images

Published: Git tag and GitHub pre-release `v0.1.0-alpha.4`. The release smoke
test verifies guided installation, recursive DNS, and DNSSEC rejection.

### Current development slice — trustworthy Stack Center

- [x] Show real runtime state, health, image reference, immutable image ID,
      start time, restart count, and published ports for every managed service
- [x] Translate Docker state into plain-language operator guidance without
      exposing raw daemon output
- [x] Add bounded, redacted service logs with explicit retention
- [x] Persist bounded update, rollback, and cleanup history across restarts
- [x] Pin release images by recorded digest and document retention policy
- [x] Inspect all five allowlisted services and expose OCI version, build
      revision, creation time, source, and manifest-digest pinning
- [x] Distinguish immutable releases, mutable tags, local builds, and incomplete
      release metadata in the bilingual Stack Center
- [x] Verify signed Core/WebApp release attestations before presenting an image
      as cryptographically trusted

### Completed development slice — reproducible Unbound on Debian Stable

Goal: keep the patched Unbound security floor without using Debian
Forky/Sid as the production runtime base.

- [x] Build a pinned Unbound `1.25.2+` release reproducibly from its upstream
      source on Debian 13 Slim instead of installing the Forky/Sid package
- [x] Verify the upstream archive by SHA-256, pin the multi-architecture base by
      digest and direct dependencies by exact version, verify Debian packages
      through signed repository metadata, and publish the resolved graph in the
      SBOM
- [x] Separate the build stage from a minimal Debian 13 runtime stage while
      preserving the stable non-root `100:101` identity and writable RFC5011
      trust anchor
- [x] Publish complete SBOM, source version, build revision, checksums, and
      provenance for `amd64` and `arm64`
- [x] Verify configuration compatibility, recursive DNS, DNSSEC validation,
      trust-anchor writes, and health checks on `amd64` and `arm64`
- [x] Verify Core-managed update and rollback with the source-built image on
      native `amd64` and `arm64` installations
- [x] Document the rebuild and security-update procedure so a newer supported
      Unbound release can be adopted without silently changing the base system

Exit: RootGuard runs a current, traceable Unbound build on Debian Stable and no
production image depends on Forky/Sid packages.

### Next development slice — accessible WebGUI navigation and appearance

Goal: make every RootGuard area and setting quickly reachable while keeping the
application shell consistent, readable, responsive, and fully keyboard
operable.

- [x] Rework the header as a coherent utility bar whose search, documentation,
      GitHub, language, theme, user, and sign-out controls share consistent
      sizing, borders, spacing, icons, hover states, and visible focus states.
      Search itself is added by the global-search items below and will
      inherit the same control style
      ([rootguard-webapp#60](https://github.com/foxly-it/rootguard-webapp/pull/60))
- [x] Move language, appearance, and sign-out into an accessible user menu;
      retain direct Docs and GitHub actions on desktop and consolidate utility
      actions into the menu on narrow viewports
      ([rootguard-webapp#62](https://github.com/foxly-it/rootguard-webapp/pull/62))
- [x] Add a global, local-only search covering pages, Unbound tabs, all guided
      settings, configuration views, diagnostics, history, and important
      actions in both supported languages, including technical directive names.
      Opens and focuses from its visible header control, the `S` key, and
      `Ctrl`/`Cmd` + `K`; ignores shortcuts while typing and supports complete
      keyboard navigation, `Escape`, focus restoration, and reduced motion
      ([rootguard-webapp#64](https://github.com/foxly-it/rootguard-webapp/pull/64))
- [ ] Navigate search results to the correct page and tab, reveal the relevant
      section, and place focus at the destination without changing a setting
      directly from the result list
- [x] Replace hard-coded theme colours with semantic design tokens and offer
      persistent System, Light, and Dark modes, with the system preference as
      the default for users who have not made a selection. Covers the app
      shell, login, Dashboard, Setup, Stack Center, AdGuard, and Unbound
      settings; code/config viewers (expert editor, live-config, logs) stay
      dark by design, like a code block. Box-shadow color and intensity are
      themed the same way (theme-aware `--shadow-ink`/`--shadow-scale`
      tokens) so light mode reads as a crisp lift instead of a dark-tuned
      grey halo
      ([rootguard-webapp#54](https://github.com/foxly-it/rootguard-webapp/pull/54),
      [#56](https://github.com/foxly-it/rootguard-webapp/pull/56),
      [#58](https://github.com/foxly-it/rootguard-webapp/pull/58),
      [#72](https://github.com/foxly-it/rootguard-webapp/pull/72))
- [x] Move the sidebar collapse control to its bottom edge, default new desktop
      sessions to the collapsed icon view, and preserve an existing explicit
      local preference
      ([rootguard-webapp#68](https://github.com/foxly-it/rootguard-webapp/pull/68))
- [x] Keep sidebar navigation and its collapse control visible while long
      settings pages scroll; allow the navigation region itself to scroll when
      viewport height is insufficient and keep mobile navigation fully labelled
      ([rootguard-webapp#70](https://github.com/foxly-it/rootguard-webapp/pull/70))
- [ ] Add sticky in-page navigation for long settings sections where it
      materially improves orientation and connect its destinations to global
      search results
- [ ] Verify responsive behaviour, 200% browser zoom, WCAG 2.2 AA contrast,
      screen-reader names, focus order, tooltips, menus, shortcuts, and both
      colour themes in German and English

Exit: all pages and settings are discoverable without navigating the complete
sidebar manually, and the application shell remains readable and operable with
keyboard, screen reader, zoom, mobile viewport, and light or dark appearance.

---

## 0.2 — useful Unbound administration

Goal: common home, lab, and small-business resolver tasks work through guided
forms. Users should not need the expert editor for normal DNS administration.

The detailed ownership and directive plan lives in
[`docs/unbound-configuration-roadmap.md`](docs/unbound-configuration-roadmap.md).

### Guided configuration

- [x] QNAME minimisation, prefetch, serve-expired, cache TTLs, and threads
- [x] Local A, AAAA, and CNAME records
- [x] Conditional forwarding zones with multiple ordered servers
- [x] Private domains and RFC1918 reverse-zone handling ([#41](https://github.com/foxly-it/rootguard/issues/41))
- [x] Keep Unbound client access fixed to the internal RootGuard network;
      end-client CIDR policy belongs to AdGuard Home by design
- [x] IPv4/IPv6 operating mode with capability and connectivity checks
      ([#43](https://github.com/foxly-it/rootguard/issues/43))
- [x] Cache memory sizing derived from an explicit resource profile
- [x] Serve-expired TTL and client timeout controls
- [x] Prefetch-key and aggressive NSEC controls with compatibility guidance
- [x] EDNS buffer size with safe default `1232` and validation
- [x] Privacy-safe logging level and temporary diagnostic logging

### Fixed secure base

- [ ] Document and test DNSSEC hardening, glue/referral hardening, identity and
      version hiding, unwanted-reply threshold, root hints, and trust-anchor
      maintenance
- [ ] Show fixed base protections in the WebGUI without making unsafe values
      freely editable
- [ ] Validate generated configurations against every supported Unbound image

### User experience and safety

- [x] Fix the Dashboard service KPI so its active and total counts come from
      the same allowlisted five-service inventory; a healthy full stack now
      shows `5 / 5` instead of a stale `5 / 2`
      ([rootguard-webapp#50](https://github.com/foxly-it/rootguard-webapp/pull/50))
- [ ] Shared guided workflow: draft → explanation → preview → validate → activate
- [x] Add a fullscreen mode to the expert configuration editor, and show the
      immutable base configuration's already-active directives inline
      (greyed out, read-only) instead of only in a separate popup, so expert
      users get a complete picture of the effective configuration without
      leaving the editor
      ([rootguard-webapp#74](https://github.com/foxly-it/rootguard-webapp/pull/74))
- [ ] Conflict detection across zones, forwarding, access rules, and expert text
- [ ] Import/export of the complete logical resolver configuration
- [ ] Scenario tests for home network, VLANs, split DNS, IPv6-only local records,
      broken upstreams, and DNSSEC failures

Exit: normal resolver administration no longer depends on raw Unbound syntax.

---

## 0.3 — AdGuard integration without feature duplication

Goal: RootGuard integrates AdGuard Home safely and explains the complete DNS
path without rebuilding functions already maintained by AdGuard Home.

Product boundary:

- AdGuard Home remains the primary interface for filter lists, allow/deny
  exceptions, clients, query logs, statistics, and per-client policy.
- RootGuard owns installation, protected access, the fixed Unbound upstream,
  cross-service health, updates, backup/recovery, and plain-language guidance.
- A native AdGuard function is added to RootGuard only when RootGuard must
  coordinate it with another component or enforce an appliance safety boundary.

### Integration work

- [x] Protected same-origin gateway to the native AdGuard Home interface
- [x] Automatic private bootstrap with generated credentials and fixed Unbound
      upstream
- [ ] Show AdGuard version, configuration state, protected upstream, and
      gateway availability together in RootGuard
- [ ] Cross-service diagnostics showing Client → AdGuard → Unbound → DNSSEC
- [ ] Contextual links from RootGuard guidance to the relevant native AdGuard
      page without exposing its administration port
- [ ] Document backup and restore ownership for AdGuard configuration, work
      data, query history, and filter state
- [ ] Compatibility tests against every supported AdGuard Home release
- [ ] Ship a self-hosted, RootGuard-branded blocked-request landing page as a
      new `rootguard-blockpage` component repository (submodule, same
      pattern as core/webapp/unbound/updater), AGPL-3.0-or-later like the
      rest of RootGuard. A fresh design, not a port of the visuals from the
      existing standalone `foxly-it/adguard-blockpage` project; keep its
      proven technical shape (static HTML/CSS/JS, auto dark/light, no
      external dependencies). Configure AdGuard Home's "custom IP for
      blocked hosts" to it automatically during guided setup. HTTP only - a
      DNS-level blocking IP cannot present a valid TLS certificate for
      arbitrary blocked domains, so HTTPS requests show the browser's own
      certificate-warning interstitial instead of the page; this is AdGuard
      Home's documented `custom_ip` behaviour, not a RootGuard limitation

Exit: AdGuard Home remains recognisably native while RootGuard provides a safe,
coherent appliance lifecycle around it.

---

## 0.4 — operations, backup, and recovery

Goal: an operator can understand failures and recover the appliance without
manual Docker forensics.

- [x] Bounded and redacted logs for every managed component
- [x] Real component versions, image digests, uptime, and health reasons
- [x] Persistent, bounded update and rollback history for data and control plane
- [ ] Configurable backup retention with storage-usage visibility
- [x] Safe automatic post-update cleanup:
      retain the active and previous successful image, prune only older image
      IDs recorded by RootGuard, and never call global Docker prune commands
- [x] Prune only unused transient volumes carrying an explicit RootGuard cleanup
      label; permanently protect configuration, data, session, state, and backup
      volumes
- [x] Record every automatic cleanup in the update history and expose
      a clear no-op result when nothing can be removed safely
- [ ] Add an optional manual cleanup preview with a reclaimed-space estimate
- [ ] Encrypted or explicitly protected backup export
- [ ] Full restore into a clean RootGuard installation
- [ ] Pre-update snapshot and post-update restore verification
- [ ] Power-loss and interrupted-write tests for installation and updates
- [ ] Disaster-recovery runbook tested on a separate host

Exit: backup export/import and failed-update recovery are automated and tested.

---

## 0.5 — security, HTTPS, and accessibility

Goal: the appliance has a documented, reviewable security posture suitable for
a trusted network.

- [x] Enforce a patched Unbound `1.25.2+` security floor before exposing
      serve-expired client-timeout controls
- [ ] Built-in HTTPS or a supported reverse-proxy deployment with secure defaults
- [ ] Secure-cookie enforcement when HTTPS is active
- [ ] Session inventory and session revocation
- [x] Recovery path for lost administrator credentials
- [ ] Rate limits and audit events for authentication and destructive actions
- [ ] Threat model covering Docker socket holders, browser, internal networks,
      update supply chain, backups, and the AdGuard gateway
- [ ] Dependency, container, secret, and static-analysis scans in CI
- [ ] Keyboard and screen-reader audit of every WebGUI workflow
- [ ] WCAG 2.2 AA contrast, focus, labels, errors, and reduced-motion review
- [x] Security policy and private vulnerability-reporting instructions

Later: multiple roles and external identity providers unless real 1.0 demand
requires them.

Exit: documented security review has no unresolved critical or high finding.

---

## 0.6 — beta release engineering

Goal: releases are immutable, traceable, upgradeable, and easy to evaluate.

- [ ] Automated semantic versioning across all component repositories
- [ ] Multi-architecture GHCR manifest lists pinned by digest
- [ ] SBOM and provenance for every image
- [ ] Image signing and signature verification in the release/update path
- [ ] Compatibility matrix for RootGuard, Docker, AdGuard, and Unbound versions
- [ ] Upgrade tests from every supported previous RootGuard release
- [ ] Migration framework for persistent state and configuration schemas
- [ ] GitHub issue templates for bugs, installations, security, and features
- [ ] Public changelog generated from reviewed release entries
- [ ] Website status and Wiki updated as a required CI/release check

Exit: publish `0.6.0-beta.1` for broader self-hosted testing.

---

## 0.9 — release candidate

Goal: freeze features and prove reliability.

- [ ] No unresolved release-blocking defect
- [ ] Thirty-day continuous DNS test with update and restore exercises
- [ ] Fresh install, upgrade, rollback, backup, and restore matrix is green
- [ ] Performance and memory baselines for small and medium networks
- [ ] Final accessibility and security review
- [ ] Documentation tested by a user without development context
- [ ] Supported platforms, limitations, and support policy frozen
- [ ] Versioned 1.0 migration and rollback instructions complete

Exit: publish `1.0.0-rc.1`; only bug fixes and documentation may follow.

---

## 1.0.0 — stable Docker appliance

RootGuard 1.0 ships when:

- [ ] installation, DNS operation, configuration, upgrades, and restore are
      repeatable on every supported platform;
- [ ] no supported change can bypass validation and recovery;
- [ ] immutable signed artifacts and complete source are published;
- [ ] the Wiki matches the shipped UI and configuration model;
- [ ] security, accessibility, backup, and release gates above are complete;
- [ ] a tested rollback path from 1.0 to the previous stable state is documented.

Post-1.0 candidates: bare-metal/systemd provider, multi-node management, high
availability, external identity providers, and plugin APIs.

## How we work with this roadmap

For each development slice:

1. Select one unchecked item and assign a stable issue ID.
2. Write acceptance tests before or with the implementation.
3. Update code, API, WebGUI translations, Wiki, and project state together.
4. Record verification and mark the checkbox only after it passes.
5. Do not start the next release phase while an earlier safety gate remains
   unresolved unless the work is independent and explicitly tracked.
