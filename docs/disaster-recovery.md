# Disaster recovery runbook

A step-by-step guide for recovering a RootGuard appliance when something has
gone seriously wrong - not routine operations. For everyday update/rollback
and backup/restore usage, use the WebGUI directly; this document exists for
the moments the WebGUI itself is unreachable or the host it ran on is gone.

Each scenario below maps to a specific, already-existing RootGuard recovery
mechanism. This document does not introduce new recovery capability; it is
the checklist for finding and using the right one under pressure.

The encrypted full backup/restore feature this document leans on most
(`docs/backup-export.md`) is included starting with `v0.1.0-beta.1`, the
current public release.

## Before you need this

A disaster recovery only works if it was prepared before the disaster:

- Take a portable encrypted backup from the Backups page regularly, and
  store it (and its passphrase, separately) somewhere that survives losing
  the RootGuard host itself. See [backup-export.md](backup-export.md) for
  what the archive contains and how restore works.
- Record the installation's recovery key
  (`ROOTGUARD_RECOVERY_TOKEN` in the deployment's `.env`) somewhere safe.
  Without it, a lost administrator password has no recovery path short of
  redeploying.
- Know where the deployment's `.env` and Compose files actually live on the
  host (the guided AIO installer's default is
  `/var/lib/rootguard/installation`, overridable via
  `ROOTGUARD_INSTALLATION_DIR`) - the WebGUI cannot help if Core itself is
  down.

## Scenario: total host loss

The host, VM, or LXC RootGuard ran on is gone or its disk is unrecoverable.

1. Provision a clean replacement host meeting the same
   [platform requirements](platform-support.md).
2. Install RootGuard fresh, following the normal guided setup, but stop
   before completing first-time AdGuard bootstrap - the restore below
   requires a clean, not-yet-installed target.
3. Open the Backups page, choose restore, and upload the most recent
   encrypted archive plus its passphrase. Preview first; it reports the
   archived configuration and re-runs the clean-target checks without
   changing anything.
4. If the replacement host's DNS bind address or port differs from the
   original, change it in the restore form and preview again before
   applying.
5. Apply and confirm. RootGuard creates the stack in stopped state, restores
   local, AdGuard, and Unbound data, starts everything, and verifies the
   protected AdGuard-to-Unbound path itself - a successful apply means DNS
   is already working, not just that files were copied.
6. Point routers/clients at the replacement host's address once verified.

Full detail, exact included/excluded data, and the failure-cleanup behavior:
[backup-export.md](backup-export.md).

## Scenario: a failed update did not roll back cleanly

RootGuard's own update path (Stack Center for AdGuard/Unbound, the
control-plane updater for Core/WebApp) already attempts an automatic
rollback on a failed health check, and refuses to restore a pre-update
snapshot that fails its own checksum verification rather than applying
corrupted data - see the update history entries this produces before doing
anything by hand.

1. Open Stack Center's update history. `rolled_back` means the automatic
   rollback already ran and reported success; check the affected service is
   actually healthy now before assuming otherwise.
2. A `failed` outcome (update *and* rollback both failed, or rollback was
   refused because the pre-update snapshot didn't check out) needs manual
   intervention:
   - If Core/WebApp are still reachable, retry the update from the Stack
     Center UI once the underlying cause (registry, disk space, a stuck
     port) is fixed - a retried update always takes a fresh snapshot first.
   - If Core is unreachable (the control-plane update itself is the failure),
     SSH to the host and run `docker compose` directly against the
     installation's Compose files
     (`ROOTGUARD_INSTALLATION_DIR`, default
     `/var/lib/rootguard/installation`) to bring Core/WebApp back to a known
     image tag, then continue from the WebGUI.
3. If data (not just the image) needs restoring and no automatic rollback
   snapshot is usable, fall back to the most recent full backup - see the
   total host loss scenario above; a full restore also works in place of a
   from-scratch installation.

## Scenario: lost administrator credentials

1. Confirm the deployment actually has a recovery key configured -
   `ROOTGUARD_RECOVERY_TOKEN` present in the host's `.env`. Without one, no
   recovery path exists short of redeploying and restoring from backup.
2. Open the WebGUI login page and choose the recovery option (only shown
   when a recovery key is configured). Enter the recovery key and a new
   administrator password.
3. A successful reset invalidates every existing session - expect to be
   asked to sign in again everywhere, including other browsers/devices.
4. If the recovery key itself has been lost too, there is no in-app path
   left; redeploy and restore from the most recent full backup instead.

## Scenario: a deployment or update looks stuck after a crash

RootGuard detects an interrupted deployment or update automatically the next
time Core starts - a status left in an in-progress state after an unclean
shutdown (power loss, OOM kill, a forcibly stopped container) is reported as
a failed, retryable operation instead of silently staying "in progress"
forever.

1. Restart Core (or the whole host) if it isn't already back up.
2. Check the Setup or Stack Center page for an "interrupted" diagnostic. This
   is the expected, safe outcome of a crash mid-operation - not a sign of
   corruption by itself.
3. Simply retry the same operation (deployment or update) from the WebGUI.
   RootGuard reuses persisted configuration safely and redrives every step,
   including ones the interrupted attempt never reached.
4. If a retry itself fails, treat it as the corresponding scenario above
   (failed update, or fall back to a full restore).

## Scenario: DNS resolution stopped working

Triage before reaching for a full restore - most DNS incidents are not disk
loss:

1. Dashboard: is the protected upstream (AdGuard -> Unbound) shown healthy?
2. Stack Center: are all five managed services running and healthy?
3. Unbound Overview's live diagnostics and "Path diagnostics" card: recursive
   resolution and DNSSEC checks, plus the AdGuard-to-Unbound chain
   specifically.
4. Logs & Diagnostics: bounded, redacted logs per service if the above
   doesn't explain it.
5. Only after the above: consider whether a recent change (a guided Unbound
   setting, an expert config edit, an update) is the cause, and use Unbound's
   own version history/rollback before considering a full restore.

## Verification

The total-host-loss scenario above was drilled against a real second host,
distinct from the host that produced the backup - not just a code-level test
of the restore feature (already covered separately, see
`docs/backup-export.md`). A passphrase-encrypted backup was exported from a
live RootGuard installation, transferred to an entirely separate,
freshly provisioned Debian 13 host with no prior RootGuard state, and
restored there through the same `POST /api/backups/restore/preview` /
`POST /api/backups/restore` calls the Backups page itself makes. The restore
reached `installed` after all seven steps completed, and the replacement
host's own resolver was then verified independently: a recursive query
resolved with the `ad` (DNSSEC-authenticated) flag set, and a deliberately
broken DNSSEC chain (`dnssec-failed.org`) was correctly rejected with
`SERVFAIL` - proof the restored AdGuard-to-Unbound chain works, not only
that files were copied. Last drilled 2026-08-13.
