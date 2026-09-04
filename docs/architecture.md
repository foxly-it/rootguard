**English** · [Deutsch](architecture.de.md)

# Repository structure

The RootGuard main repository coordinates the versions of the independent
components. Each component keeps its own history, dependencies, releases,
and development workflows.

```text
rootguard/
├── docs/
├── rootguard-attestation-proxy/
├── rootguard-blockpage/
├── rootguard-core/
├── rootguard-unbound/
├── rootguard-updater/
└── rootguard-webapp/
```

A commit in the main repository references exactly one commit per
submodule. This makes it possible to trace which combination of components
was used together at any point.

Local backups and stale working copies are deliberately not part of the
main repository.

## Runtime architecture

```text
Browser
  │ Login + HttpOnly session
  ▼
RootGuard WebApp
  │ internal bearer token
  ▼
RootGuard Core ────── Docker API ─── AdGuard Home + Unbound
  │
  │ narrow, internal update jobs
  ▼
Updater helper ────── Docker API ─── Core + WebApp
  │
  └── persistent status and atomic image pin
Persistent RootGuard data
```

Only the WebApp and DNS get host ports. AdGuard Home is reachable
exclusively on internal networks for Core; its randomly generated
credentials live with owner-only permissions in the persistent RootGuard
volume. Core and the separate, internally reachable updater helper each
have Docker access for different, narrowly scoped tasks. The WebApp knows
neither the socket nor host system commands.

## Login and local password recovery

The WebApp manages HttpOnly/SameSite-Strict sessions server-side in the
protected session volume. A separate `ROOTGUARD_RECOVERY_TOKEN` enables
only setting a new admin password on the login page. It grants neither a
session nor access to Core or AdGuard, and must be generated independently
of the admin password and internal API token.

After a successful reset, only a salted PBKDF2-SHA256 verifier with
600,000 iterations is stored in the session volume. Every existing session
becomes invalid. Without a configured recovery key, the local operator
path remains via `.env` and a controlled recreation of the WebApp
container; there is deliberately no email or cloud recovery service.

## AIO bootstrap and stack lifecycle

The public `compose.yaml` starts the persistent control plane consisting
of WebApp, Core, and the internal updater helper. The DNS data plane is
only created after an authenticated setup:

```text
Bootstrap Compose
  → WebApp + Core + updater helper
  → IP/port preflight
  → fixed DNS Compose specification
  → image pull
  → Unbound + health check
  → AdGuard Home + protected bootstrap
```

Core persists configuration, individual steps, and errors atomically under
`/var/lib/rootguard/installation`. The WebApp only sends typed network
details and the allowed AdGuard release channel `stable` or `beta`. A
missing channel value from older installations is treated as `stable`;
other values are rejected by Core. Image names, container privileges,
volumes, networks, and commands come from the controlled Core
specification and cannot be freely chosen via the browser API.

The controller is connected to the private DNS network after
installation. On restart or replacement of the Core container, it
re-establishes this connection based on the persisted installation state.
AdGuard continues to publish only TCP/UDP 53; its native administration
stays private. Bootstrap waits, with a bound, for the actual AdGuard
installer API - a running container alone doesn't yet count as ready.

For Core/WebApp/Updater/Unbound/Blockpage releases referenced immutably by
digest, Core (and, for its own two managed images, the separate updater
helper) additionally verifies the signed SLSA provenance before
activation. The embedded Cosign verifier, itself pinned by digest,
enforces the expected GitHub repository and workflow signer identity plus
the GitHub Actions OIDC issuer, and checks the Sigstore transparency data.
Results are cached for ten minutes. A missing attestation, a
cryptographically invalid one, and a temporarily unreachable registry are
deliberately reported as distinct states. Local builds, mutable tags, and
third-party images never receive RootGuard trust approval.

Core and the updater helper both run only on the internal `control`
network (no route to the internet at all), so this Cosign check needs a
narrow, explicit bridge to actually reach GHCR/Sigstore:
`rootguard-attestation-proxy`, a minimal CONNECT-only forward proxy with
a hardcoded, 3-host allowlist, sitting on both `control` and a separate,
real-internet-facing `egress` network. It's the sixth RootGuard
component, self-update managed via its own dedicated channel since
2026-09-03 (a fully separate update path from the other five, not a
shared one - see `docs/security-audit-log.md` for why) - see
`rootguard-attestation-proxy/README.md` and `docs/threat-model.md`
(§3) for the full design and trust model.

## AdGuard first-time setup

The WebApp offers only status and the explicit bootstrap action. Core uses
the typed AdGuard installer and DNS endpoints for this, verifies Unbound's
fixed address `172.29.53.2:5335` on the internal DNS network before
activation, and configures no public fallback. The only path to the
native interface is the fixed, authenticated `/adguard-ui/` route: WebApp
and Core forward it to the fixed, internally configured target, Core
supplies the internal AdGuard credentials, and mutating browser requests
must originate from the same origin. Freely chosen targets, AdGuard
credentials, and a public administration port remain excluded.

## Controlled container updates

The Stack area can only check and update the DNS services AdGuard Home
and Unbound, both fixed in Core's allowlist. Browser requests can specify
neither image names, Compose arguments, nor containers. A check pulls the
server-side configured target image and compares its actual image ID
against the running container.

Before a swap, Core copies the persistent service paths into its
protected data volume. Exactly one Compose service is then replaced and
the complete service-specific health check runs. On failure, Core pins the
previous image ID again, restores the backup, and re-verifies the rolled
back service.

