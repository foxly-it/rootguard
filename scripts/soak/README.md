# 0.9 endurance-test harness

Unattended scripts for `ROADMAP.md`'s 0.9 "Thirty-day continuous DNS test
with update and restore exercises" item. Not part of CI - these run on a
dedicated, long-lived host against a real `v0.1.0-*` release deployed from
`compose.alpha.yaml`, driven by systemd timers, not by any CI workflow or
Claude session.

- `common.sh` - shared login/call/log helpers, sourced by the others.
  Also provides `soak_acquire_mutation_lock`, used by the two mutating
  exercises below (not `probe.sh`, which is read-only and already
  tolerates the brief restart window either one causes) so an
  update-exercise and a backup-restore-drill overlapping in time can't
  fight over the same containers and cookie jar.
- `probe.sh` - every ~10 min: DNS resolution, DNSSEC rejection, AdGuard
  filtering, WebGUI liveness, and per-container `docker stats` (doubles as
  the "small network" performance/memory baseline data). Logs to
  `probe.jsonl`.
- `update-exercise.sh` - every ~5 days: drives a full
  check/poll/install/poll/assert cycle against `/api/control-plane-updates`
  (Core+WebApp), same pattern as `release-alpha.yml`'s `upgrade-test` job.
  Logs to `update.jsonl`.
- `backup-restore-drill.sh` - every ~7 days: a **real** disaster-recovery
  exercise against the primary instance (export -> teardown -> fresh
  install -> restore -> verify DNS), not a side-by-side secondary - RootGuard
  hardcodes `rootguard-adguard`/`rootguard-unbound`/`rootguard-blockpage`/
  the `rootguard-dns` network at the Core level (see
  `rootguard-core/internal/installer/manager.go`), so a second install
  cannot coexist with the first on one host. Falls back to a plain fresh
  install if restore itself doesn't come up cleanly, so the 30-day
  DNS-continuity measurement survives a bad restore rather than staying
  down for the rest of the window - that fallback is logged as the
  incident it is. Logs to `backup-restore.jsonl`.
- `report.sh` - rolls all three logs up into a human-readable summary,
  appended to `daily-report.log`. Run daily via timer and once by hand at
  day 30 for the `ROADMAP.md` evidence line.

## Deploying to a soak host

1. `mkdir -p /root/rootguard-soak && cd /root/rootguard-soak`
2. `curl -LO https://raw.githubusercontent.com/foxly-it/rootguard/<tag>/compose.alpha.yaml`
   and `.env.alpha.example` -> `.env`, fill in real secrets (see the main
   README's Quick Start).
3. `docker compose -f compose.alpha.yaml up -d`, then complete guided setup
   once via `/api/auth/login` -> `/api/installation/preflight` ->
   `/api/installation/deploy` (see `scripts/verify-clean-install.sh` for the
   exact call shape).
4. Copy this `scripts/soak/` directory next to `compose.alpha.yaml` on the
   host (e.g. `/root/rootguard-soak/scripts/soak/`).
5. Run each script manually once to confirm real output before arming any
   timer - a probe that can't fail loud is worse than no probe.
6. Install systemd service+timer units for each script (probe every 10 min,
   update-exercise every 5 days, backup-restore-drill every 7 days, report
   daily) and `systemctl daemon-reload && systemctl enable --now
   rootguard-soak-*.timer`.

All state lives under `/var/log/rootguard-soak/*.jsonl` on the soak host
itself - nothing here depends on a Claude session or GitHub Actions to keep
running.
