# RootGuard roadmap to 1.0

Last reviewed: 2026-08-09

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
- [x] Navigate search results to the correct page and tab: Unbound's active
      tab is now URL-addressable (`/unbound/:section`, deep-linkable and
      reload-safe) and every Unbound search entry points at its actual tab
      instead of only the bare page
      ([rootguard#106](https://github.com/foxly-it/rootguard/pull/106))
- [x] Reveal the relevant section within the destination tab and place focus
      there, without changing a setting directly from the result list -
      search entries now carry a `#unbound-section-...` hash alongside
      their tab route; landing scrolls to and focuses that element, and
      auto-expands it if it's a collapsed panel like the version history
      ([rootguard#107](https://github.com/foxly-it/rootguard/pull/107))
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
- [x] Add sticky in-page navigation for long settings sections where it
      materially improves orientation and connect its destinations to global
      search results - a sticky section nav appears under the tab strip for
      Resolver, Local DNS & forwarding, and Advanced (each has 3+ distinct
      sub-sections), with scroll-spy highlighting of the currently visible
      section
      ([rootguard#107](https://github.com/foxly-it/rootguard/pull/107))
- [x] Verify responsive behaviour, 200% browser zoom, WCAG 2.2 AA contrast,
      screen-reader names, focus order, tooltips, menus, shortcuts, and both
      colour themes in German and English - automated axe-core scans across
      every page, both themes, both languages, and several interactive
      states, plus scripted checks for focus order/trapping, keyboard
      shortcuts, tooltip-on-focus, and 200%/400%-zoom reflow. Found and
      fixed four color-contrast design tokens that failed 4.5:1 once
      composited over real card backgrounds (not just checked against flat
      `--surface`), a blanket-opacity dimming that silently broke
      descendant text contrast, eight keyboard-inaccessible scrollable
      `<pre>` blocks, and a dialog (`ContentModal`) that didn't trap focus
      ([rootguard#109](https://github.com/foxly-it/rootguard/pull/109))

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
- [x] Expand Local DNS into a zone-centred host inventory: create a zone once,
      then add, rename, remove, and bulk-edit devices and servers by hostname
      plus IPv4/IPv6 address, with optional forward-confirmed PTR generation.
      The typed backend model landed first: `LocalZone`/`LocalHost` in
      `rootguard-core/internal/unbound/settings.go`, validated (canonical
      zone/hostname syntax, per-zone and total host limits, address-family
      and reserved-address checks, one PTR claim per address across zones)
      and rendered as `local-zone`/`local-data`/`local-data-ptr` directives
      through the existing `Preview`/`Apply`/`History`/`Restore` lifecycle
      unchanged - no new activation path
      ([rootguard#129](https://github.com/foxly-it/rootguard/pull/129), closes
      [#127](https://github.com/foxly-it/rootguard/issues/127)). The
      guided-zones frontend (`UnboundGuidedZones.tsx`) is now migrated onto
      this typed model instead of its old JSON-in-a-comment scheme inside
      the custom expert config - per the decision in
      [#131](https://github.com/foxly-it/rootguard/issues/131), CNAME
      records and per-host TTL are dropped from the guided UI entirely (the
      typed model was never scoped to support them; the rare CNAME/TTL case
      routes to the unrestricted expert editor instead)
      ([rootguard#167](https://github.com/foxly-it/rootguard/pull/167))
- [x] Import local hosts through bounded `in-addr.arpa`/`ip6.arpa` discovery and
      optional router adapters, beginning with the documented FRITZ!Box TR-064
      host list; keep router credentials server-side and never persist them in
      browser storage or generated Unbound configuration. The FRITZ!Box
      adapter landed first: `rootguard-core/internal/routerimport` speaks the
      standard TR-064 `Hosts:1` service (`GetHostNumberOfEntries` +
      a bounded `GetGenericHostEntry` loop, capped at 256 entries), answers
      an HTTP Digest challenge (RFC 7616, MD5/qop=auth) when the router
      requires one and tolerates one that doesn't, and never persists the
      submitted credentials - they're used for exactly one discovery
      request and discarded. Verified live against a real FRITZ!Box 6690
      Cable (firmware 267.08.25): both the unauthenticated and the
      Digest-challenged path. The same import card now also supports
      router-independent PTR discovery over operator-supplied private IPv4
      and unicast IPv6 prefixes. Prefixes and the combined request are capped
      at 256 unique addresses, lookups run with 16 workers inside a 15-second
      budget, and results remain unselected drafts until the existing
      preview/validate/activate lifecycle is completed. Additional vendor
      adapters can reuse the normalized discovery contract but are not
      required for this router-independent path
      ([rootguard#133](https://github.com/foxly-it/rootguard/pull/133),
      closes [#132](https://github.com/foxly-it/rootguard/issues/132);
      router-independent discovery tracked in
      [#184](https://github.com/foxly-it/rootguard/issues/184))
- [x] Make every host import preview-only until the operator selects entries;
      canonicalise names and addresses, detect duplicates and conflicts with
      guided and expert records, and use the existing preview, validation,
      versioning, activation-health, and rollback lifecycle. Shipped as a
      "Router import" card on Unbound's Local DNS & forwarding tab - the
      first UI for the typed `LocalZone`/`LocalHost` model from #129:
      discovered hosts start unselected, hostnames are editable per row
      before import (renaming a device the operator doesn't like the
      discovered name for), and selection merges into a named zone through
      the same preview → validate → activate lifecycle as every other
      guided Unbound setting. Duplicate-hostname detection is scoped to the
      target zone itself; the cross-system gap against
      `UnboundGuidedZones.tsx`'s own records no longer exists now that both
      surfaces write the same typed `local_zones` field (#131, below)
      ([rootguard#137](https://github.com/foxly-it/rootguard/pull/137))

### Fixed secure base

- [x] Document and test DNSSEC hardening, glue/referral hardening, identity and
      version hiding, unwanted-reply threshold, root hints, and trust-anchor
      maintenance - added the previously-missing `unwanted-reply-threshold`
      (upstream's own recommended defensive-reset value), documented every
      fixed-base directive's purpose and test coverage in
      `docs/unbound-configuration-roadmap.md`, and added CI checks for what
      wasn't tested yet: `hide-identity`/`hide-version` now get a real CHAOS
      `id.server`/`version.server` probe (previously only declared, never
      exercised), plus a presence check confirming every hardening directive
      actually ships in the built image. `harden-referral-path` and
      `use-caps-for-id` remain deliberately deferred (documented why), not
      silently dropped
      ([rootguard#122](https://github.com/foxly-it/rootguard/pull/122))
- [x] Show fixed base protections in the WebGUI without making unsafe values
      freely editable - two read-only surfaces, neither editable: the
      Advanced tab's "LIVE · READ ONLY" panel opens the actual running
      `/etc/unbound/unbound.conf` (hardening directives included) in a
      focused, keyboard-accessible view, and the expert editor's permanently
      visible immutable-base panel (already checked below,
      [rootguard-webapp#74](https://github.com/foxly-it/rootguard-webapp/pull/74))
      shows the same directives inline next to guided settings. Both are raw
      config dumps rather than an explained, per-directive protections list -
      that framing would be a follow-up, not what this item asked for
- [x] Validate generated configurations against every supported Unbound image -
      `unbound-checkconf`, the DNSSEC/identity/version smoke tests, and the
      trust-anchor volume-compatibility check now run natively on both
      supported architectures (`amd64`, `arm64`) before the multi-arch image
      is pushed. Previously only `amd64` ever ran them; the `arm64` half of
      `linux/amd64,linux/arm64` shipped on the assumption that a successful
      QEMU-emulated build implied a working image, with no functional
      verification of its own
      ([rootguard#121](https://github.com/foxly-it/rootguard/pull/121))

### User experience and safety

- [x] Fix the Dashboard service KPI so its active and total counts come from
      the same allowlisted five-service inventory; a healthy full stack now
      shows `5 / 5` instead of a stale `5 / 2`
      ([rootguard-webapp#50](https://github.com/foxly-it/rootguard-webapp/pull/50))
- [x] Shared guided workflow: draft → explanation → preview → validate → activate -
      extracted `useUnboundDraftWorkflow` (`rootguard-webapp/frontend/src/
      hooks/`) and a shared `GuidedFlowSteps` component, and migrated guided
      zones, router import, private domains, and forward zones onto both -
      four of the five places this pattern was independently implemented.
      The hook owns the fetch/reload lifecycle, the concurrency-checked
      preview/activate calls, and busy/message/error state, while staying
      generic over what each consumer edits: a pluggable comparator handles
      private domains' and forward zones' whole-object concurrency check
      vs. guided zones' and router import's field-scoped one, and an
      `onPreviewStart` hook point preserves forward zones' parallel
      reachability probe. All four now show the same step indicator that
      previously only guided zones had. The main settings form (the fifth
      place) is deliberately not migrated - it edits the whole settings
      object as its own draft with no separate source/candidate/concurrency
      check or confirm-before-activate step, a genuinely different shape
      that would have forced the abstraction rather than fit it
      ([rootguard#168](https://github.com/foxly-it/rootguard/pull/168))
- [x] Add a fullscreen mode to the expert configuration editor, and show the
      immutable base configuration's already-active directives inline
      (greyed out, read-only) instead of only in a separate popup, so expert
      users get a complete picture of the effective configuration without
      leaving the editor. The base config panel is permanently visible next
      to the editor, not behind a click-to-expand disclosure
      ([rootguard-webapp#74](https://github.com/foxly-it/rootguard-webapp/pull/74),
      [rootguard#101](https://github.com/foxly-it/rootguard/issues/101))
- [x] Conflict detection across every currently guided Unbound surface and
      expert text -
      more already ships than this line credited: `validateGuidedConflicts`
      (`rootguard-core/internal/unbound/custom.go`) cross-checks guided
      forward zones, private domains, and the RFC1918/local-zone inventory
      against expert-text `forward-zone`/`private-domain`/`local-zone`
      blocks on every activation. Cross-zone uniqueness is now enforced for
      both PTR addresses (`TestLocalZonePTRRejectsDuplicateAddressAcrossZones`)
      and hostnames (`TestLocalZoneHostnameRejectsDuplicateAcrossZones`)
      across the whole typed host inventory, not just within one zone -
      checked both client-side (guided zones, router import) and
      server-side (`validateLocalZones`) on every save/activation. The
      legacy `UnboundGuidedZones.tsx` JSON-in-comment scheme this line used
      to worry about no longer exists (migrated to the typed model in
      #167). Access rules deliberately remain expert-only before 1.0, with
      the existing advisor warning and high-risk catalog entry. A guided
      access-rules surface is therefore not part of this pre-1.0 gate; it is
      a possible reference extension for the post-1.0 architecture tracked
      in [#186](https://github.com/foxly-it/rootguard/issues/186), where
      conflict detection becomes an acceptance criterion of that extension
- [x] Import/export of the complete logical resolver configuration - guided
      settings and expert custom config bundled together as a single
      downloadable/uploadable JSON file (`GET /api/unbound/export`,
      `POST /api/unbound/import/preview`, `POST /api/unbound/import`), with
      a new "Import / export configuration" card on the Advanced tab
      (`UnboundConfigTransfer.tsx`). Distinct from the existing per-version
      history, which is for in-place rollback, not for taking a
      configuration to another instance or a backup. The import path
      validates settings and custom config as the *pair* they'll become
      (`Manager.PreviewBundle`/`ApplyBundle`, reusing the existing atomic
      `applyStateLocked`) rather than each against the other's stale
      on-disk value - otherwise an import resolving an existing
      guided/expert conflict by changing both sides at once would be
      spuriously rejected
- [x] Import an existing hand-written `unbound.conf` - delivered at full
      roadmap scope, per explicit user direction (2026-08-10) to build this
      item completely rather than ship a leaner classify-only version.
      `ImportUnboundConf` (`rootguard-core/internal/unbound/import.go`) parses
      an arbitrary unbound.conf into clauses and classifies every directive:
      fixed-base directives are filtered (or rejected as a conflict if the
      imported value differs, e.g. `hide-version: no`), the ~12 flat
      `server:` scalars with an existing guided setting (qname-minimisation,
      prefetch(-key), aggressive-nsec, serve-expired(-ttl/-client-timeout),
      cache-min/max-ttl, num-threads, edns-buffer-size, verbosity) are applied
      to a candidate `Settings`, dangerous clauses (`remote-control`,
      `python`, `dynlib`, `dnstap`) are blocked outright, and everything else
      allowed for manual expert input is offered for expert-config adoption -
      including whole clause blocks that don't map cleanly onto a guided
      structure, which is already exactly what pasting them into the expert
      editor by hand does today. `forward-zone` blocks now DO reverse-map
      onto guided `ForwardZone`s when they're a clean fit (a name, one or
      more `forward-addr`, optionally `forward-first` - anything else, e.g.
      `forward-tls-upstream`, keeps the whole block as expert-adoptable
      instead of silently dropping what doesn't fit). The zone-scoped
      `domain-insecure`/`private-domain` opt-ins are resolved against the
      zones just extracted: matching a zone sets that zone's
      `AllowUnsigned`/`AllowPrivateAddresses`, a `private-domain` matching no
      zone joins the global guided list instead, and a `domain-insecure`
      matching no zone (meaningless outside that context) is offered for
      expert adoption. `local-zone "static"` clauses (plus their contiguous
      `local-data`/`local-data-ptr` lines, matching exactly the shape
      `Settings.Render()` itself produces) now reverse-map onto the typed
      host inventory too when they're a clean fit: every `local-data` line
      is an A/AAAA record shaped `<hostname>.<zone>`, and every
      `local-data-ptr` line's address and target match a host already
      established in the same group. CNAME records (out of scope for the
      guided model, see #131), a mismatched PTR, a non-`static` type, or one
      of RootGuard's own RFC1918 reverse-zone names (which would otherwise
      produce an empty, invalid zone) all fall back to per-line expert
      adoption instead of guessing wrong. RFC1918 reverse-zone policy
      reverse-maps too: a network's reverse-zone lines (one for
      `10.0.0.0/8`, sixteen for `172.16.0.0/12`, one for `192.168.0.0/16`)
      apply as that network's `ReverseZonePolicy` only when *every* expected
      zone name is present exactly once with a consistent, recognized type
      (`static`→nxdomain, `transparent`→transparent) - a partial set,
      duplicate, or unrecognized type keyword leaves those lines unconsumed
      rather than guessing. `do-ip4`/`do-ip6`/`prefer-ip6` reverse-map onto
      `NetworkMode` only when all three appear exactly once in one of the
      three combinations `Settings.Render()` itself produces.
      `rrset-cache-size`/`msg-cache-size` reverse-map onto `ResourceProfile`
      only when both appear exactly once and match one of RootGuard's three
      preset size pairs exactly. Any incomplete or inconsistent set among
      these three falls back to a blocked finding naming the specific
      guided concept it belongs to, not silent expert adoption (which would
      just be rejected downstream anyway) or a guess. New
      `UnboundConfImport.tsx` (Unbound Advanced tab) drives paste-or-upload
      → classify → the existing bundle preview/activate lifecycle from the
      import/export feature above. Hardened against a real hand-written
      config (11 hosts, private-address/access-control lines, a redirect
      zone, a forward-zone, a remote-control block) reported live: fixed a
      PTR-target trailing-dot mismatch that silently misclassified an
      otherwise-clean zone as expert instead of guided, a custom-config
      accumulation bug where re-classifying after an earlier activation
      produced a genuine `unbound-checkconf` syntax error, and a follow-on
      duplicate-zone error on re-import - zones now merge by name, making
      re-import idempotent. The guided local-zone host list also moved from
      an inline chip row to a proper Hostname/IPv4/IPv6/PTR table
      (`UnboundGuidedZones.tsx`), since the same real-world zone made an
      11-row wrapped chip list hard to scan
- [x] Scenario tests for home network, VLANs, split DNS, IPv6-only local records,
      broken upstreams, and DNSSEC failures - all 6 now covered as named,
      end-to-end tests (`rootguard-core/internal/unbound/
      scenario_integration_test.go`, build-tag-gated so `go test ./...`
      stays Docker-free) that construct real `Settings` values exactly the
      way a user's guided WebGUI configuration would, render them through
      the actual production `Settings.Render()`, apply them to a real
      running `rootguard-unbound` container, and verify with real `dig`
      queries - not string-matching rendered config. Wired into CI as a new
      `scenario-tests` job in `ci-unbound.yml`, gating image publishing
      alongside the existing fixed-base checks. Two design corrections
      found only by testing against the real resolver rather than reasoning
      about it: an `AllowUnsigned`/split-DNS check that initially relied on
      a real third-party domain's DNSSEC status turned out to test the
      wrong thing (`dnssec-failed.org` is deliberately signed-with-broken-
      signatures, not unsigned, so `domain-insecure` never applied to it
      regardless) and was redesigned around forwarding behavior instead;
      and the broken-upstream scenario originally expected a fast SERVFAIL,
      but a forward target that's silently dropped rather than actively
      refused turns out to have no effective timeout in Unbound's default
      retry behavior at all (still pending after 5 minutes in manual
      testing) - redesigned to verify what actually matters operationally,
      that one stuck forward zone doesn't block unrelated queries, instead
      ([rootguard#183](https://github.com/foxly-it/rootguard/pull/183))

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
- [x] Show AdGuard version, configuration state, protected upstream, and
      gateway availability together in RootGuard - configuration state,
      protected upstream, and gateway reachability were already shown in
      the AdGuard status panel; AdGuard's own reported version (previously
      discarded from `/control/status`) is now surfaced there too
- [x] Cross-service diagnostics showing Client → AdGuard → Unbound → DNSSEC -
      verified the container network topology first: AdGuard was reachable by
      container name (`rootguard-adguard`) on the `rootguard-dns` network,
      and only `rootguard-unbound`'s container has both DNS tooling (`dig`)
      and network line-of-sight to it, so the new "Path diagnostics" check
      reuses that same container - same pattern as Unbound's own
      resolution/DNSSEC checks, just targeting AdGuard's listener
      (`rootguard-adguard:53`) instead of Unbound's own port. Surfaced as a
      second card on the Unbound Overview tab, next to the existing "Live
      diagnostics"
      ([rootguard#160](https://github.com/foxly-it/rootguard/pull/160)).
      AdGuard being unpinned on that network at the time turned out to be a
      real bug, not just an implementation detail - fixed in the 0.4 entry
      below
- [x] Contextual links from RootGuard guidance to the relevant native AdGuard
      page without exposing its administration port: deep-links through the
      existing protected `/adguard-ui/` proxy (never the raw admin port) to
      general settings, DNS settings, and the blocklist page, placed next
      to the status rows and filter-test results they're each relevant to
      ([rootguard#150](https://github.com/foxly-it/rootguard/pull/150))
- [x] Master filtering on/off toggle surfaced directly in RootGuard, next to
      the Live-Filterprüfung diagnostic that exercises it - a deliberate,
      explicit exception to the "AdGuard remains the primary interface"
      boundary above, made so operators don't need a second admin surface
      for the single most common toggle; every other filtering concern
      (list management, allow/deny rules, per-client policy) still routes
      through native AdGuard Home via the contextual links above
      ([rootguard#155](https://github.com/foxly-it/rootguard/pull/155))
- [x] Document backup and restore ownership for AdGuard configuration, work
      data, query history, and filter state (`docs/architecture.md`): the
      two dedicated volumes (`rootguard-adguard-config`,
      `rootguard-adguard-work`) are already covered by the existing
      pre-update backup/rollback mechanism, but that's an internal
      update-time safety net, not an operator-triggered or long-term
      retained restore point - documents the current split of
      responsibility and a manual `docker run`-based volume backup for
      anything beyond it, pending the native backup/export work in 0.4
      ([rootguard#151](https://github.com/foxly-it/rootguard/pull/151))
- [x] Compatibility tests against every supported AdGuard Home release -
      RootGuard only ever ships two channels (the pinned stable image and
      the beta channel, per the guided setup's `adguard_channel` option), so
      "every supported release" means both of those, not an unbounded
      version matrix. A new `ci-adguard-compat.yml` workflow runs the full
      bootstrap → status → filtering-toggle → DNS-resolution → filter-check
      surface against each, on a matrix so a channel-specific break doesn't
      mask the other, on pushes/PRs touching the AdGuard integration code
      plus a weekly schedule to catch upstream drift independent of
      RootGuard's own commits
      ([rootguard#158](https://github.com/foxly-it/rootguard/pull/158))
- [x] Ship a self-hosted, RootGuard-branded blocked-request landing page as a
      new `rootguard-blockpage` monorepo directory (same pattern as
      core/webapp/unbound/updater: own Dockerfile, own path-filtered CI),
      AGPL-3.0-or-later like the rest of RootGuard. Static HTML/CSS/JS, auto
      dark/light, no external dependencies. Configures AdGuard Home's
      "custom IP for blocked hosts" automatically during guided setup,
      optional and enabled by default (a dedicated Setup toggle lets users
      who explicitly don't want it disable it; recommended by default). HTTP
      only - a DNS-level blocking IP cannot present a valid TLS certificate
      for arbitrary blocked domains, so HTTPS requests show the browser's own
      certificate-warning interstitial instead of the page; this is AdGuard
      Home's documented `custom_ip` behaviour, not a RootGuard limitation
      ([rootguard#98](https://github.com/foxly-it/rootguard/pull/98)).
      Redesigned after initial feedback that the first version leaned too
      much on the WebGUI's own dashboard visual language (hero/KPI grid, red
      "danger" framing); the shipped design instead follows AdGuard Home's
      own brand green as a positive "protection working" signal, with a
      restrained, high-contrast, single-accent layout
      ([rootguard#100](https://github.com/foxly-it/rootguard/pull/100)).
      Setup's blockpage toggle now links to a bundled static preview so an
      operator can see the actual page before (or without) deploying it;
      building the preview surfaced a real WCAG AA contrast failure in the
      shipped page itself (light-theme `--accent` too light for both
      white-on-green buttons and green-on-white links), fixed in the same
      change - the earlier blockpage work and the separate WebGUI-wide
      accessibility audit had each verified their own surface but neither
      had scanned this component
      ([rootguard#112](https://github.com/foxly-it/rootguard/pull/112))
- [x] The "why blocked" reasons on the landing page were a static,
      always-all-checked list - not tied to what actually matched for this
      request. Three-part fix: (1) a narrow, rate-limited nginx proxy on the
      blockpage itself (`/api/reason`) that asks AdGuard's own `check_host`
      for the real verdict on the specific host that got sinkholed here,
      authenticated with a derived token Core publishes - never the raw
      AdGuard admin credentials - to a dedicated volume, since the blockpage
      is the one component here reachable, unauthenticated, by anyone who
      gets DNS-sinkholed to it
      ([rootguard#117](https://github.com/foxly-it/rootguard/pull/117));
      (2) wire the real per-request reason into five honest category cards
      (the previous four conflated two AdGuard reasons that `check_host`
      can't actually distinguish, and had no card at all for AdGuard's
      "blocked service"/parental-control verdicts)
      ([rootguard#118](https://github.com/foxly-it/rootguard/pull/118));
      (3) a visual redesign bringing back cards and motion in the spirit of
      the main WebApp, deliberately not the #98 hero/KPI treatment #100
      already rejected - same restrained AdGuard-green accent, borrowed card/
      shadow/animation *mechanics* rather than the WebApp's teal/danger
      *palette*
      ([rootguard#119](https://github.com/foxly-it/rootguard/pull/119))
- [x] Follow-up redesign after live user feedback on #119: the five-card grid
      still made someone read past four irrelevant cards to find the one that
      actually applied. Replaced with a narrative layout - "Diese Seite ist
      blockiert - {reason}." as the headline, the domain and reason spelled
      out in one sentence - alongside a commissioned RootGuard fox mascot
      illustration (green cloak/shield matching the brand accent, root-vein
      motif tying "fox" to "root network"), closer in spirit to how AdGuard
      Home's own block page reads. The two-tier fallback (`FilteredBlackList`
      alone can't distinguish ads from generic threat-list hits) collapses
      into the sentence's `Filterliste` phrasing rather than a card label;
      unmatched/failed lookups keep the original generic headline, still the
      default render
      ([rootguard#125](https://github.com/foxly-it/rootguard/pull/125))

Exit: AdGuard Home remains recognisably native while RootGuard provides a safe,
coherent appliance lifecycle around it.

---

## 0.4 — operations, backup, and recovery

Goal: an operator can understand failures and recover the appliance without
manual Docker forensics.

- [x] Bounded and redacted logs for every managed component
- [x] Real component versions, image digests, uptime, and health reasons
- [x] Persistent, bounded update and rollback history for data and control plane
- [x] Fixed a real production failure mode where an Unbound image update -
      and its automatic rollback - could both fail with "Address already in
      use", leaving Unbound down: AdGuard was deliberately unpinned on the
      `rootguard-dns` network (see the 0.3 cross-service diagnostics entry
      below), which let it grab Unbound's reserved static address whenever
      that address happened to be free (e.g. mid-update, or if AdGuard's own
      container was recreated while Unbound was down for any reason) -
      permanently blocking Unbound from reclaiming its own address until
      manually repaired. The controller's own `docker network connect` to
      that network had the identical gap. Both AdGuard and the controller
      now get their own pinned, non-conflicting addresses, matching Unbound
      and the blockpage
      ([rootguard#182](https://github.com/foxly-it/rootguard/pull/182))
- [x] Configurable backup retention with storage-usage visibility - the Core
      updater now retains a configurable 2–50 recognized pre-update restore
      points per service (default 5), persists the policy separately from
      update state, and enforces it only after an update/rollback lifecycle or
      an explicitly confirmed settings change. The Stack Center shows total
      and per-service counts, logical bytes, and newest timestamps. Pruning
      requires the canonical timestamp/service layout plus a manifest matching
      the allowlisted service and container; unknown data and symlinks are
      measured separately and never deleted
      ([rootguard#189](https://github.com/foxly-it/rootguard/pull/189))
- [x] Safe automatic post-update cleanup:
      retain the active and previous successful image, prune only older image
      IDs recorded by RootGuard, and never call global Docker prune commands
- [x] Prune only unused transient volumes carrying an explicit RootGuard cleanup
      label; permanently protect configuration, data, session, state, and backup
      volumes
- [x] Record every automatic cleanup in the update history and expose
      a clear no-op result when nothing can be removed safely
- [x] Add an optional manual cleanup preview with a reclaimed-space estimate -
      the Stack Center lists only obsolete image IDs from RootGuard's own
      successful update history and unused volumes carrying the explicit
      cleanup label, shows Docker's per-resource `UniqueSize`/volume-size
      estimate, and requires confirmation before deletion. Execution always
      recomputes eligibility, retains two successful images per service, and
      records the result in bounded history; unverifiable resources remain
      protected and global prune commands remain prohibited
      ([rootguard#191](https://github.com/foxly-it/rootguard/pull/191))
- [x] Encrypted or explicitly protected backup export - the Stack Center now
      creates a portable, passphrase-protected age-v1 archive containing a
      versioned, checksummed manifest, RootGuard configuration state, live
      AdGuard Home config/work data, and Unbound runtime state. Browser
      sessions, update-restore history, temporary exports, and external `.env`
      secrets are excluded. Source paths and container names are fixed
      server-side, symlinks are rejected, plaintext staging uses a private Core
      directory and is removed on every exit, and data-plane updates are locked
      out while the artifact is created
      ([rootguard#193](https://github.com/foxly-it/rootguard/pull/193))
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
- [x] Built-in HTTPS or a supported reverse-proxy deployment with secure
      defaults - scoped deliberately to documentation, not a RootGuard-native
      TLS implementation: `docs/https-reverse-proxy.md` covers the two hard
      requirements (Host-header passthrough for the same-origin write check,
      `X-Forwarded-Proto` for secure-cookie detection) plus working examples
      for Caddy, Zoraxy, Nginx Proxy Manager, and HAProxy
- [x] Secure-cookie enforcement when HTTPS is active - already implemented
      (`requestIsHTTPS` in `rootguard-webapp/backend/internal/httpapi/auth.go`
      sets the session cookie's `Secure` flag whenever `r.TLS` is set or
      `X-Forwarded-Proto` reads `https`); documented in
      `docs/https-reverse-proxy.md`
- [x] Session inventory and session revocation: `GET /api/auth/sessions` lists
      every active session with a friendly device label, sign-in/expiry
      time, and remote address, without ever returning the actual session
      token; `DELETE /api/auth/sessions/{id}` revokes one by its separate
      opaque id. Reachable from the user menu ("Active sessions"). Sessions
      persisted before this feature (no id/created-at) are backfilled on
      the next load instead of staying permanently unrevocable
- [x] Recovery path for lost administrator credentials
- [x] Rate limits and audit events for authentication: login and password
      recovery both lock out after 5 failures in a 5-minute window (an
      already-issued lockout blocks even a subsequently correct password,
      rather than letting enough guesses eventually succeed), and a bounded,
      persisted audit log records login success/failure, rate-limiting,
      logout, recovery, and session revocation - visible in the user menu's
      "Active sessions" panel alongside the session inventory
      ([rootguard#148](https://github.com/foxly-it/rootguard/pull/148))
- [ ] Rate limits and audit events for destructive actions elsewhere in the
      app (Unbound activation, service updates/rollbacks, AdGuard bootstrap,
      and similar) - only the authentication surface above is covered so far
- [x] Threat model covering Docker socket holders, browser, internal networks,
      update supply chain, backups, and the AdGuard gateway
      (`docs/threat-model.md`,
      [rootguard#144](https://github.com/foxly-it/rootguard/pull/144))
- [x] Dependency, container, secret, and static-analysis scans in CI:
      `govulncheck` and `staticcheck` per Go module, `gitleaks` against full
      git history, and `trivy` filesystem scanning (dependency
      vulnerabilities, secrets, and Dockerfile misconfigurations) across the
      whole repository, all gated on pull requests and pushes to `main`
      ([rootguard#143](https://github.com/foxly-it/rootguard/pull/143))
- [ ] Keyboard and screen-reader audit of every WebGUI workflow - partial
      groundwork already done as part of the 0.1 navigation slice's exit
      gate ([rootguard#109](https://github.com/foxly-it/rootguard/pull/109)),
      but that pass covered pages and common interaction patterns rather
      than every workflow branch (error states, full Setup wizard flow,
      etc.); treat this as a formal release-gate re-verification, not a
      from-scratch audit
- [ ] WCAG 2.2 AA contrast, focus, labels, and errors review - contrast and
      focus got real fixes in
      [rootguard#109](https://github.com/foxly-it/rootguard/pull/109);
      labels and errors still need a systematic pass
- [x] Reduced-motion review: verified live with `prefers-reduced-motion:
      reduce` emulated - `styles/motion.css`'s universal `*`/`!important`
      reset neutralizes every CSS animation and transition in the app (no
      competing `!important` animation/transition rule exists anywhere
      else in the codebase, so it cannot be overridden regardless of
      cascade order), and the one JS-driven smooth-scroll call
      (`Unbound.tsx`'s `jumpToSection`) already branches on the same media
      query
- [x] Security policy and private vulnerability-reporting instructions

Later: multiple roles and external identity providers unless real 1.0 demand
requires them.

Exit: documented security review has no unresolved critical or high finding.

---

## 0.6 — beta release engineering

Goal: releases are immutable, traceable, upgradeable, and easy to evaluate.

- [ ] Automated semantic versioning across all component repositories
- [x] Multi-architecture GHCR manifest lists pinned by digest -
      `release-alpha.yml` publishes proper `linux/amd64,linux/arm64`
      manifest lists for all 5 components, and `compose.alpha.yaml`
      references every one of them, including third-party AdGuard, by
      digest. The `update-alpha-pins` job (added in #165) now updates those
      pins automatically after every release instead of by hand - proven in
      production for the first time with the `v0.1.0-alpha.7` release: all
      four RootGuard image pins landed correctly in a `[skip ci]` commit
      pushed straight to `main`, verified against the actual published
      digests before the tag was cut
- [ ] SBOM and provenance for every image
- [ ] Image signing and signature verification in the release/update path
- [ ] Compatibility matrix for RootGuard, Docker, AdGuard, and Unbound versions
- [ ] Upgrade tests from every supported previous RootGuard release
- [ ] Migration framework for persistent state and configuration schemas
- [x] GitHub issue templates for bugs, installations, security, and features -
      `.github/ISSUE_TEMPLATE/bug_report.yml` (its "Betroffene Komponente"
      dropdown includes "Compose / Installation" as a category, covering
      installation issues within the bug template rather than a separate
      one) and `feature_request.yml` cover bugs/installations/features;
      `config.yml` routes security reports to GitHub's private security
      advisories instead of a public template, which is the correct handling
      for vulnerabilities, not a gap
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
availability, and external identity providers.

---

## Post-1.0 / Future — extensions

Goal: let advanced operators add narrowly scoped integrations and guided
features without weakening RootGuard's validation, recovery, or appliance
security model. This work is explicitly deferred until after 1.0 and carries no
current release commitment ([#186](https://github.com/foxly-it/rootguard/issues/186)).

- [ ] Define a versioned extension API, compatibility contract, capability
      model, and explicit permission boundaries.
- [ ] Provide constrained integration, configuration, and UI extension points;
      RootGuard must retain ownership of preview, validation, activation,
      history, rollback, audit, backup, and restore.
- [ ] Define signing or explicit trust handling plus safe install, disable,
      upgrade, failure-isolation, and removal semantics. Unrestricted Docker
      socket access or arbitrary root scripts are not the default model.
- [ ] Build an official reference extension after the platform contract is
      stable. A guided access-rules extension is a candidate: professionals can
      already use the expert editor today, while a future guided surface must
      conflict-check zones, forwarding, and expert configuration before it can
      activate.

## How we work with this roadmap

For each development slice:

1. Select one unchecked item and assign a stable issue ID.
2. Write acceptance tests before or with the implementation.
3. Update code, API, WebGUI translations, Wiki, and project state together.
4. Record verification and mark the checkbox only after it passes.
5. Do not start the next release phase while an earlier safety gate remains
   unresolved unless the work is independent and explicitly tracked.
