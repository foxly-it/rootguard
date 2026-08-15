#!/usr/bin/env bash
# 0.9 endurance test: periodic update-mechanism exercise. Drives a full
# check -> poll -> install -> poll -> assert cycle against the real
# control-plane updater (Core+WebApp), the same call()/poll_status()
# pattern release-alpha.yml's own upgrade-test job already proves works.
# If no newer version is published this is a same-digest no-op cycle - it
# still proves the check/poll/history path and the updater's own
# long-running health don't degrade over time. Also does a cheap
# status-only probe of the AdGuard/Unbound update path (no install) so
# that surface stays exercised too, without the added risk of repeatedly
# swapping AdGuard/Unbound underneath a 30-day DNS-continuity measurement.

set -Eeuo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
. ./common.sh
soak_acquire_mutation_lock

ts="$(soak_now)"
outcome="login_failed"
before_core="$(docker inspect rootguard-core --format '{{.Image}}' 2>/dev/null || true)"
before_webapp="$(docker inspect rootguard-webapp --format '{{.Image}}' 2>/dev/null || true)"

poll_idle() {
  local budget="$1" status state
  for _ in $(seq 1 "$budget"); do
    status="$(soak_call GET /api/control-plane-updates 2>/dev/null || true)"
    if [ -n "$status" ]; then
      state="$(jq -r .state <<<"$status")"
      [ "$state" = idle ] && { printf '%s' "$status"; return 0; }
      [ "$state" = failed ] && { printf '%s' "$status"; return 1; }
    fi
    sleep 2
  done
  return 1
}

if soak_login; then
  if soak_call POST /api/control-plane-updates/check >/dev/null 2>&1 && status="$(poll_idle 60)"; then
    if soak_call POST /api/control-plane-updates/install >/dev/null 2>&1 && status="$(poll_idle 90)"; then
      outcome="$(jq -r '.history[0].outcome // "unknown"' <<<"$status")"
    else
      outcome="install_timed_out_or_failed"
    fi
  else
    outcome="check_timed_out_or_failed"
  fi
  # Status-only AdGuard/Unbound update-path probe - no install triggered.
  soak_call GET /api/updates >/dev/null 2>&1 || true
  soak_call POST /api/updates/check >/dev/null 2>&1 || true
fi

after_core="$(docker inspect rootguard-core --format '{{.Image}}' 2>/dev/null || true)"
after_webapp="$(docker inspect rootguard-webapp --format '{{.Image}}' 2>/dev/null || true)"

line="$(jq -nc \
  --arg ts "$ts" --arg outcome "$outcome" \
  --arg before_core "$before_core" --arg after_core "$after_core" \
  --arg before_webapp "$before_webapp" --arg after_webapp "$after_webapp" \
  '{ts:$ts, outcome:$outcome, before_core:$before_core, after_core:$after_core,
    before_webapp:$before_webapp, after_webapp:$after_webapp}')"
soak_log update.jsonl "$line"

case "$outcome" in
  success|no_change) exit 0 ;;
  *)
    {
      echo "=== update-exercise failure ${ts} ==="
      echo "$line"
    } >> "${ROOTGUARD_SOAK_LOG_DIR}/failures.log"
    exit 1
    ;;
esac
