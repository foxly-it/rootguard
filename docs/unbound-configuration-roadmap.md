# Unbound configuration plan

Last reviewed: 2026-07-23

RootGuard should expose useful resolver configuration without becoming an
unrestricted text editor for system-critical Unbound settings. Each directive
belongs to one of four ownership levels.

## Ownership model

| Level | Meaning | Examples |
|---|---|---|
| Fixed base | Secure RootGuard default, visible but not freely editable | listeners, remote control, root hints, trust anchor, identity hiding |
| Guided | Typed WebGUI form with explanations and validation | cache TTL, threads, forwarding zones, access CIDRs |
| Expert | Allowed only in `90-rootguard-custom.conf` with policy checks | narrowly supported advanced server/zone directives |
| Blocked | Cannot be supplied from the browser | includes, file paths, control interfaces, trust-anchor replacement |

Every guided value requires a safe default, type/range validation, generated
comments, preview, `unbound-checkconf`, activation health check, versioned
history, and rollback.

## Existing guided settings

| Function | Directive | State |
|---|---|---|
| Query minimisation | `qname-minimisation` | Delivered |
| Popular-record refresh | `prefetch` | Delivered |
| Availability during outages | `serve-expired` | Delivered |
| Minimum cache TTL | `cache-min-ttl` | Delivered |
| Maximum cache TTL | `cache-max-ttl` | Delivered |
| Resolver parallelism | `num-threads` | Delivered |
| Local records | `local-zone`, `local-data` | Delivered for A/AAAA/CNAME |
| Conditional forwarding | `forward-zone`, `name`, `forward-addr`, `forward-first` | Delivered |

## Priority A — required for useful 0.2 administration

### Conditional forwarding — delivered

- Multiple zones with canonical FQDN validation
- Multiple IPv4/IPv6 targets per zone
- Optional TLS only when a supported authenticated target model exists
- Loop detection against RootGuard, AdGuard, and other forwarding zones
- Authoritative zone check (`NOERROR` plus SOA) and clear fallback semantics
- Directives: `forward-zone`, `name`, `forward-addr`, optionally
  `forward-first`, zone-scoped `domain-insecure`, and zone-scoped
  `private-domain`, all with explicit warnings

RootGuard now owns this group as typed settings. The WebGUI preserves target
order, probes every target from the running Unbound container, blocks
RootGuard-internal and local loop targets, rejects duplicate zones and expert
`forward-zone` conflicts, and accepts a target only when the configured zone
returns `NOERROR` with an SOA record. `NXDOMAIN`, `REFUSED`, timeouts, and empty
successful responses stay visible as diagnostics but cannot unlock activation.
The settings remain in the existing preview, effective-checkconf, history, and
rollback lifecycle. Authenticated DNS-over-TLS forwarding remains intentionally
deferred until certificate identity can be modeled safely. DNSSEC validation
remains enabled by default. A trusted private
zone whose internal server returns unsigned answers can explicitly opt into a
zone-scoped exception; the generated directive and active policy remain visible
in the preview and zone card. RFC1918 and other protected private-address
answers stay blocked by default and require a separate, visible zone-scoped
opt-in, preserving rebinding protection everywhere else.

### Private domains and reverse DNS — delivered

- Guided, canonical private-domain list with duplicate and ownership checks
- Explicit RFC1918 reverse-zone policy per network: safe NXDOMAIN or transparent
  fallback with a leakage warning
- Optional local PTR records derived from guided A/AAAA entries only when the
  address is unambiguous across all guided zones
- Typed preview, effective `unbound-checkconf`, version history, activation,
  rollback, and expert-setting conflict detection
- Directives: `private-domain`, `local-zone`, `local-data-ptr`

RootGuard now owns the three standard RFC1918 ranges as typed settings. New and
migrated installations default to NXDOMAIN; transparent fallback requires a
separate choice for each range. The local-zone assistant offers PTR generation
for A and AAAA records, suppresses ambiguous duplicates, and keeps the generated
reverse data in the same visible, reversible configuration lifecycle.
AdGuard Home's separate private reverse-resolver routing remains part of the
later cross-service integration instead of being reimplemented in RootGuard.

### Zone-centred host inventory — delivered

The guided local-zone assistant (`UnboundGuidedZones.tsx`) presents named
hosts grouped below a zone, so an operator can create `home.lab.` once and
manage named clients, servers, gateways, access points, printers, and similar
local systems without editing individual Unbound directives. It's backed by
the typed `LocalZone`/`LocalHost` model (`rootguard-core/internal/unbound/
settings.go`), not custom-config text.

- Add, rename, remove, and bulk-edit hosts with IPv4 and/or IPv6 addresses.
- Generate PTR records only after the shared uniqueness and reverse-zone
  checks pass - duplicate PTR claims across any zone are a hard validation
  error, not a silently suppressed one.
- Keep the term *client* for AdGuard Home policy ownership. Unbound entries are
  local DNS hosts and do not represent end-client access rules.
- Apply changes only through the shared preview, effective `unbound-checkconf`,
  version history, activation health check, and rollback path.

