#!/usr/bin/env bash
# 0.9 endurance test: rollup report. Run daily via timer for a running
# summary, and once by hand at day 30 for the final ROADMAP.md evidence.
# Reads the three JSONL logs and prints a human-readable summary to stdout;
# writes the same summary to daily-report.log for a running history.

set -Eeuo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
. ./common.sh

probe_log="${ROOTGUARD_SOAK_LOG_DIR}/probe.jsonl"
update_log="${ROOTGUARD_SOAK_LOG_DIR}/update.jsonl"
backup_log="${ROOTGUARD_SOAK_LOG_DIR}/backup-restore.jsonl"

summarize_probes() {
  [ -f "$probe_log" ] || { echo "no probes recorded yet"; return; }
  jq -s '
    length as $total
    | ([.[] | select(.resolve_ok and .dnssec_reject_ok and .block_ok and .api_ok)] | length) as $pass
    | {
        total: $total,
        pass: $pass,
        pass_rate: (if $total > 0 then ($pass / $total * 100 | round) else 0 end),
        first_ts: (.[0].ts // null),
        last_ts: (.[-1].ts // null),
        incidents: [.[] | select(.resolve_ok != true or .dnssec_reject_ok != true or .block_ok != true or .api_ok != true) | .ts]
      }' "$probe_log"
}

summarize_updates() {
  [ -f "$update_log" ] || { echo "no update exercises recorded yet"; return; }
  jq -s '
    length as $total
    | {
        total: $total,
        by_outcome: (group_by(.outcome) | map({(.[0].outcome): length}) | add),
        entries: [.[] | {ts, outcome, before_core, after_core}]
      }' "$update_log"
}

summarize_backups() {
  [ -f "$backup_log" ] || { echo "no backup/restore drills recorded yet"; return; }
  jq -s '
    length as $total
    | ([.[] | select(.restore_ok and (.fallback_used | not))] | length) as $clean_restores
    | ([.[] | select(.fallback_used)] | length) as $fallbacks
    | {
        total: $total,
        clean_restores: $clean_restores,
        fallbacks_used: $fallbacks,
        all_post_restore_dns_ok: (all(.[]; .post_restore_dns_ok))
      }' "$backup_log"
}

{
  echo "RootGuard 0.9 endurance test report - $(soak_now)"
  echo
  echo "## DNS/filtering/WebGUI probes"
  summarize_probes
  echo
  echo "## Update exercises"
  summarize_updates
  echo
  echo "## Backup/restore drills"
  summarize_backups
} | tee -a "${ROOTGUARD_SOAK_LOG_DIR}/daily-report.log"