Core and WebApp are only ever replaced together, as a shared control
plane, by the separate updater helper. The browser can specify neither
images, Compose services, nor arguments. The helper only knows `core` and
`webapp`, pulls the configured target images, writes a narrow Compose
override, and then verifies both actual image IDs plus Core and WebApp
health. If a check fails, both previous image IDs are pinned together and
re-verified. The helper itself stays unchanged during the process and is
not reachable from the host. WebApp sessions live in their own volume and
survive a controlled WebApp replacement.

Automatic and manual Docker cleanup share the same server-side candidate
determination. Images are only derived from successful, persistent
RootGuard update entries; the active and the previous successful image
stay protected per service. Volumes additionally require the fixed
`io.rootguard.cleanup=true` label and must not be used by any container.
The manual preview shows Docker's rounded `UniqueSize`/volume size
estimate and skipped resources. After confirmation, the selection is
fully recomputed rather than trusting a possibly stale browser preview.
Global prune commands and freely supplied resource names remain excluded.

## Backup and restore ownership for AdGuard Home

AdGuard's state lives in two separate, named Docker volumes rather than in
Core-owned paths:

- `rootguard-adguard-config` (`/opt/adguardhome/conf`) - filter lists,
  allow lists, DNS rewrites, client/DHCP settings, encryption
  configuration; everything the operator configures through the native
  AdGuard interface.
- `rootguard-adguard-work` (`/opt/adguardhome/work`) - query log and
  statistics.

Both paths are part of the same backup mechanism described above via
`BackupPaths`: before every AdGuard update, Core copies them into its
protected data volume; if the subsequent health check fails, Core
automatically restores both paths and rolls back to the previous image
ID. This backup is exclusively an internal update safeguard - not an
operator-triggered, downloadable, or directly WebApp-restorable export.
RootGuard retains the five most recent update restore points per service
by default; on the Backups page, the operator can configure this value
between 2 and 50 and see the count, storage usage, and most recent
timestamp per service. Only canonical directories uniquely matched to
RootGuard and the allowed service via a matching manifest are pruned.
Unknown files, directories, and symlinks are shown separately as
unmanaged storage and are never deleted.

For data protection beyond the plain update safeguard, the Backups page
creates a portable, password-encrypted age-v1 archive. It contains
RootGuard's Unbound/AdGuard/installation state, AdGuard's live
configuration and work data, and Unbound's runtime state. Browser
sessions, external `.env` secrets, internal update restore points, and
temporary export data remain excluded. A versioned manifest records every
regular file with its size and SHA-256 checksum. All source paths and
containers are fixed in Core; symlinks are rejected. Docker copies exist
only during creation, in a private `0700` directory in the protected Core
volume, and are removed on every success/failure path. The download is
encrypted directly via age/scrypt and blocks concurrent data-plane
updates. The guided restore fully validates the same artifact, only
accepts a clean installation with no colliding managed Docker resources,
populates newly created, stopped service volumes, and then starts the
health-checked DNS chain. A failure removes the new Docker resources and
restores previously existing local volume contents.

## Unbound configuration lifecycle

```text
WebGUI draft
  → preview and field comparison
  → unbound-checkconf in the resolver
  → atomic activation
  → resolver restart
  → versioned snapshot
```

Core retains at most 20 validated versions. A manual rollback is
re-rendered and validated just like any other change. If a restart fails
after activation, Core restores the previously read configuration and
settings files and restarts Unbound again. The WebApp gets no generic
file or command access.

The live view likewise has no generic file access. Core only reads the
fixed Unbound base and managed files from the running resolver and
delivers them read-only to the authenticated WebApp. This lets the
interface show the effective container state instead of just a
re-rendered draft.

Predefined operating profiles and the RootGuard Advisor operate
exclusively on the draft. Recommendations are deterministic, change no
files, and are checked against the same value ranges as a later activation
before being returned. This means neither profiles nor suggestions bypass
the safety chain.

Conditional forwarding is part of the typed managed config. Zones must be
canonical FQDNs; target servers must be canonical IPv4/IPv6 addresses. The
root zone, loopback, link-local, multicast, the internal RootGuard DNS
network, duplicates, and parallel expert `forward-zone` blocks are
rejected. An authenticated reachability endpoint runs only DNS SOA probes
via `dig` from the running Unbound container. Only `NOERROR` together with
an SOA record for the configured zone counts as success; `NXDOMAIN`,
`REFUSED`, transport errors, and empty successful responses remain
diagnostic results but don't clear activation. Count, concurrency, output,
and runtime are bounded; the endpoint writes no configuration. The later
activation still goes through the complete checkconf, snapshot, and
rollback cycle. DNSSEC stays enabled by default for every forwarding zone.
Only an explicit `allow_unsigned` renders a zone-specific
`domain-insecure` directive inside the `server` block. This makes trusted
unsigned split-DNS zones work without disabling global DNSSEC validation.
Correspondingly, only `allow_private_addresses` renders a zone-specific
`private-domain` directive. Unbound's rebinding protection thus stays
globally active, while explicitly trusted internal zones may return
RFC1918 and other protected private address answers.

## Unbound expert configuration

The immutable `/etc/unbound/unbound.conf` includes configuration modules
from `/etc/unbound/unbound.d/*.conf`. The expert editor only owns the file
`90-rootguard-custom.conf`; `50-rootguard.conf` stays reserved for the
typed WebGUI. Includes, listeners, remote control, container paths, trust
anchors, and guided values are locked in the free-form editor.

Before activation, Core validates a combined candidate file. Settings,
managed config, and custom config are then written atomically, and the
effective `/etc/unbound/unbound.conf` is re-verified with
`unbound-checkconf`. On a validation or restart failure, Core restores all
three previous files. A history entry therefore always represents the
combined resolver state.