CNAME records and per-host TTL are deliberately out of scope for this guided
surface (see [issue #131](https://github.com/foxly-it/rootguard/issues/131)):
the typed model was never scoped to support them, and the rare CNAME/TTL case
routes to the unrestricted expert editor instead.

### Router import — partially delivered

- Bounded discovery from an optional router adapter: the FRITZ!Box adapter
  (`rootguard-core/internal/routerimport`) speaks the documented TR-064
  host-list interface and lands discovered hosts as an untrusted draft -
  source, hostname, address, and conflicts shown before selection, applied
  only through the same typed model and preview/activate lifecycle as the
  host inventory above.
- Router credentials stay in bounded server-side request handling only -
  never browser storage, logs, history, generated configuration, or
  diagnostic responses.
- Still planned: bounded discovery from `in-addr.arpa`/`ip6.arpa` without a
  router, and adapters for vendors beyond FRITZ!Box (same normalized import
  contract).

### Existing-configuration import — partially delivered

Operators arriving with a hand-written `unbound.conf` from a prior manual
setup should not have to retype it directive by directive to get useful
guided coverage. A bounded importer (`rootguard-core/internal/unbound/
import.go`'s `ImportUnboundConf`, `UnboundConfImport.tsx`) accepts a pasted
or uploaded existing configuration, parses its directives, and classifies
each one against the ownership model above before offering anything for
adoption.

- Directives already covered by the fixed base (e.g. `hide-identity`,
  `hide-version`, `harden-*`, `unwanted-reply-threshold`, `root-hints`,
  `auto-trust-anchor-file`) are filtered out and never presented for import.
  A directive whose imported value actually conflicts with the fixed base
  (compared exactly for the simple scalar ones) is reported as rejected with
  the reason, not silently dropped. **Delivered.**
- Directives that map cleanly onto an existing guided setting are applied to
  a candidate `Settings` for review before activation. **Delivered for the
  ~12 flat `server:` scalars** (qname-minimisation, prefetch(-key),
  aggressive-nsec, serve-expired(-ttl/-client-timeout), cache-min/max-ttl,
  num-threads, edns-buffer-size, verbosity) **and for `forward-zone`
  blocks**: a clean fit (a name, one or more `forward-addr`, optionally
  `forward-first`) becomes a guided `ForwardZone`; the zone-scoped
  `domain-insecure`/`private-domain` opt-ins are resolved against the zones
  just extracted (a name match sets that zone's `AllowUnsigned`/
  `AllowPrivateAddresses`; a `private-domain` matching no zone joins the
  global guided list instead; a `domain-insecure` matching no zone has no
  meaning outside that context and is offered for expert adoption). A block
  that isn't a clean fit (missing a name/target, or containing a directive
  this importer doesn't reverse-map, e.g. `forward-tls-upstream`) falls back
  to whole-block expert adoption rather than silently dropping what doesn't
  fit. **Still open** for the remaining block-shaped or multi-directive
  guided settings: `local-zone`/`local-data`/`local-data-ptr` → the typed
  host inventory, RFC1918 reverse-zone policy, and resource-profile
  inference from `rrset-cache-size`/`msg-cache-size` - these are explicitly
  classified as blocked-pending-support today (not silently mishandled),
  and `do-ip4`/`do-ip6`/`prefer-ip6` the same pending network-mode
  reverse-mapping.
- Remaining directives that have no guided equivalent but pass the same
  expert allowlist as manual expert input are offered for adoption into
  `90-rootguard-custom.conf`, preserving whole clause blocks (e.g. a
  `forward-zone:` block that isn't a clean guided fit, reconstructed
  verbatim) rather than individual lines; anything on the permanently
  blocked list, or that opens a control surface (`remote-control`, `python`,
  `dynlib`, `dnstap`), is reported as unsupported, never silently accepted.
  **Delivered** - this is also today's path for the block-shaped guided
  directives above that don't have their own reverse-mapping yet.
- The imported file is treated as an untrusted draft, matching the
  router-import pattern above: classification is a pure, read-only step:
  applying it reuses the same `ConfigBundle` preview/activate lifecycle as
  the complete-configuration import/export feature (shared `unbound-checkconf`,
  version history, activation health check, and rollback path).
  **Delivered.**

### Client access networks — fixed ownership

RootGuard clients query AdGuard Home, not Unbound. Exposing Unbound CIDR rules
would therefore duplicate AdGuard policy while implying protection it cannot
provide. Unbound remains limited to loopback and RootGuard's internal DNS
network; end-client access policy stays in AdGuard Home. Raw interfaces,
listeners, and sockets remain unavailable to both guided and expert input.

### Network and protocol mode — delivered

- IPv4-only safe default plus dual-stack and IPv6-only guided modes
- Bounded IPv4 and IPv6 root-server probes from the running Unbound container
- IPv6-dependent modes stay locked in the WebGUI and are rejected server-side
  when authoritative connectivity is unavailable
- Typed preview, effective `unbound-checkconf`, history, rollback, migration,
  and expert ownership checks
- Directives: `do-ip4`, `do-ip6`, `prefer-ip6`

### Cache and resource profile

- Explicit small, balanced, and large resource profiles
- Calculate message, RRset, key, and negative cache values consistently
- Ensure slab counts remain valid for the thread count
- Directives: `msg-cache-size`, `rrset-cache-size`, `key-cache-size`,
  `neg-cache-size`, `msg-cache-slabs`, `rrset-cache-slabs`

### Resilience details

- Serve-expired duration and client response timeout
- Refresh DNSSEC keys for cached popular answers
- Aggressive NSEC as an explained DNSSEC optimisation
- Directives: `serve-expired-ttl`, `serve-expired-client-timeout`,
  `prefetch-key`, `aggressive-nsec`

### Transport safety

- Default EDNS buffer size `1232`
- Allow a narrow validated range only with a clear fragmentation warning
- Directive: `edns-buffer-size`

### Operational logging

- Default no query logging
- Short-lived diagnostic mode with automatic expiry
- Redact or avoid client/query data by default
- Directives: `verbosity`, `log-queries`, `log-replies`, `log-servfail`

## Priority B — fixed and visible hardening

These values should be tested and displayed in the live base configuration,
but not casually editable:

- `hide-identity`, `hide-version`
- `harden-glue`, `harden-dnssec-stripped`, `harden-below-nxdomain`
- `harden-referral-path` only after compatibility testing
- `use-caps-for-id` only if current Unbound guidance and compatibility justify it
- `unwanted-reply-threshold`
- root hints and automatic trust-anchor maintenance
- private-address protections for RFC1918, link-local, ULA, and documentation
  networks without breaking explicitly configured private domains

### Status: fixed base directives (implemented in `rootguard-unbound/unbound.conf`)

| Directive | Purpose | Tested by |
| --- | --- | --- |
| `hide-identity`, `hide-version` | Refuse CHAOS-class `id.server`/`version.server` probes that fingerprint the resolver for targeted exploits | `ci-unbound.yml`: `dig CH TXT id.server`/`version.server` return empty |
| `harden-glue` | Ignore untrusted glue records outside a delegation's own zone (glue/cache poisoning) | Directive-presence check in the built image (behavioral poisoning test would need a malicious authoritative test zone - out of scope here, same as upstream Unbound's own test suite boundary) |
| `harden-dnssec-stripped` | Treat an unsigned answer as bogus, not insecure, when the zone is known-signed (downgrade-attack resistance) | Directive-presence check; downgrade behavior is exercised indirectly by the existing `dnssec-failed.org` `SERVFAIL` check |
| `harden-below-nxdomain` | Trust a proven NXDOMAIN for the whole empty non-terminal space below it (RFC 8020), reducing needless lookups an attacker could otherwise abuse for enumeration | Directive-presence check |
| `aggressive-nsec` | Synthesize denial-of-existence from cached NSEC/NSEC3 without repeat upstream queries | Directive-presence check |
| `unwanted-reply-threshold` | Reset internal caches after `10000000` unsolicited/mismatched replies per thread - upstream's own recommended defensive-reset value, not a RootGuard tuning choice | Directive-presence check |
| `root-hints`, `auto-trust-anchor-file` | Debian `dns-root-data` root server list; RFC 5011 trust-anchor auto-maintenance in the writable `/var/lib/unbound/root.key` | Existing DNSSEC `dig +dnssec` / trust-anchor volume-compatibility checks |
| `private-address` (RFC1918, link-local, ULA) | Reject externally-sourced answers claiming to be these ranges (DNS rebinding protection) | Directive-presence check |

`harden-referral-path` and `use-caps-for-id` remain deliberately deferred, not
forgotten: `harden-referral-path` needs compatibility testing against
RootGuard's supported upstream/forwarding configurations first (it can break
legitimate non-compliant authoritative servers), and `use-caps-for-id`
depends on current upstream guidance justifying it as still worthwhile
against modern spoofing risk versus its 0x20-encoding compatibility cost.
Revisit both if upstream guidance or a real compatibility test changes the
calculus.

## Priority C — later or expert-only

- Stub zones for authoritative local servers
- Response Policy Zones if AdGuard policy cannot cover a validated use case
- DNS64/NAT64
- ECS behavior
- Authenticated DNS-over-TLS forwarding
- Per-zone advanced flags

These are not 0.2 gates and must not weaken the default recursive,
privacy-preserving RootGuard path.

## Permanently blocked browser controls

- arbitrary `include` and file paths
- `interface`, `port`, and control socket ownership
- `remote-control` and control certificates
- module loading
- trust-anchor replacement or DNSSEC bypass
- arbitrary shell commands, environment expansion, and container paths
- values already owned by the guided configuration

## Acceptance matrix

Every new guided group needs tests for:

1. valid minimum, recommended, and maximum values;
2. malformed values and size limits;
3. conflicts with existing guided and expert configuration;
4. generated comments and deterministic rendering;
5. candidate and effective `unbound-checkconf`;
6. successful DNS behavior after activation;
7. failed activation with automatic rollback;
8. history restore across settings and custom content;
9. German and English explanations;
10. keyboard, mobile, and screen-reader operation.
