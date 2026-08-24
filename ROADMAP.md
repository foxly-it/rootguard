# RootGuard roadmap to 1.0

Last reviewed: 2026-08-09

This is the canonical product and engineering roadmap. The public website
summarises it; implementation decisions and release readiness are tracked here.
Items are completed only when their acceptance criteria are verified.

## Status and scope

RootGuard is in **pre-release beta development**. The end-to-end DNS path,
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

- [x] Compact the Unbound resource-profile selector and align the trusted
      private-domain input/action as one control row. Desktop now uses a
      bounded 38 px profile selector and equal 46 px domain controls; mobile
      retains the full accessible action name while reducing the add action to
      a square icon button
      ([rootguard#216](https://github.com/foxly-it/rootguard/issues/216))
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

- [x] Bounded and redacted logs for every managed component - the dedicated
      Logs & Diagnostics page selects every allowlisted service, offers local
      text/severity filters, optional refresh and a downloadable redacted
      report, while Core retains the 30-minute, 100-line, and 64-KiB bounds
      ([rootguard#201](https://github.com/foxly-it/rootguard/pull/201)). Live
      LXC verification additionally separated the five-service log-read
      allowlist from the deliberately narrower AdGuard/Unbound lifecycle-action
      allowlist, so control-plane logs are readable without granting browser
      start/stop/restart authority over those containers
      ([rootguard#210](https://github.com/foxly-it/rootguard/pull/210))
- [x] Real component versions, image digests, uptime, and health reasons
- [x] Persistent, bounded update and rollback history for data and control plane
- [x] Keep the operations UI scannable as capabilities grow - import scope is
      explicit on the Backups page and searchable down to each workflow; Stack
      keeps routine health and actions visible, folds release metadata into
      technical details, and loads Docker cleanup only on demand
      ([rootguard#203](https://github.com/foxly-it/rootguard/pull/203),
      [rootguard#206](https://github.com/foxly-it/rootguard/pull/206))
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
- [x] Encrypted or explicitly protected backup export - the dedicated Backups
      page now creates a portable, passphrase-protected age-v1 archive containing a
      versioned, checksummed manifest, RootGuard configuration state, live
      AdGuard Home config/work data, and Unbound runtime state. Browser
      sessions, update-restore history, temporary exports, and external `.env`
      secrets are excluded. Source paths and container names are fixed
      server-side, symlinks are rejected, plaintext staging uses a private Core
      directory and is removed on every exit, and data-plane updates are locked
      out while the artifact is created
      ([rootguard#193](https://github.com/foxly-it/rootguard/pull/193),
      [rootguard#195](https://github.com/foxly-it/rootguard/pull/195))
- [x] Full restore into a clean RootGuard installation - the Backups page now
      validates an encrypted age-v1 export before mutation, including bounded
      upload/expanded size/file count, exact allowlisted paths and regular
      file types, required recovery files, manifest inventory, sizes, and
      SHA-256 checksums. A second apply request repeats all checks, requires an
      explicit confirmation, and is accepted only when installation state plus
      Docker containers, internal volumes, network, target address, and port
      pass a fresh clean-target preflight. Operators can recheck a replacement
      host with a different bind address/port. RootGuard creates stopped
      containers and empty volumes, restores local plus AdGuard/Unbound data,
      normalizes Unbound ownership, then starts and health-verifies the full
      protected DNS chain. Passphrases are never persisted, plaintext staging
      is removed on every exit, and a failed attempt removes its new Docker
      resources and rolls back prior local volume contents
      ([rootguard#199](https://github.com/foxly-it/rootguard/pull/199))
- [x] Pre-update snapshot and post-update restore verification - distinct
      from the operator-triggered portable backup/restore above: the
      *internal, automatic* pre-update snapshot Core takes before replacing
      an AdGuard/Unbound image now carries a SHA-256 checksum per backed-up
      file in its manifest (`rootguard-core/internal/updater/backups.go`).
      A failed update's automatic rollback verifies every checksum before
      trusting the snapshot; a corrupted or partial backup is refused rather
      than silently restored, while the container still lands back on the
      known-good previous image even when the data restore itself is
      refused. A real Docker integration test
      (`rollback_integration_test.go`, `go test -tags integration
      ./internal/updater/... -run TestRollback`) proves both paths against
      an actual container and a real `docker compose` image swap, under its
      own isolated Compose project so it can never interact with a real
      RootGuard deployment; wired into `ci-core.yml` as a dedicated job
      ([rootguard#223](https://github.com/foxly-it/rootguard/issues/223))
- [x] Power-loss and interrupted-write tests for installation and updates -
      every persisted file already used atomic temp+rename except
      `writeBackupManifest`, now fixed to match. Since Go has no
      per-goroutine kill, genuinely simulating a killed process needs a real
      child OS process SIGKILLed mid-operation (the same "TestHelperProcess"
      technique Go's own `os/exec` tests use): both `updater.Manager`
      (`power_loss_integration_test.go`, real Docker, killed between
      `backup()` and the candidate image swap, and again between the swap
      and Verify/rollback) and `installer.Manager`
      (`power_loss_test.go`, Docker-free - `deploy()` hardcodes production
      container/network names with no isolated-project escape hatch, so a
      real-Docker version would risk colliding with an actual deployment;
      the process-persistence layer under test does not depend on the
      containers being real) prove a restarted Manager reports the existing
      interrupted-operation diagnostic instead of a stale in-progress
      status, that whatever was already captured/written before the kill
      stays intact, and that a retried operation afterward completes
      cleanly - the appliance is never left stuck. Wired into `ci-core.yml`;
      the installer test needs no Docker and already runs in the plain
      `go test ./...` job
      ([rootguard#225](https://github.com/foxly-it/rootguard/issues/225))
- [x] Disaster-recovery runbook tested on a separate host -
      `docs/disaster-recovery.md` covers total host loss, a failed update
      that didn't roll back cleanly, lost administrator credentials, a
      stuck deployment/update after a crash, and DNS-outage triage, each
      mapped to the actual existing mechanism rather than introducing new
      recovery capability. The total-host-loss scenario was drilled for
      real: an encrypted backup exported from a live installation was
      restored on an entirely separate, freshly provisioned host with no
      prior RootGuard state, reaching `installed` through all seven restore
      steps, with the replacement host's own resolver independently
      verified (`ad`-flagged recursive resolution, `SERVFAIL` for a broken
      DNSSEC chain) - proof the restored DNS chain actually works, not only
      that files were copied. The drill also surfaced a real gap: the
      backup/restore feature landed on `main` after the published
      `v0.1.0-alpha.7` release, so the current public alpha doesn't have it
      yet - noted directly in the runbook
      ([rootguard#227](https://github.com/foxly-it/rootguard/issues/227))

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
- [x] Authenticated account settings: `POST /api/auth/account` lets the
      logged-in operator rename their account and/or set a new password
      by confirming the current one, reusing the same PBKDF2/
      `credentials.json` persistence the recovery flow already had (now
      extended to also cover the username). Unlike recovery, the calling
      session stays alive and only every *other* session is invalidated.
      Reachable from the user menu ("Account settings")
      ([rootguard#329](https://github.com/foxly-it/rootguard/issues/329))
- [x] Rate limits and audit events for authentication: login and password
      recovery both lock out after 5 failures in a 5-minute window (an
      already-issued lockout blocks even a subsequently correct password,
      rather than letting enough guesses eventually succeed), and a bounded,
      persisted audit log records login success/failure, rate-limiting,
      logout, recovery, and session revocation - visible in the user menu's
      "Active sessions" panel alongside the session inventory
      ([rootguard#148](https://github.com/foxly-it/rootguard/pull/148))
- [x] Rate limits and audit events for destructive actions elsewhere in the
      app: a shared, per-session sliding-window limiter (30 requests / 5
      minutes across every destructive route, not a separate budget per
      route) and audit logging now cover Unbound settings activation,
      history restore, custom-config apply, import apply, and diagnostic
      logging start/stop; service start/stop/restart and image updates;
      backup settings changes, export, and restore; manual cleanup;
      control-plane update install; installation deploy; and AdGuard
      bootstrap and filtering toggle - the same `GET /api/auth/audit` log
      the authentication surface already used
      ([rootguard#219](https://github.com/foxly-it/rootguard/issues/219))
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
- [x] Keyboard and screen-reader audit of every WebGUI workflow - formal
      re-verification of every current route (the original pass predates
      Login, Backups, and Logs & Diagnostics entirely) plus every
      interactive state added since: automated axe-core across 8 routes x
      2 themes (30 scan combinations, all clean after fixes), scripted
      keyboard walk-throughs of the full login/recovery flow, the
      collapsed-sidebar sub-nav, three independent `ContentModal` focus
      traps (Sessions, AdGuard filter-test, plus the existing search
      modal), and the Zones tab's guided-workflow wizards. Found and fixed
      three real bugs beyond the earlier pass's scope: collapsed-sidebar
      sub-nav links had no accessible name on 3 of 4 Unbound tabs (icon
      only, label hidden via CSS, no `aria-label` fallback - the exact
      pattern the top-level nav items already had); the Overview tab's
      own two sub-nav entries never appeared at all (`sectionsFor()` had
      no case for it); and closing the session-inventory modal silently
      dropped focus to `<body>` instead of the user-menu trigger, the
      same async-unmount race `returnFocusTo` was built for in
      [rootguard#110](https://github.com/foxly-it/rootguard/pull/110) -
      that prop just had no live caller left once Logs became its own
      page ([rootguard#221](https://github.com/foxly-it/rootguard/issues/221))
- [x] WCAG 2.2 AA contrast, focus, labels, and errors review - contrast and
      focus got real fixes in
      [rootguard#110](https://github.com/foxly-it/rootguard/pull/110);
      the labels and errors pass above closed the rest: error text on
      Overview, AdGuard, Stack, Backups, Logs, and the session/audit
      panel was never announced to screen readers (`role="alert"` only
      existed on Login/Setup/Unbound/the 4 guided wizards) - now
      consistent across every page. Two further contrast regressions
      found and fixed: `ContentModal`'s intentionally-always-dark shell
      broke contrast for theme-tokened content once the page itself was
      in light theme (the shell's own literals were already fixed per
      design, but its legacy `--rg-*` token aliases were declared once at
      `:root` and never re-pinned for content rendered inside it); and
      two guided-workflow "active" buttons lost contrast once composited
      over an already-tinted `--info-soft` card
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

- [x] Automated semantic versioning across all component repositories -
      RootGuard now follows Conventional Commits
      (`CONTRIBUTING.md`); a manually-triggered `release-version-bump.yml`
      computes the next `v0.1.0-alpha.N` tag, refuses an empty release, and
      pushes it. A human still decides *when* to cut a release, this only
      removes having to type or remember the next version number. Verified
      live end to end (`v0.1.0-alpha.9`) - version computation, changelog
      generation, and the GitHub Release all worked correctly on the first
      run. That same run also surfaced a real bug now fixed: GitHub Actions
      never auto-triggers another workflow's `on: push: tags` from a push
      made with the built-in `GITHUB_TOKEN` (an anti-recursion safeguard),
      so the pushed tag alone never started `release-alpha.yml` - a
      "phantom" release existed (tag + GitHub Release, no images) until the
      fix explicitly dispatches `release-alpha.yml` with the computed
      version, the same `workflow_dispatch` path already used by hand for
      every prior alpha release
      ([rootguard#233](https://github.com/foxly-it/rootguard/issues/233))
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
- [x] SBOM and provenance for every image -
      `release-alpha.yml`'s shared `docker/build-push-action` step now sets
      `sbom: true` and `provenance: mode=max` for all 5 matrix components
      (previously only `ci-unbound.yml` did, for unbound alone, outside the
      actual release path). Verified against the real `v0.1.0-alpha.8`
      release: every one of the 5 published images carries a fetchable SPDX
      SBOM and SLSA provenance predicate
      (`docker buildx imagetools inspect --format '{{json .SBOM}}'`/
      `'{{json .Provenance}}'`)
      ([rootguard#229](https://github.com/foxly-it/rootguard/issues/229))
- [x] Image signing and signature verification in the release/update path -
      the signing half already existed: `actions/attest@v4` produces a
      GitHub/Sigstore-backed keyless build-provenance attestation
      (Fulcio-signed over GitHub Actions OIDC) for every published image.
      The gap was verification coverage: `attestation.go`'s policy map only
      ever checked `core`/`webapp`, so `updater`/`unbound`/`blockpage`
      silently reported `not_applicable` in the Stack Center despite being
      published by the identical signer. Extended to all 5. Verified
      against the real `v0.1.0-alpha.8` release: `cosign verify-attestation`
      (run from inside a live `rootguard-core` container, the exact binary
      and policy Core itself uses) reports `verified` for all 5 images
      ([rootguard#230](https://github.com/foxly-it/rootguard/issues/230))
- [x] Compatibility matrix for RootGuard, Docker, AdGuard, and Unbound versions -
      implemented (`docs/compatibility-matrix.md` consolidates all four
      axes: three already had real, independent CI proof - Docker
      platform/engine via `docs/platform-support.md`'s clean-install
      matrix, AdGuard channel via `ci-adguard-compat.yml`'s stable/beta
      matrix, Unbound via `ci-unbound.yml`'s native-arch and scenario
      checks - RootGuard pins its own Unbound build, not an operator
      choice. Its fourth axis (the `upgrade-test` job) is now live-verified
      passing against `v0.1.0-alpha.16`, see below
      ([rootguard#243](https://github.com/foxly-it/rootguard/issues/243))
- [x] Upgrade tests from every supported previous RootGuard release -
      scoped to the one release directly before the current one (N-1 -> N):
      RootGuard is pre-1.0 alpha and doesn't yet promise compatibility
      further back. `release-alpha.yml`'s new `upgrade-test` job deploys
      the previous release exactly as it shipped, completes guided setup,
      verifies DNS, then upgrades Core/WebApp in place through the real
      control-plane updater (never a synthetic fixture) to the version
      being published, and verifies the running images and DNS resolution
      afterward. Building it surfaced a real, independent bug now fixed in
      the same change: the release tag is created *before* the pin-update
      commit that follows it, so every existing tag
      (confirmed live: `v0.1.0-alpha.7`, `v0.1.0-alpha.8`) has always
      pointed at a `compose.alpha.yaml` still pinned to the *previous*
      release's images - the documented quick start
      (`curl .../v0.1.0-alpha.N/compose.alpha.yaml`) fetched the wrong
      images for every past release. `update-alpha-pins` now moves the tag
      to the pin-update commit it just created; the `upgrade-test` job
      itself doesn't depend on that fix, since it locates the correct
      pin-update commit directly rather than trusting the tag. Diagnosing
      the job's first live runs took six rounds of fixes to the job itself
      (silently swallowed curl errors, a check/install race, and a
      poll loop that didn't tolerate the WebApp's brief restart mid-swap),
      the last of which surfaced a genuine, independent, previously
      untested product bug: Core's `controlplane.Client` had no `History`
      field, so it silently dropped the updater's update-outcome history
      while proxying its status - fixed with a matching type and a new
      regression test. Live-verified passing end to end against
      `v0.1.0-alpha.16`
      ([rootguard#242](https://github.com/foxly-it/rootguard/issues/242))
- [x] Migration framework for persistent state and configuration schemas -
      scoped deliberately, per explicit user direction, to schema-version +
      fail-closed consistency rather than a full transform-function
      framework: RootGuard doesn't yet have a real breaking-schema-change
      history that would justify one. Every persisted JSON file across
      `rootguard-core`/`rootguard-updater` was audited; two genuine gaps
      were found and closed (the rest already had adequate cheap validity
      checks and low-consequence failure modes, where adding versioning
      would have been ceremony, not a real improvement). The updater's
      per-backup `manifest.json` - which authorizes restoring files into a
      live container during rollback - now carries a `schema_version`,
      matching its sibling `backupexport.Manifest` which already had one.
      Unbound's `settings.json` gets a `schema_version` envelope kept
      deliberately off the `Settings` type itself (also the guided-settings
      HTTP API response shape); `Load` refuses only a *newer* version than
      the build knows, while an absent or older version still flows through
      the existing hand-rolled additive-field migration
      (`jsonFieldExists`) unchanged
      ([rootguard#237](https://github.com/foxly-it/rootguard/issues/237))
- [x] GitHub issue templates for bugs, installations, security, and features -
      `.github/ISSUE_TEMPLATE/bug_report.yml` (its "Betroffene Komponente"
      dropdown includes "Compose / Installation" as a category, covering
      installation issues within the bug template rather than a separate
      one) and `feature_request.yml` cover bugs/installations/features;
      `config.yml` routes security reports to GitHub's private security
      advisories instead of a public template, which is the correct handling
      for vulnerabilities, not a gap
- [x] Public changelog generated from reviewed release entries -
      `cliff.toml` (git-cliff) generates each release's `CHANGELOG.md`
      section and GitHub Release body from Conventional Commit history
      since the last tag, grouped by type and linking `(#123)` references
      to their PR. `CHANGELOG.md` is seeded with the full history across
      all 8 pre-existing alpha releases, scoped to the real
      `v0.1.0-alpha.N` tag pattern rather than a bare `v*` glob - the
      repository also carries a few stray, non-release tags (`v1.0.0`,
      `v0.1.0`, `v0.2.0-service-discovery`) that would otherwise interleave
      into the changelog out of chronological order. Verified live:
      `v0.1.0-alpha.9`'s changelog section and GitHub Release were both
      generated correctly
      ([rootguard#234](https://github.com/foxly-it/rootguard/issues/234))
- [x] Website status and Wiki updated as a required CI/release check -
      scoped, per explicit user direction, to hard verifiable facts rather
      than content review: `scripts/check-site-facts.sh` compares every
      `0.1.0-alpha.N` mention in `site/*.html` against the latest real
      release tag (excluding deliberate historical references like
      "Starting with 0.1.0-alpha.2, ..."), and verifies every local
      `href`/`src` resolves to a real file. Runs on every push/PR to `main`
      via `ci-site-facts.yml`, not path-filtered to `site/**` - the site can
      go stale purely because a release was cut elsewhere with no
      `site/*.html` edit to trigger a path-filtered check. Building this
      surfaced real drift, fixed in the same change: `site/*.html` still
      said `0.1.0-alpha.7` in the status badge, quick-start `curl` commands,
      pinned `.env` image digests, and version labels, even though
      `v0.1.0-alpha.8`/`v0.1.0-alpha.9` had already shipped
      ([rootguard#240](https://github.com/foxly-it/rootguard/issues/240))

Exit: publish `0.1.0-beta.1` for broader self-hosted testing - continuing
the existing `0.1.0-alpha.N` version series, not jumping to `0.6.0`; "0.6"
here is this roadmap's own milestone label, not the software's version
number.

---

## 0.9 — release candidate

Goal: freeze features and prove reliability.

- [ ] No unresolved release-blocking defect - deferred to a final `gh issue
      list` sweep immediately before cutting `1.0.0-rc.1`, not before; doing
      it earlier would just need repeating.
- [ ] Thirty-day continuous DNS test with update and restore exercises -
      **in progress**, started 2026-08-14 on a dedicated host
      (`scripts/soak/*.sh` + systemd timers, see that directory's README).
      Live status as of 2026-08-22 (day 8): 952/960 probes passed (99%);
      the 8 misses were all transient DNSSEC-reject timeouts correlated
      with elevated resolve latency (~12s vs. the usual ~3s), each
      self-recovered by the next 10-minute probe - looks like real
      upstream/root-server jitter, not a RootGuard defect. 4 no-op
      update-exercise cycles (the host's `*_UPDATE_IMAGE` pins were never
      bumped past the beta.1 it was deployed with, so these only proved
      the check/poll/history path stayed healthy, not a real upgrade),
      then one real one after repinning to beta.3 and re-running on
      2026-08-22: `outcome: "success"`, Core and WebApp both verified
      swapped from their beta.1 to beta.3 image digests, DNS/DNSSEC/
      filtering/WebGUI probe clean immediately after.
      2 backup/restore drills: the first (08-14) fully clean; the second
      (08-21) had `restore_ok=true` but its single-shot post-restore DNS
      check lost a narrow timing race right after container recreation
      and logged a false failure, fixed by giving that check a bounded
      retry instead of one attempt
      ([rootguard#301](https://github.com/foxly-it/rootguard/issues/301)).
      Closes with the final `report.sh` rollup around 2026-09-13
      ([rootguard#271](https://github.com/foxly-it/rootguard/issues/271)).
- [x] Fresh install, upgrade, rollback, backup, and restore matrix is green -
      fresh install (`clean-install.yml`), upgrade
      (`release-alpha.yml`'s `upgrade-test`), and rollback
      (`ci-core.yml`/`ci-updater.yml`'s rollback-integration jobs) already
      had real CI coverage against live containers; backup/restore only had
      unit tests against a mocked `docker` command runner until now. Added
      `scripts/verify-backup-restore.sh` + `backup-restore.yml`, mirroring
      `verify-clean-install.sh`'s own conventions, running a real
      export → teardown → fresh install → restore → verify-DNS cycle on
      `amd64`/`arm64` GitHub-hosted runners
      ([rootguard#276](https://github.com/foxly-it/rootguard/pull/276)).
- [x] Performance and memory baselines for small and medium networks -
      `docs/performance-baseline.md`. Small network: passive `docker
      stats` sampling from the endurance test's own probe (100% pass rate
      at write time). Medium network (defined here as 20 sustained QPS,
      since no prior definition existed): a live `dnsperf` run against the
      running instance, 95.9% completion, sub-millisecond average latency,
      no leak signature
      ([rootguard#277](https://github.com/foxly-it/rootguard/pull/277)).
- [x] Final accessibility and security review - `docs/
      accessibility-security-review.md`. `@axe-core/playwright` against a
      real `v0.1.0-beta.1` instance, all 11 routes (including the three
      Unbound sub-sections, specifically re-checked since that's where the
      0.5 audit's real bugs were) x both themes = 22 scans, 0 violations.
      Security: every mutating route cross-referenced against
      `guardDestructive` coverage, a common-anti-pattern sweep (TLS bypass,
      shell-interpolated `exec`, `dangerouslySetInnerHTML`, `eval`,
      hardcoded credentials - all clean), and a threat-model currency
      check. Builds on this cycle's own auth hardening: rate limiting no
      longer trusts `X-Forwarded-For` (bypass + unbounded memory growth),
      password recovery writes are now failure-safe (sessions-then-
      credentials ordering with rollback on either write failing), logout
      surfaces persistence errors like session revocation already did, and
      the static-file guard no longer misinterprets `[ ] * ?` as glob
      patterns
      ([rootguard#274](https://github.com/foxly-it/rootguard/pull/274),
      [#279](https://github.com/foxly-it/rootguard/pull/279)).
- [x] Documentation tested by a user without development context - a
      fresh read-through of the public `docs.html` manual (Requirements
      through Operations) surfaced three real issues: a leftover "Gestoppte
      Alpha wieder starten" from the alpha→beta sweep that grep missed
      because it's inside a `<pre><code>` block rather than a
      `data-de`/`data-en` string, a `dig @ROOTGUARD_LAN_IP` example that
      read as a real shell variable nobody ever told the reader to set, and
      no CPU/RAM guidance in the Requirements checklist. All three fixed
      ([rootguard#281](https://github.com/foxly-it/rootguard/pull/281)).
- [x] Supported platforms, limitations, and support policy frozen -
      `docs/platform-support.md` extended with explicit supported-platform
      statements, minimum requirements grounded in the real
      performance-baseline numbers (2 vCPU/2 GB as the practical floor,
      not a guess), known limitations cross-referencing the docs that
      already cover each in depth, and a pre-1.0 support policy that's
      explicitly deferred to a real support window once 1.0 ships
      ([rootguard#280](https://github.com/foxly-it/rootguard/pull/280)).
- [ ] Versioned 1.0 migration and rollback instructions complete -
      deliberately deferred: the underlying mechanism (the control-plane
      updater's upgrade/rollback path) is already fully built, tested, and
      documented, but 1.0.0 itself still has 10 open checklist items below
      that could still change its exact shape. Writing version-specific
      migration instructions now would risk describing a release that
      doesn't exist yet - stays open until 1.0's scope is actually final.

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
