# Changelog

All notable changes to RootGuard are documented here, generated from the
commit history at release time. See [ROADMAP.md](ROADMAP.md) for what's
still ahead.

## [0.1.0-alpha.8] - 2026-08-13

### Added

- Add safe backup retention controls ([#189](https://github.com/foxly-it/rootguard/pull/189))
- Add safe manual cleanup preview ([#191](https://github.com/foxly-it/rootguard/pull/191))
- Centralize service logs ([#201](https://github.com/foxly-it/rootguard/pull/201))
- Surface import workflows ([#203](https://github.com/foxly-it/rootguard/pull/203))

### CI

- Pin compose.alpha.yaml to 0.1.0-alpha.7 image digests [skip ci]

### Changed

- Move Unbound's in-page section nav into the main sidebar ([#180](https://github.com/foxly-it/rootguard/pull/180))
- Clarify stack operations ([#206](https://github.com/foxly-it/rootguard/pull/206))

### Documentation

- Close the digest-pin automation item, sync site to alpha.7
- Defer extension architecture until post-1.0 ([#187](https://github.com/foxly-it/rootguard/pull/187))
- Synchronize backups navigation ([#197](https://github.com/foxly-it/rootguard/pull/197))
- Align operations UI documentation ([#208](https://github.com/foxly-it/rootguard/pull/208))
- Record log allowlist separation ([#211](https://github.com/foxly-it/rootguard/pull/211))

### Fixed

- Clamp section-nav rail to never overlap expanded sidebar ([#179](https://github.com/foxly-it/rootguard/pull/179))
- Stop truncating sidebar sub-item labels, show them collapsed ([#181](https://github.com/foxly-it/rootguard/pull/181))
- Pin AdGuard and controller addresses on the DNS network ([#182](https://github.com/foxly-it/rootguard/pull/182))
- Allow managed control-plane logs ([#210](https://github.com/foxly-it/rootguard/pull/210))
- Style backup archive picker ([#213](https://github.com/foxly-it/rootguard/pull/213))
- Compact AdGuard contextual actions ([#215](https://github.com/foxly-it/rootguard/pull/215))
- Compact Unbound form controls ([#217](https://github.com/foxly-it/rootguard/pull/217))

### Other

- Sync the compose.alpha.yaml header comment version, automate it going forward
- Add bounded reverse-DNS host discovery ([#185](https://github.com/foxly-it/rootguard/pull/185))
- Add passphrase-encrypted full backup export ([#193](https://github.com/foxly-it/rootguard/pull/193))
- Add dedicated backups page ([#195](https://github.com/foxly-it/rootguard/pull/195))
- Add guided clean-install full restore ([#199](https://github.com/foxly-it/rootguard/pull/199))
- Polish Unbound cache card spacing ([#218](https://github.com/foxly-it/rootguard/pull/218))
- Rate limits and audit events for destructive actions ([#220](https://github.com/foxly-it/rootguard/pull/220))
- Formal keyboard/screen-reader re-verification of every WebGUI workflow ([#222](https://github.com/foxly-it/rootguard/pull/222))
- Verify pre-update snapshot integrity before an automatic rollback ([#224](https://github.com/foxly-it/rootguard/pull/224))
- Prove recovery from a real process kill mid-install and mid-update (#225) ([#226](https://github.com/foxly-it/rootguard/pull/226))
- Add a disaster-recovery runbook, drilled on a real separate host (#227) ([#228](https://github.com/foxly-it/rootguard/pull/228))
- Attach SBOM/provenance to every release image, verify all 5 signers (#229, #230) ([#231](https://github.com/foxly-it/rootguard/pull/231))

### Testing

- Add end-to-end scenario tests for guided Unbound settings ([#183](https://github.com/foxly-it/rootguard/pull/183))

## [0.1.0-alpha.7] - 2026-08-10

### Added

- Sticky in-page section nav, reveal-and-focus for search ([#108](https://github.com/foxly-it/rootguard/pull/108))

### CI

- Run functional checks on native arm64, not just amd64 ([#121](https://github.com/foxly-it/rootguard/pull/121))
- Add dependency, container, secret, and static-analysis scans ([#143](https://github.com/foxly-it/rootguard/pull/143))
- Add AdGuard compatibility tests against both supported channels ([#158](https://github.com/foxly-it/rootguard/pull/158))
- Complete OCI image metadata labels across all 5 components ([#162](https://github.com/foxly-it/rootguard/pull/162))
- Automate compose.alpha.yaml digest pins after each alpha release ([#165](https://github.com/foxly-it/rootguard/pull/165))

### Documentation

- Document blockpage, fix stale monorepo component links ([#104](https://github.com/foxly-it/rootguard/pull/104))
- Point blockpage redesign roadmap link at the merged PR ([#119](https://github.com/foxly-it/rootguard/pull/119))
- Refresh zone-centred host inventory + FRITZ!Box import plan ([#113](https://github.com/foxly-it/rootguard/pull/113))
- Add threat model covering the six 0.5 trust boundaries ([#144](https://github.com/foxly-it/rootguard/pull/144))
- Verify WCAG reduced-motion coverage, split it out of the contrast/focus item ([#145](https://github.com/foxly-it/rootguard/pull/145))
- Add HTTPS/reverse-proxy deployment guide, verify secure-cookie enforcement ([#146](https://github.com/foxly-it/rootguard/pull/146))
- Document AdGuard backup/restore ownership ([#151](https://github.com/foxly-it/rootguard/pull/151))
- Sync ROADMAP.md status against the public website ([#154](https://github.com/foxly-it/rootguard/pull/154))
- Fix a stale roadmap checkbox and persist a sequenced next-steps plan ([#157](https://github.com/foxly-it/rootguard/pull/157))
- Correct roadmap items against a code audit, not assumptions ([#163](https://github.com/foxly-it/rootguard/pull/163))
- Note the unbound.conf importer hardening and host-table UI polish

### Fixed

- Show Unbound expert base config always, not behind a click ([#102](https://github.com/foxly-it/rootguard/pull/102))

### Maintenance

- Stop tracking AI assistant tooling files
- Gitignore the local-only AI memory file
- Rename the local AI context file to something inconspicuous

### Other

- Bump the actions group across 1 directory with 6 updates ([#94](https://github.com/foxly-it/rootguard/pull/94))
- Refresh README/website for alpha.6, fix stale monorepo/UI references ([#96](https://github.com/foxly-it/rootguard/pull/96))
- Ship RootGuard Blockpage: fresh design + automatic AdGuard wiring ([#98](https://github.com/foxly-it/rootguard/pull/98))
- Redesign blockpage around AdGuard's own color language, not RootGuard's ([#100](https://github.com/foxly-it/rootguard/pull/100))
- Make Unbound tabs URL-addressable, wire precise search-to-tab navigation ([#106](https://github.com/foxly-it/rootguard/pull/106))
- Add a blockpage preview link in Setup, fix a real contrast bug found while building it ([#112](https://github.com/foxly-it/rootguard/pull/112))
- Reason-lookup backend proxying AdGuard's check_host ([#117](https://github.com/foxly-it/rootguard/pull/117))
- Show the real per-request block reason, not a static list ([#118](https://github.com/foxly-it/rootguard/pull/118))
- Visual redesign - cards and motion, not a dashboard ([#119](https://github.com/foxly-it/rootguard/pull/119))
- Fix reason card never highlighting on a real match ([#120](https://github.com/foxly-it/rootguard/pull/120))
- Accessibility verification pass: contrast, focus trap, keyboard access, zoom/reflow ([#110](https://github.com/foxly-it/rootguard/pull/110))
- Document and test fixed-base hardening directives ([#122](https://github.com/foxly-it/rootguard/pull/122))
- Refresh dashboard preview, drop off-topic nav, fix real gaps ([#123](https://github.com/foxly-it/rootguard/pull/123))
- Hover feedback for ContentModal's close button + a real contrast fix found along the way ([#124](https://github.com/foxly-it/rootguard/pull/124))
- Narrative redesign with a RootGuard fox mascot ([#125](https://github.com/foxly-it/rootguard/pull/125))
- Add jump-to-top button ([#126](https://github.com/foxly-it/rootguard/pull/126))
- Fix header GitHub link pointing at the archived component repo ([#128](https://github.com/foxly-it/rootguard/pull/128))
- Typed host-inventory backend (zone-centred local hosts) ([#129](https://github.com/foxly-it/rootguard/pull/129))
- Fix sticky section-nav unsticking early + style blockpage preview as a button ([#130](https://github.com/foxly-it/rootguard/pull/130))
- Bump nanoid to clear a high-severity advisory ([#134](https://github.com/foxly-it/rootguard/pull/134))
- FRITZ!Box TR-064 host discovery adapter ([#133](https://github.com/foxly-it/rootguard/pull/133))
- Float Unbound's section nav as an icon rail on wide viewports ([#135](https://github.com/foxly-it/rootguard/pull/135))
- Anchor the floating rail to content instead of the viewport edge ([#136](https://github.com/foxly-it/rootguard/pull/136))
- FRITZ!Box import UI - discover, rename, select, activate ([#137](https://github.com/foxly-it/rootguard/pull/137))
- Fix the section-nav rail rendering way too wide ([#138](https://github.com/foxly-it/rootguard/pull/138))
- Sync roadmap.html's 0.2 status with what actually shipped ([#139](https://github.com/foxly-it/rootguard/pull/139))
- Keep local_zones when switching resolver presets ([#140](https://github.com/foxly-it/rootguard/pull/140))
- Fix misaligned columns in the Cache & performance card ([#141](https://github.com/foxly-it/rootguard/pull/141))
- Restyle the Unbound in-page rail as a left-gutter "on this page" widget ([#142](https://github.com/foxly-it/rootguard/pull/142))
- Add session inventory and revocation ([#147](https://github.com/foxly-it/rootguard/pull/147))
- Add login/recovery rate limiting and an auth audit log ([#148](https://github.com/foxly-it/rootguard/pull/148))
- Surface AdGuard's own reported version ([#149](https://github.com/foxly-it/rootguard/pull/149))
- Add contextual deep-links from AdGuard status to native AdGuard pages ([#150](https://github.com/foxly-it/rootguard/pull/150))
- Fix a broken healthcheck on the distroless runtime image ([#152](https://github.com/foxly-it/rootguard/pull/152))
- Fix toggle-row hover background overhanging the advanced-settings card ([#153](https://github.com/foxly-it/rootguard/pull/153))
- Redesign AdGuard status panel and add filtering toggle ([#155](https://github.com/foxly-it/rootguard/pull/155))
- Close layout gaps on Setup network step and Dashboard data-flow card ([#156](https://github.com/foxly-it/rootguard/pull/156))
- Fix Unbound section-nav rail links overflowing the widget ([#159](https://github.com/foxly-it/rootguard/pull/159))
- Add cross-service Client -> AdGuard -> Unbound -> DNSSEC diagnostics ([#160](https://github.com/foxly-it/rootguard/pull/160))
- Align the section-nav rail with the tab strip and ease truncation ([#161](https://github.com/foxly-it/rootguard/pull/161))
- Fix webapp's attestation policy pointing at the archived per-repo ([#166](https://github.com/foxly-it/rootguard/pull/166))
- Migrate guided zones onto the typed host-inventory model ([#167](https://github.com/foxly-it/rootguard/pull/167))
- Extract the shared guided draft/preview/activate workflow ([#168](https://github.com/foxly-it/rootguard/pull/168))
- Enforce hostname uniqueness across zones ([#169](https://github.com/foxly-it/rootguard/pull/169))
- Import/export of the complete logical resolver configuration ([#170](https://github.com/foxly-it/rootguard/pull/170))
- Classify and adopt a hand-written unbound.conf (partial) ([#171](https://github.com/foxly-it/rootguard/pull/171))
- Fix missing spacing between Advanced-tab cards ([#172](https://github.com/foxly-it/rootguard/pull/172))
- Reverse-map forward-zone blocks in the unbound.conf importer ([#173](https://github.com/foxly-it/rootguard/pull/173))
- Reverse-map local-zone host inventory in the unbound.conf importer ([#174](https://github.com/foxly-it/rootguard/pull/174))
- Reverse-map RFC1918 reverse-zone policy, network mode, and resource ([#175](https://github.com/foxly-it/rootguard/pull/175))
- Validate zone names/addresses before classifying as guided; fix filter-chip contrast ([#176](https://github.com/foxly-it/rootguard/pull/176))
- Fix PTR trailing-dot rejection and custom-config accumulation on re-import ([#177](https://github.com/foxly-it/rootguard/pull/177))
- Show local zone hosts as a table instead of an inline chip list ([#178](https://github.com/foxly-it/rootguard/pull/178))
- Update the public roadmap page for the 0.2 Unbound work

## [0.1.0-alpha.6] - 2026-08-06

### Added

- Full-stack container integration with stable SPA routing
- Professional dashboard layout with status indicator
- Stabilize router + SPA fallback, add dashboard API + service endpoint, UI glass layout baseline
- Implement service detection engine
- Implement docker container lifecycle control
- Introduce rootguard service model
- Introduce service registry
- Select AdGuard release channel ([#46](https://github.com/foxly-it/rootguard/pull/46))
- Show cryptographic release provenance ([#47](https://github.com/foxly-it/rootguard/pull/47))
- Introduce semantic design tokens with System/Light/Dark support ([#54](https://github.com/foxly-it/rootguard/pull/54))
- Migrate Dashboard to design tokens ([#56](https://github.com/foxly-it/rootguard/pull/56))
- Migrate remaining pages to design tokens ([#58](https://github.com/foxly-it/rootguard/pull/58))
- Rework as a coherent utility bar ([#60](https://github.com/foxly-it/rootguard/pull/60))
- Consolidate language, appearance, and sign-out into a user menu ([#62](https://github.com/foxly-it/rootguard/pull/62))
- Add global, local-only search with S / Ctrl+Cmd+K ([#64](https://github.com/foxly-it/rootguard/pull/64))
- Move collapse control to bottom edge, default desktop to collapsed ([#68](https://github.com/foxly-it/rootguard/pull/68))
- Add AdGuard release channels ([#32](https://github.com/foxly-it/rootguard/pull/32))
- Verify release attestations ([#33](https://github.com/foxly-it/rootguard/pull/33))
- Stable rootguard unbound base image with dnssec, internal bind and production defaults

### CI

- Add GHCR multi-arch pipeline + improved Dockerfile
- Restore id-token/attestations permissions dropped during port

### Changed

- Remove duplicate service model file

### Documentation

- Add rootguard-blockpage roadmap item ([#84](https://github.com/foxly-it/rootguard/pull/84))
- Add expert editor fullscreen + inline base config roadmap item ([#85](https://github.com/foxly-it/rootguard/pull/85))
- Catch up ROADMAP.md/project-state.md for items 7 and 8 (missed earlier) ([#86](https://github.com/foxly-it/rootguard/pull/86))
- Mark 4 already-shipped roadmap items as done (audit finding) ([#87](https://github.com/foxly-it/rootguard/pull/87))

### Fixed

- Regenerate package-lock for CI build
- Extend service model to support detection layer fields
- Preserve service provenance fields ([#48](https://github.com/foxly-it/rootguard/pull/48))
- Bump brace-expansion and postcss to clear high-severity audit gate ([#52](https://github.com/foxly-it/rootguard/pull/52))
- Derive service KPI total from allowlist instead of hardcoded 2 ([#50](https://github.com/foxly-it/rootguard/pull/50))
- Remove distracting hero circles, fix search placement ([#66](https://github.com/foxly-it/rootguard/pull/66))
- Verify exact SLSA v1 predicate ([#34](https://github.com/foxly-it/rootguard/pull/34))
- Install unbound-anchor for automatic dnssec trust anchor generation
- Bind unbound to 0.0.0.0 for docker port mapping compatibility
- Use writable trust anchor in /var/lib/unbound (Debian 13 compliant)
- Explicit IPv4 + IPv6 binding for Docker compatibility

### Maintenance

- Gitignore local AI-assistant tooling (.claude/, CLAUDE.md) ([#76](https://github.com/foxly-it/rootguard/pull/76))
- Bump rootguard-webapp submodule for design-token theme system ([#77](https://github.com/foxly-it/rootguard/pull/77))
- Bump rootguard-webapp submodule for Dashboard design-token migration ([#78](https://github.com/foxly-it/rootguard/pull/78))
- Bump rootguard-webapp submodule, theme-tokens roadmap item complete ([#79](https://github.com/foxly-it/rootguard/pull/79))
- Bump rootguard-webapp submodule for header utility-bar rework ([#80](https://github.com/foxly-it/rootguard/pull/80))
- Bump rootguard-webapp submodule for the accessible user menu ([#81](https://github.com/foxly-it/rootguard/pull/81))
- Bump rootguard-webapp submodule for global search ([#82](https://github.com/foxly-it/rootguard/pull/82))
- Bump rootguard-webapp submodule for hero-circle/search-header fix ([#83](https://github.com/foxly-it/rootguard/pull/83))

### Other

- Add verified RootGuard product screenshots
- Sync Core API docs and WebGUI roadmap
- Integrate tested rootless Podman documentation
- Bump rootguard-webapp submodule: Dashboard KPI fix + audit-gate bump ([#75](https://github.com/foxly-it/rootguard/pull/75))
- Bump rootguard-webapp: theme-aware shadow fix for washed-out light mode ([#89](https://github.com/foxly-it/rootguard/pull/89))
- Bump rootguard-webapp: expert editor fullscreen + inline base config ([#91](https://github.com/foxly-it/rootguard/pull/91))
- Remove git submodules ahead of monorepo migration
- Initial commit
- Add initial backend implementation (health + version endpoints)
- Add professional project README
- Add docker stats engine, system API and dashboard metrics
- Connect WebApp to Core and add Unbound settings ([#12](https://github.com/foxly-it/rootguard/pull/12))
- Add AdGuard bootstrap interface ([#13](https://github.com/foxly-it/rootguard/pull/13))
- Add Unbound preview rollback and diagnostics UI ([#14](https://github.com/foxly-it/rootguard/pull/14))
- Add Unbound advisor and preset interface ([#15](https://github.com/foxly-it/rootguard/pull/15))
- Add Unbound expert configuration editor ([#16](https://github.com/foxly-it/rootguard/pull/16))
- Add secure AIO management interface ([#17](https://github.com/foxly-it/rootguard/pull/17))
- Enforce frontend dependency security audit ([#18](https://github.com/foxly-it/rootguard/pull/18))
- Add guided conditional forwarding interface ([#19](https://github.com/foxly-it/rootguard/pull/19))
- Guide unsigned private forward zones ([#20](https://github.com/foxly-it/rootguard/pull/20))
- Guide private answers for forward zones ([#21](https://github.com/foxly-it/rootguard/pull/21))
- Clarify forwarding authority checks ([#22](https://github.com/foxly-it/rootguard/pull/22))
- Fix alpha dependency security audit ([#23](https://github.com/foxly-it/rootguard/pull/23))
- Add secure local password recovery ([#24](https://github.com/foxly-it/rootguard/pull/24))
- Improve GitHub project discoverability ([#25](https://github.com/foxly-it/rootguard/pull/25))
- Explain service runtime state in Stack Center ([#27](https://github.com/foxly-it/rootguard/pull/27))
- Add on-demand service diagnostics ([#28](https://github.com/foxly-it/rootguard/pull/28))
- Show update and cleanup history ([#29](https://github.com/foxly-it/rootguard/pull/29))
- Show actionable installation diagnostics ([#30](https://github.com/foxly-it/rootguard/pull/30))
- Add guided private DNS controls ([#31](https://github.com/foxly-it/rootguard/pull/31))
- Add guided resolver protocol modes ([#32](https://github.com/foxly-it/rootguard/pull/32))
- Explain containers without healthchecks ([#33](https://github.com/foxly-it/rootguard/pull/33))
- Polish Unbound fields and runtime badge ([#34](https://github.com/foxly-it/rootguard/pull/34))
- Polish management UI ([#35](https://github.com/foxly-it/rootguard/pull/35))
- Add Unbound resource profile controls ([#36](https://github.com/foxly-it/rootguard/pull/36))
- Preserve Unbound resource profiles in proxy ([#37](https://github.com/foxly-it/rootguard/pull/37))
- Add serve-expired controls to Webapp ([#38](https://github.com/foxly-it/rootguard/pull/38))
- Add DNSSEC cache controls ([#39](https://github.com/foxly-it/rootguard/pull/39))
- Expose EDNS buffer size control ([#40](https://github.com/foxly-it/rootguard/pull/40))
- Expose temporary diagnostic logging ([#41](https://github.com/foxly-it/rootguard/pull/41))
- Add collapsible navigation sidebar ([#42](https://github.com/foxly-it/rootguard/pull/42))
- Show AdGuard filter diagnostics ([#43](https://github.com/foxly-it/rootguard/pull/43))
- Unify actions and focus AdGuard diagnostics
- Show trusted stack release metadata ([#45](https://github.com/foxly-it/rootguard/pull/45))
- Keep sidebar navigation and collapse control visible while long pages scroll ([#70](https://github.com/foxly-it/rootguard/pull/70))
- Make box-shadow color/intensity theme-aware to fix washed-out light mode ([#72](https://github.com/foxly-it/rootguard/pull/72))
- Add fullscreen mode and inline base-config reference to expert editor ([#74](https://github.com/foxly-it/rootguard/pull/74))
- Initial project structure
- Add Apache 2.0 license
- Add NOTICE
- Build secure RootGuard control plane ([#1](https://github.com/foxly-it/rootguard/pull/1))
- Add secure AdGuard bootstrap control ([#2](https://github.com/foxly-it/rootguard/pull/2))
- Add versioned Unbound configuration lifecycle ([#4](https://github.com/foxly-it/rootguard/pull/4))
- Add Unbound presets and advisor ([#5](https://github.com/foxly-it/rootguard/pull/5))
- Add safe Unbound custom configuration ([#6](https://github.com/foxly-it/rootguard/pull/6))
- Build AIO control plane and safe updates ([#7](https://github.com/foxly-it/rootguard/pull/7))
- Add guided conditional forwarding ([#8](https://github.com/foxly-it/rootguard/pull/8))
- Support unsigned private forward zones ([#9](https://github.com/foxly-it/rootguard/pull/9))
- Allow private answers per forward zone ([#10](https://github.com/foxly-it/rootguard/pull/10))
- Require authoritative forwarding probes ([#11](https://github.com/foxly-it/rootguard/pull/11))
- Stabilize forwarding probe errors ([#12](https://github.com/foxly-it/rootguard/pull/12))
- Improve GitHub project discoverability ([#13](https://github.com/foxly-it/rootguard/pull/13))
- Expose trustworthy service runtime metadata ([#15](https://github.com/foxly-it/rootguard/pull/15))
- Add bounded redacted service diagnostics ([#16](https://github.com/foxly-it/rootguard/pull/16))
- Add safe update cleanup history ([#17](https://github.com/foxly-it/rootguard/pull/17))
- Add typed installation diagnostics ([#18](https://github.com/foxly-it/rootguard/pull/18))
- Add guided private DNS settings ([#19](https://github.com/foxly-it/rootguard/pull/19))
- Add capability-checked resolver modes ([#20](https://github.com/foxly-it/rootguard/pull/20))
- Distinguish missing container healthchecks ([#21](https://github.com/foxly-it/rootguard/pull/21))
- Add live dashboard metrics ([#22](https://github.com/foxly-it/rootguard/pull/22))
- Add bounded Unbound resource profiles ([#23](https://github.com/foxly-it/rootguard/pull/23))
- Add guided serve-expired controls ([#24](https://github.com/foxly-it/rootguard/pull/24))
- Add DNSSEC cache controls ([#25](https://github.com/foxly-it/rootguard/pull/25))
- Add EDNS buffer size control ([#26](https://github.com/foxly-it/rootguard/pull/26))
- Add privacy-safe diagnostic logging ([#27](https://github.com/foxly-it/rootguard/pull/27))
- Migrate service volume ownership during updates ([#28](https://github.com/foxly-it/rootguard/pull/28))
- Add AdGuard filter diagnostics and DNS defaults ([#29](https://github.com/foxly-it/rootguard/pull/29))
- Fix AdGuard DNS baseline status ([#30](https://github.com/foxly-it/rootguard/pull/30))
- Expose stack release metadata ([#31](https://github.com/foxly-it/rootguard/pull/31))
- Initial rootguard-unbound engine (Debian based) + GH Actions
- Fix Dockerfile: remove entrypoint and stabilize build
- Use system user instead of fixed UID
- Use packaged unbound system user
- Add MIT license and README
- Enterprise versioning: automatic Debian Unbound tagging
- Fix Docker tag: sanitize Debian version (+ → -)
- Update README.md
- Finalize recursive DNSSEC configuration with persistent trust anchor
- Finalize Debian-based recursive DNSSEC configuration (enterprise commented)
- Run container as root (Debian compliant), let unbound drop privileges internally
- Install dns-root-data package to provide Debian root.key
- Container-optimized recursive DNSSEC configuration with documentation
- Finalize enterprise container-optimized Unbound configuration
- Finalize container-optimized Unbound configuration (no pidfile, no syslog)
- Move DNSSEC trust anchor to /var/lib/unbound (writable runtime path)
- Add runtime entrypoint to fix volume ownership and drop privileges correctly
- Finalize runtime bootstrap: proper trust anchor handling without su
- Enterprise runtime model: root PID1 + internal privilege drop
- Enterprise entrypoint: state init + ownership fix + DNSSEC bootstrap
- Replace unbound-anchor bootstrap with Debian trust anchor copy
- Remove unbound-anchor, use Debian dns-root-data trust anchor
- Finalize immutable runtime model with documented unbound.conf
- Finalize immutable runtime model with documented unbound.conf
- Remove legacy entrypoint.sh, keep docker-entrypoint.sh
- Production-ready rootguard unbound base image
- Production-ready rootguard unbound image without anchor hard-fail
- Debian 13 compliant rootguard unbound image using dns-root-data trust anchor
- Debian 13 compliant writable trust anchor under /var/lib/unbound
- Docker-compatible access-control with bridge network support
- Rootguard-ready modular unbound base (unbound.d includes)
- Update README.md
- Harden Unbound for RootGuard runtime ([#1](https://github.com/foxly-it/rootguard/pull/1))
- Adopt AGPL-3.0-or-later ([#2](https://github.com/foxly-it/rootguard/pull/2))
- Allow versioned RootGuard image tags ([#3](https://github.com/foxly-it/rootguard/pull/3))
- Improve GitHub project discoverability ([#4](https://github.com/foxly-it/rootguard/pull/4))
- Upgrade Unbound security baseline to 1.25.2 ([#6](https://github.com/foxly-it/rootguard/pull/6))
- Enable loopback-only runtime control ([#7](https://github.com/foxly-it/rootguard/pull/7))
- Keep Unbound volume identity stable ([#8](https://github.com/foxly-it/rootguard/pull/8))
- Build Unbound reproducibly from verified source ([#9](https://github.com/foxly-it/rootguard/pull/9))
- Prepare RootGuard alpha foundation and documentation ([#5](https://github.com/foxly-it/rootguard/pull/5))
- Add safe update lifecycle history ([#34](https://github.com/foxly-it/rootguard/pull/34))
- Prepare standalone updater component ([#1](https://github.com/foxly-it/rootguard/pull/1))
- Test real paired update and rollback ([#3](https://github.com/foxly-it/rootguard/pull/3))
- Bump the actions group across 1 directory with 6 updates ([#2](https://github.com/foxly-it/rootguard/pull/2))
- Rebuild CI, docs, and workflow tooling for the monorepo

## [0.1.0-alpha.5] - 2026-08-01

### Added

- Add AdGuard release channel setup ([#66](https://github.com/foxly-it/rootguard/pull/66))
- Integrate signed release provenance ([#67](https://github.com/foxly-it/rootguard/pull/67))

### Fixed

- Use exact SLSA v1 attestation predicate ([#68](https://github.com/foxly-it/rootguard/pull/68))
- Preserve Stack Center provenance status ([#69](https://github.com/foxly-it/rootguard/pull/69))

### Other

- Integrate WebApp action audit
- Integrate reproducible Unbound source build ([#63](https://github.com/foxly-it/rootguard/pull/63))
- Complete Unbound amd64 update verification ([#64](https://github.com/foxly-it/rootguard/pull/64))
- Integrate stack release metadata ([#65](https://github.com/foxly-it/rootguard/pull/65))
- Prepare v0.1.0-alpha.5 ([#70](https://github.com/foxly-it/rootguard/pull/70))

## [0.1.0-alpha.4] - 2026-08-01

### Other

- Integrate Unbound resource profiles ([#50](https://github.com/foxly-it/rootguard/pull/50))
- Pin patched Unbound 1.25.2 image ([#51](https://github.com/foxly-it/rootguard/pull/51))
- Integrate serve-expired controls ([#52](https://github.com/foxly-it/rootguard/pull/52))
- Integrate DNSSEC cache controls ([#53](https://github.com/foxly-it/rootguard/pull/53))
- Integrate EDNS buffer size control ([#54](https://github.com/foxly-it/rootguard/pull/54))
- Integrate privacy-safe diagnostic logging ([#55](https://github.com/foxly-it/rootguard/pull/55))
- Integrate sidebar and matching preview icons ([#56](https://github.com/foxly-it/rootguard/pull/56))
- Match preview KPI icons to dashboard ([#57](https://github.com/foxly-it/rootguard/pull/57))
- Integrate safe Unbound volume migration ([#58](https://github.com/foxly-it/rootguard/pull/58))
- Prioritize reproducible Unbound build ([#59](https://github.com/foxly-it/rootguard/pull/59))
- Integrate AdGuard filter diagnostics ([#60](https://github.com/foxly-it/rootguard/pull/60))
- Prepare RootGuard v0.1.0-alpha.4

## [0.1.0-alpha.3] - 2026-07-29

### Other

- Record alpha.2 release status ([#17](https://github.com/foxly-it/rootguard/pull/17))
- Add live project overview to website ([#18](https://github.com/foxly-it/rootguard/pull/18))
- Simplify project website experience ([#19](https://github.com/foxly-it/rootguard/pull/19))
- Improve GitHub project discoverability ([#20](https://github.com/foxly-it/rootguard/pull/20))
- Stabilize manual section navigation ([#22](https://github.com/foxly-it/rootguard/pull/22))
- Update component repository presentation ([#23](https://github.com/foxly-it/rootguard/pull/23))
- Animate hero dashboard preview ([#24](https://github.com/foxly-it/rootguard/pull/24))
- Use production environment terminology ([#25](https://github.com/foxly-it/rootguard/pull/25))
- Add project legal pages and tool navigation ([#26](https://github.com/foxly-it/rootguard/pull/26))
- Group and polish public site navigation ([#27](https://github.com/foxly-it/rootguard/pull/27))
- Correct AdGuard Home Updater naming ([#28](https://github.com/foxly-it/rootguard/pull/28))
- Integrate language switch into navigation ([#29](https://github.com/foxly-it/rootguard/pull/29))
- Unify site header and documentation naming ([#30](https://github.com/foxly-it/rootguard/pull/30))
- Sync roadmap and Stack Center visibility ([#31](https://github.com/foxly-it/rootguard/pull/31))
- Publish bounded Stack Center diagnostics ([#32](https://github.com/foxly-it/rootguard/pull/32))
- Refine AdGuard integration and cleanup roadmap ([#33](https://github.com/foxly-it/rootguard/pull/33))
- Add safe update lifecycle history ([#34](https://github.com/foxly-it/rootguard/pull/34))
- Pin alpha images by digest ([#35](https://github.com/foxly-it/rootguard/pull/35))
- Pin standalone updater component ([#36](https://github.com/foxly-it/rootguard/pull/36))
- Pin real update rollback test ([#37](https://github.com/foxly-it/rootguard/pull/37))
- Publish installation diagnostics milestone ([#38](https://github.com/foxly-it/rootguard/pull/38))
- Add clean install platform matrix ([#40](https://github.com/foxly-it/rootguard/pull/40))
- Add guided private DNS lifecycle ([#42](https://github.com/foxly-it/rootguard/pull/42))
- Add checked resolver protocol modes ([#44](https://github.com/foxly-it/rootguard/pull/44))
- Unify Docs link colors ([#45](https://github.com/foxly-it/rootguard/pull/45))
- Integrate explicit healthcheck states ([#46](https://github.com/foxly-it/rootguard/pull/46))
- Integrate RootGuard UI polish ([#47](https://github.com/foxly-it/rootguard/pull/47))
- Integrate dashboard metrics and UI polish ([#48](https://github.com/foxly-it/rootguard/pull/48))
- Prepare RootGuard v0.1.0-alpha.3 ([#49](https://github.com/foxly-it/rootguard/pull/49))

## [0.1.0-alpha.2] - 2026-07-26

### Other

- Announce public alpha on website ([#14](https://github.com/foxly-it/rootguard/pull/14))
- Integrate local password recovery ([#15](https://github.com/foxly-it/rootguard/pull/15))
- Prepare RootGuard v0.1.0-alpha.2 ([#16](https://github.com/foxly-it/rootguard/pull/16))

## [0.1.0-alpha.1] - 2026-07-26

### Other

- Initialize RootGuard project repository
- Integrate RootGuard stack and project website ([#1](https://github.com/foxly-it/rootguard/pull/1))
- Update GitHub Pages actions ([#2](https://github.com/foxly-it/rootguard/pull/2))
- Integrate Unbound configuration v2 ([#3](https://github.com/foxly-it/rootguard/pull/3))
- Integrate Unbound advisor and project state ([#4](https://github.com/foxly-it/rootguard/pull/4))
- Prepare RootGuard alpha foundation and documentation ([#5](https://github.com/foxly-it/rootguard/pull/5))
- Integrate guided conditional forwarding ([#6](https://github.com/foxly-it/rootguard/pull/6))
- Allow private answers for forward zones ([#7](https://github.com/foxly-it/rootguard/pull/7))
- Require authoritative forwarding checks ([#8](https://github.com/foxly-it/rootguard/pull/8))
- Refresh dashboard website preview ([#9](https://github.com/foxly-it/rootguard/pull/9))
- Prepare public RootGuard alpha Compose release ([#10](https://github.com/foxly-it/rootguard/pull/10))
- Respect component package ownership ([#11](https://github.com/foxly-it/rootguard/pull/11))
- Publish WebApp from its component repository ([#12](https://github.com/foxly-it/rootguard/pull/12))
- Pin published alpha component revisions ([#13](https://github.com/foxly-it/rootguard/pull/13))


