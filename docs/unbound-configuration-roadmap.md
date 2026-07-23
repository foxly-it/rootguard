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

## Priority A — required for useful 0.2 administration

### Conditional forwarding

- Multiple zones with canonical FQDN validation
- Multiple IPv4/IPv6 targets per zone
- Optional TLS only when a supported authenticated target model exists
- Loop detection against RootGuard, AdGuard, and other forwarding zones
- Reachability check and clear fallback semantics
- Directives: `forward-zone`, `name`, `forward-addr`, optionally
  `forward-first` with an explicit warning

### Private domains and reverse DNS

- Guided private-domain list
- RFC1918 reverse zones with clear NXDOMAIN versus transparent behavior
- Local PTR records derived from guided A/AAAA entries where unambiguous
- Directives: `private-domain`, `local-zone`, `local-data-ptr`

### Client access networks

- Allow only validated host and CIDR entries
- Show rule order and effective result
- Protect the internal Docker networks required by RootGuard
- Never allow raw interface or socket selection
- Directive: `access-control`

### Network and protocol mode

- IPv4, IPv6, or dual stack selected from detected host capability
- Connectivity preflight before activation
- Safe handling when IPv6 exists locally but has no upstream connectivity
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
