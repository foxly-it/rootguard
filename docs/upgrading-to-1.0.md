# Upgrading to 1.0.0, and rolling back

Closes `ROADMAP.md`'s last open `0.9` item: versioned migration and
rollback instructions for the `1.0.0` transition specifically. The
underlying mechanisms (self-update, automatic post-swap rollback, backup
restore) are not new here - see
[compatibility-matrix.md](compatibility-matrix.md) and
[disaster-recovery.md](disaster-recovery.md) for how they work in
general; this page only covers what's specific to landing on `1.0.0`.

## Upgrading

- **From `1.0.0-rc.2` or later**: no special steps. Use the normal
  Stack & Updates page (or the control-plane update for Core/WebApp) -
  `1.0.0` ships no new service, network, or env var beyond what
  `rc.2` already introduced.
- **From `1.0.0-rc.1`**: that release predates `rootguard-attestation-proxy`
  and the `egress` network. Self-update can never deliver a
  compose-topology change to an existing installation (see
  [release-process.md](release-process.md), "Self-update can never
  deliver a compose-topology change") - a fresh install or a manual
  `compose.release.yaml` refresh is required first. Attempting a normal
  update without it fails with a specific, actionable error
  (`CheckAttestationProxyReachable`), not a silent hang.
- **From `0.1.0-beta.14` or earlier**: outside the N-1 → N compatibility
  scope (see compatibility-matrix.md) and predates automatic release
  discovery's current version scheme. Needs the one-time manual
  `target_images` pointer already documented on the public docs page's
  Updates section before it can discover `1.0.0` at all, then upgrade
  through each release in sequence from there - skipping releases isn't
  tested or supported.

## Rolling back

Two different mechanisms cover different situations - neither is new,
but which applies depends on when the problem shows up:

- **The update itself fails its post-swap health check**: already
  automatic, no operator action needed. Core/the Updater pin the
  previous image ID(s) and restore the pre-update backup on failure -
  covered in compatibility-matrix.md and the public docs page.
- **`1.0.0` runs, but you want to go back after the fact**: this is
  where Core/WebApp and the other managed services differ.
  - **Core/WebApp**: the control-plane updater explicitly refuses to
    install a version older than what's currently running
    (`rootguard-updater/manager.go`'s `isOlderReleaseVersion` check) -
    a deliberate safety feature against downgrade attacks, not a bug to
    work around. There is no supported "click to downgrade" once an
    update has already succeeded. The only path back is a full restore
    from a backup taken before upgrading, onto a freshly (re)installed
    `1.0.0-rc.4` instance - see disaster-recovery.md's "total host
    loss" procedure; the steps are identical, just deliberate rather
    than forced by hardware failure.
  - **AdGuard/Unbound/Blockpage**: Core's own update manager for these
    three has no such version check and no per-request override either
    - the target image is whatever `ROOTGUARD_*_UPDATE_IMAGE` is set to
    in `.env`. Rolling one back means editing that value to an older
    digest and restarting Core, then triggering a normal update from
    the Stack page. Doing this alone does not roll back Core/WebApp -
    since RootGuard only promises N-1 → N compatibility, running a
    deliberately mismatched combination this way is unsupported and
    untested; treat it as a stopgap on the way to a full restore, not a
    steady state.
  - **Restoring a `1.0.0` backup onto `1.0.0-rc.4`**: restore requires
    the archive's backup-format schema version to exactly match the
    target installation's (`rootguard-core/internal/backuprestore`,
    `SchemaVersion`) - not a version-number check. This happens to work
    between `rc.4` and `1.0.0` today since neither changed the schema,
    but it's an implementation detail, not a cross-version guarantee:
    always run Preview first (it reports the archive's schema version
    and refuses cleanly on a mismatch) before relying on it.
