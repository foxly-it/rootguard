# Extension API design (draft)

Last reviewed: 2026-09-21

**Status: design draft, no implementation.** This is the first deliverable
under [#186](https://github.com/foxly-it/rootguard/issues/186) ("Plan
extension architecture for post-1.0") - a concrete, reviewable proposal
for the four ROADMAP.md Post-1.0/Future bullets on extensions, not a
commitment to build it on any timeline. Nothing here ships before 1.0 is
explicitly reopened for it; see that issue and ROADMAP.md's own "no
current release commitment" wording.

## 1. Why this needs a real design, not just "add a plugin system"

RootGuard's entire pre-1.0 security posture rests on one property: **a
browser request can never choose container names, images, commands,
mounts, or arbitrary configuration paths** (`ROADMAP.md`'s release rule
1). Every mutation path - Unbound settings, AdGuard bootstrap, backups,
updates - goes through RootGuard's own preview/validate/activate/
history/rollback pipeline, never a raw passthrough. `docs/threat-model.md`
spent five actor sections and two full audit cycles narrowing exactly
which operations Core, the WebApp, and the Docker socket holders are each
allowed to trigger.

A generic "extension" concept threatens all of that in one step, unless
the design explicitly forecloses it: an extension is by definition
third-party code the operator didn't get from `foxly-it/rootguard`
itself, running with *some* level of access to a system whose whole
value proposition is "we validated this appliance surface for you." The
goal of this document is to describe a shape where that stays true - an
extension can add real capability without becoming a second, ungoverned
Docker-socket-equivalent trust boundary.

## 2. Goals and non-goals

**Goals:**

- Let an extension add a guided UI surface, a background integration
  (e.g. reading a router's DHCP leases), or both.
- Keep every mutation an extension can cause routed through RootGuard's
  own existing validation/preview/activation/history/rollback machinery
  for the subsystem it touches - never a new, parallel write path.
- Make what an extension *can* do legible before install: a fixed,
  enumerable capability list, not "trust this code."
- Keep a failing or malicious extension's blast radius bounded to itself
  - it must not be able to degrade Unbound/AdGuard/the WebApp's own
    availability or correctness, and must not reach the Docker socket,
    the host filesystem, or another extension's data.
- Reuse existing RootGuard machinery wherever the shape already fits
  (Cosign/SLSA attestation, the Preview/Apply/History/Restore pattern,
  `rootguard-docker-proxy`'s allow-list-by-body-validation approach) -
  design a smaller number of new concepts, not a parallel security model.

**Non-goals (explicitly out of scope for v1 of this design):**

- A marketplace, discovery service, or any RootGuard-operated extension
  registry. Install is by explicit local reference (an image reference an
  operator supplies), the same trust model as `compose.release.yaml`
  itself.
- Extensions that need the real Docker socket, arbitrary root scripts, or
  host filesystem access. `ROADMAP.md`'s own wording already forecloses
  this ("Unrestricted Docker socket access or arbitrary root scripts are
  not the default model") - this document does not propose a "privileged
  extension" tier at all. If a future concrete extension genuinely needs
  more than the capability model below can express, that's a new ROADMAP
  discussion, not a v1 escape hatch.
- Multi-extension composition/ordering guarantees (two extensions
  affecting the same subsystem). v1 assumes extensions are evaluated for
  conflicts against RootGuard's own guided/expert configuration only, not
  against each other - see §9.
- A stable, frozen API. This is explicitly a **versioned, evolvable**
  contract (§6) - v1 can and will add capabilities later; it must not
  need a breaking change to do so.

## 3. Core principle: extensions propose, RootGuard disposes

Every existing RootGuard subsystem that accepts a change already
separates *proposing* a change from *committing* it:

| Subsystem | Propose | Commit |
|---|---|---|
| Unbound settings | `Preview()` | `Apply()` (checkconf, restart, health check, history entry) |
| Unbound custom config | `validateCombined()` preview | `Apply()` |
| AdGuard bootstrap/filtering | preflight checks | the actual API call to AdGuard |
| Backup restore | `/api/backups/restore/preview` | `/api/backups/restore` |
| Core/WebApp updates | attestation + digest resolution | `composeUp` + health verification + rollback |

An extension **never gets a new commit path**. It only ever gets to call
the *propose* side of an existing pipeline, through a new, narrow
extension-facing API that wraps the same `Preview`/`Apply` pair Core
already exposes internally - Core still runs the same validation,
still requires an authenticated operator to confirm activation (see §9
for what "confirm" means for a non-interactive extension), and still
owns the resulting history/rollback entry. This is the single most
important property of this whole design: **an extension's worst-case
output is a rejected or reverted proposal, never a direct write.**

Concretely, this means the extension API is not "give an extension a
scoped API token to `/api/unbound/settings`" - it is a *new*, narrower
API surface (`/api/extensions/v1/...`) that internally calls the exact
same `unbound.Manager.Preview`/`Apply` Core's own HTTP handlers call,
with the extension's identity and declared capabilities threaded through
for authorization and audit, and with RootGuard's own conflict-detection
logic (guided vs. expert vs. now extension-proposed) running before
any activation.

## 4. Capability model

An extension's manifest (§7) declares a subset of a fixed, versioned
capability list. Capabilities are **read** or **propose**, never
**write** - "propose" always means "submit to an existing Preview/Apply
pipeline," never a direct mutation.

| Capability | Grants | Backing API (internal) |
|---|---|---|
| `read.system` | Dashboard/system metrics, service status | existing read-only `/api/dashboard`, `/api/system`, `/api/services` equivalents |
| `read.unbound` | Current Unbound guided settings, forward zones, local hosts | existing read paths in `unbound.Manager` |
| `read.adguard` | AdGuard filtering/protection status, filter report | existing read paths in `adguard.Manager` |
| `propose.unbound.local-hosts` | Propose additions/edits to the typed host inventory (`LocalZone`/`LocalHost`) | `unbound.Manager.Preview`/`Apply` for that subsystem only |
| `propose.unbound.forward-zones` | Propose forward-zone changes | same pattern, scoped to forward zones |
| `ui.panel` | Render a UI panel into one or more declared extension points (§9) | none - UI-only, no backend access implied |
| `notify.audit` | Emit entries into RootGuard's own audit log, tagged with the extension's identity | `recordAuditDetail`-equivalent, extension-scoped event names only |

Deliberately **not modeled in v1** (would need a new ROADMAP discussion,
not silently added): `propose.unbound.custom-config` (arbitrary
directives - too close to root-script-equivalent for a first version),
any `write.*` capability, any capability touching backups, updates, or
account/session management, and any capability implying Docker access of
any kind.

A manifest requesting a capability not in this list is rejected at
install time with a clear, specific error - the list evolves by a new
RootGuard release adding a row, never by an extension unlocking
something ambient.

## 5. Permission boundaries: what is never delegated

Restated plainly, because this is the property the whole design exists
to protect (mirrors `ROADMAP.md`'s own wording verbatim):

RootGuard retains exclusive ownership of **preview, validation,
activation, history, rollback, audit, backup, and restore** for every
subsystem an extension can touch, for the whole lifetime of the
extension being installed. An extension:

- Never receives Docker socket access, directly or via a proxy scoped
  wider than its own declared capabilities.
- Never receives a root shell, arbitrary script execution on the host,
  or a bind mount outside its own extension-private volume (§8).
- Never bypasses `unbound-checkconf`, AdGuard's own API validation, or
  any other existing pre-activation check - a proposal that would fail
  today's guided-settings validation fails identically when proposed by
  an extension.
- Never activates a change without a human operator's explicit
  confirmation in the WebGUI, the same "preview, then apply" motion any
  guided settings change already requires - there is no unattended-apply
  mode in v1, even for an extension whose manifest requests it.
- Never reads another extension's data, or RootGuard's own credentials/
  tokens/session store.

## 6. Versioning and compatibility contract

- `GET /api/extensions/v1/platform` reports the platform's own capability
  list (§4) and a semantic platform version. An extension manifest
  declares a `rootguard_api: ">=1.0.0 <2.0.0"`-shaped constraint (the
  same style already used for `AdGuardChannel`/image-tag matching
  elsewhere in this codebase); Core refuses to enable an extension whose
  constraint the running platform version doesn't satisfy, with a clear
  error - never a partial/best-effort activation.
- Adding a capability or extension point is a **minor** version bump
  (backward compatible - an older extension manifest simply doesn't
  request the new capability). Removing or changing the meaning of an
  existing capability is a **major** version bump, and Core must refuse
  to enable an extension pinned below that major version rather than
  silently reinterpret its manifest.
- A RootGuard self-update that would drop support for an extension's
  declared `rootguard_api` range disables that extension (not the
  RootGuard update) and surfaces this clearly in the WebGUI before the
  update proceeds - mirrors the existing attestation/digest-pinning
  discipline of "fail loudly and safely, never silently degrade."

## 7. Extension package and manifest

An extension is an OCI image (the same distribution mechanism every
RootGuard component already uses) plus a manifest file baked into that
image at a fixed path, e.g. `/extension.json`:

```json
{
  "schema_version": "1.0.0",
  "id": "org.example.guided-access-rules",
  "display_name": "Guided access rules",
  "version": "0.3.0",
  "rootguard_api": ">=1.0.0 <2.0.0",
  "capabilities": [
    "read.unbound",
    "propose.unbound.local-hosts",
    "ui.panel",
    "notify.audit"
  ],
  "ui_panels": [
    { "extension_point": "unbound.guided.sidebar", "entry": "/ui/panel.js" }
  ],
  "signing": {
    "issuer": "https://token.actions.githubusercontent.com",
    "identity": "https://github.com/example-org/guided-access-rules/.github/workflows/release.yml@refs/heads/main"
  }
}
```

`schema_version` is the manifest schema's own version (independent of
the platform API version in §6) - a future incompatible manifest shape
change bumps this, and Core refuses to parse a manifest whose
`schema_version` it doesn't recognize rather than guessing.

## 8. Lifecycle: install, enable, disable, upgrade, remove

Every lifecycle transition is a RootGuard-initiated, RootGuard-owned
operation - an extension never self-installs, self-upgrades, or
self-removes.

1. **Install**: operator supplies an image reference in the WebGUI. Core
   pulls it, verifies the signing identity in the manifest against a real
   Cosign/Sigstore attestation (same mechanism as
   `stack.RequireAttestation` - see `docs/threat-model.md` §4 - generalized
   to accept an operator-configured trusted-issuer list instead of only
   `foxly-it`'s own GitHub Actions identity), parses and validates the
   manifest against the current platform capability list and schema
   version, and stores it in a disabled state. No code from the extension
   image runs yet.
2. **Enable**: operator reviews the manifest's declared capabilities in
   the WebGUI (a plain, readable list - "this extension can read Unbound
   settings and propose changes to local hosts, and adds a sidebar
   panel") and confirms. Core then creates the extension's own container
   - no Docker socket, no host bind mount beyond one extension-private
   named volume, on its own isolated network reachable only by Core's
   extension-gateway process (not by Unbound/AdGuard/the WebApp
   directly) - and issues it a scoped credential valid only for the
   capabilities just confirmed.
3. **Disable**: stops the extension's container, revokes its credential.
   Its data volume is retained (so re-enabling doesn't lose state);
   nothing it previously proposed-but-not-yet-activated survives (any
   pending proposal is discarded, matching the existing behavior when a
   guided-settings preview is abandoned).
4. **Upgrade**: same verification as install, against the new image; the
   new manifest's capability list is re-confirmed by the operator if it
   requests anything beyond the previously granted set (no silent
   capability escalation on upgrade).
5. **Remove**: stops the container, deletes its credential and its data
   volume. This is the one destructive step in the lifecycle and gets the
   same "type the extension's name to confirm" pattern RootGuard's other
   irreversible actions already use.
6. **Failure isolation**: the extension's container is not a dependency
   of any RootGuard health chain - `docker compose`'s own
   `depends_on`/`service_healthy` graph never includes it, and a crashing
   or hung extension container degrades only its own UI panels (rendered
   as a clear "extension unavailable" state) and its own proposal
   capability, never Unbound/AdGuard/the WebApp's own availability. A
   resource-exhausting extension is bounded by ordinary container
   resource limits (`compose`'s `deploy.resources.limits`), applied by
   Core at container-create time, not left to the extension's own good
   behavior.

## 9. UI extension points

A `ui_panel` entry names one of a fixed, versioned set of extension
points the WebGUI frontend exposes (e.g. `unbound.guided.sidebar`,
`dashboard.widget`) - not an arbitrary DOM injection. The referenced
`entry` script runs in a sandboxed iframe served from the extension's own
container (through Core's extension gateway, never a direct route),
communicating with the parent WebGUI only through a narrow, typed
`postMessage` bridge that maps 1:1 onto the extension's declared
capabilities (a `ui.panel` scoped to `read.unbound`/
`propose.unbound.local-hosts` can request those two things through the
bridge and nothing else - the bridge itself is the enforcement point, not
just the manifest declaration). This keeps a compromised or buggy
extension panel from reading the WebGUI's own session, other panels'
data, or calling any RootGuard API the bridge doesn't explicitly proxy.

Conflict detection (`ROADMAP.md`'s own acceptance criterion for the
reference use case) happens on the *backend* proposal, not in the UI:
when an extension calls `propose.unbound.local-hosts`, the same
`unbound.Manager.Preview` pipeline that already checks a guided change
against expert-config conflicts runs identically against an
extension-proposed one, before the operator ever sees an activation
prompt.

## 10. Reference use case: a guided access-rules extension

`ROADMAP.md` names this as the concrete first-party candidate once the
platform contract is stable. Walking it through this design end to end:

- Manifest requests `read.unbound`, `propose.unbound.forward-zones`,
  `ui.panel` (an `unbound.guided.sidebar` panel for building
  per-device/per-network access rules visually).
- The panel reads current forward zones/guided settings via the bridge
  (read-only), lets the operator compose a rule visually, and on
  "propose" calls the extension's own backend (in its own container),
  which then calls RootGuard's `propose.unbound.forward-zones` API - not
  the WebGUI's own internal API directly.
- Core runs the identical `Preview`/conflict-check pipeline guided
  forward-zone edits already go through - a rule that conflicts with an
  existing guided or expert forward zone is rejected with the same
  specific error a human editing the guided UI directly would see.
- Operator reviews the preview in the WebGUI (indistinguishable in
  strictness from a native guided-settings preview) and confirms; Core's
  own `Apply` runs `unbound-checkconf`, restarts, health-checks, and
  records history - exactly as it does today, with no code path aware
  the change originated from an extension except the audit trail
  attribution.
- If the extension is later disabled or removed, the *already-applied*
  Unbound configuration it proposed stays (it's now indistinguishable
  from any other guided-settings history entry) - only its ability to
  propose *further* changes goes away. This avoids a removed extension
  silently reverting configuration the operator actively wanted to keep.

## 11. Open questions (explicitly not decided by this document)

- **Extension-to-extension interaction**: if two enabled extensions both
  declare `propose.unbound.forward-zones`, what does the WebGUI show as
  "the" pending proposal for that subsystem? Needs its own design once a
  second real extension exists to reason about concretely - speculative
  before that.
- **Metering/resource quotas** beyond a flat container resource limit
  (e.g. rate-limiting how often an extension can call `propose.*`) - not
  designed here; `rootguard-webapp`'s existing per-session destructive-
  action rate limiter is a plausible model to extend, not yet worked out
  for a non-session, extension-scoped caller.
- **Air-gapped/no-internet installs**: §8's signing verification assumes
  Sigstore/Cosign's transparency-log lookup is reachable, the same
  assumption RootGuard's own release attestation already makes
  (`rootguard-attestation-proxy` exists specifically to give Core/the
  Updater that path). An operator-only, no-attestation "developer mode"
  for local extension development is plausible but deliberately
  undesigned here - it's a real gap in the trust model if done carelessly
  (see `docs/threat-model.md`'s own treatment of
  `ROOTGUARD_SKIP_ATTESTATION`).
- **Exact scoped-credential mechanism** for an extension's backend
  calling `propose.*` (a bearer token? mTLS between Core's extension
  gateway and the extension's own container, both on an isolated
  network?) - sketched as "a scoped credential" in §8 deliberately
  loosely; this is an implementation detail once someone is actually
  building the extension gateway, not a design-level decision.

## 12. Mapping back to ROADMAP.md's four bullets

| ROADMAP bullet | Covered by |
|---|---|
| Versioned extension API, compatibility contract, capability model, explicit permission boundaries | §4, §5, §6 (this document, in full) |
| Constrained integration/configuration/UI extension points; RootGuard retains ownership of preview/validation/activation/history/rollback/audit/backup/restore | §3, §8, §9 |
| Signing or explicit trust handling; safe install/disable/upgrade/failure-isolation/removal | §8 |
| Official reference extension after the platform contract is stable | §10 (walked through against this design; not built) |
