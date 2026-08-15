#!/usr/bin/env bash
# 0.9 endurance test: periodic backup/restore drill.
#
# RootGuard is a deliberately single-node appliance: rootguard-adguard,
# rootguard-unbound, rootguard-blockpage, the rootguard-dns network, and
# their volumes are hardcoded literal names inside Core itself (see
# rootguard-core/internal/installer/manager.go), not parameterized by
# compose project - a second install cannot run side by side with the
# first on the same host. Restore is documented as "a clean replacement
# installation, not an in-place import" for the same reason (Core refuses
# ErrNotClean otherwise). So this drill is a REAL disaster-recovery
# exercise against the primary instance itself, not a disposable side
# instance: export -> tear the primary down -> fresh install -> restore
# -> verify DNS. If restore itself doesn't come up cleanly, it falls back
# to a normal fresh (non-restore) install so the 30-day DNS-continuity
# measurement can keep going rather than staying down for the rest of the
# window - that fallback is itself logged as the incident it is.

set -Eeuo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
. ./common.sh
soak_acquire_mutation_lock

ts="$(soak_now)"
archive="${ROOTGUARD_SOAK_DIR}/drill-backup.age"
passphrase="soak-drill-$(openssl rand -hex 16)"
export_ok=false
restore_ok=false
fallback_used=false
post_restore_dns_ok=false

managed_containers=(rootguard-webapp rootguard-core rootguard-updater rootguard-adguard rootguard-unbound rootguard-blockpage)
managed_volumes=(rootguard-data rootguard-sessions rootguard-unbound-config rootguard-unbound-state rootguard-adguard-work rootguard-adguard-config)
managed_networks=(rootguard-dns rootguard_control rootguard_edge)

teardown_primary() {
  docker rm -f "${managed_containers[@]}" >/dev/null 2>&1 || true
  docker compose -f "${ROOTGUARD_SOAK_DIR}/compose.alpha.yaml" down --volumes --remove-orphans >/dev/null 2>&1 || true
  for n in "${managed_networks[@]}"; do docker network rm "$n" >/dev/null 2>&1 || true; done
  docker volume rm "${managed_volumes[@]}" >/dev/null 2>&1 || true
}

wait_installed() {
  local budget="$1" state=""
  for _ in $(seq 1 "$budget"); do
    state="$(soak_call GET /api/installation 2>/dev/null | jq -r .state 2>/dev/null || true)"
    [ "$state" = installed ] && return 0
    [ "$state" = failed ] && return 1
    sleep 3
  done
  return 1
}

deploy_config='{"dns_bind_address":"0.0.0.0","dns_port":53}'

fresh_install() {
  docker compose -f "${ROOTGUARD_SOAK_DIR}/compose.alpha.yaml" up -d
  soak_login
  soak_call POST /api/installation/preflight "$deploy_config" >/dev/null
  soak_call POST /api/installation/deploy "$deploy_config" >/dev/null
  wait_installed 90
}

rm -f "$archive"
if soak_login && curl --fail --silent \
    --cookie "$ROOTGUARD_SOAK_COOKIE_JAR" \
    --header 'Content-Type: application/json' \
    --data "{\"passphrase\":\"${passphrase}\"}" \
    -o "$archive" \
    "http://127.0.0.1:${ROOTGUARD_SOAK_WEB_PORT}/api/backups/export"; then
  [ -s "$archive" ] && export_ok=true
fi

if [ "$export_ok" = true ]; then
  teardown_primary
  docker compose -f "${ROOTGUARD_SOAK_DIR}/compose.alpha.yaml" up -d
  if soak_login; then
    if curl --fail --silent \
        --cookie "$ROOTGUARD_SOAK_COOKIE_JAR" \
        --form "passphrase=${passphrase}" \
        --form "archive=@${archive};type=application/vnd.rootguard.backup+age" \
        --form "config=${deploy_config}" \
        --form "confirmation=RESTORE" \
        "http://127.0.0.1:${ROOTGUARD_SOAK_WEB_PORT}/api/backups/restore" >/dev/null \
      && wait_installed 90; then
      restore_ok=true
    fi
  fi
fi

if [ "$restore_ok" != true ]; then
  fallback_used=true
  fresh_install || true
fi

if soak_resolve_ok >/dev/null && soak_dnssec_reject_ok; then
  post_restore_dns_ok=true
fi

rm -f "$archive"

line="$(jq -nc \
  --arg ts "$ts" \
  --argjson export_ok "$export_ok" \
  --argjson restore_ok "$restore_ok" \
  --argjson fallback_used "$fallback_used" \
  --argjson post_restore_dns_ok "$post_restore_dns_ok" \
  '{ts:$ts, export_ok:$export_ok, restore_ok:$restore_ok, fallback_used:$fallback_used, post_restore_dns_ok:$post_restore_dns_ok}')"
soak_log backup-restore.jsonl "$line"

if [ "$post_restore_dns_ok" != true ]; then
  {
    echo "=== backup-restore-drill failure ${ts} ==="
    echo "$line"
    for c in rootguard-core rootguard-webapp; do
      echo "--- ${c} ---"
      docker logs --tail 50 "$c" 2>&1 || true
    done
  } >> "${ROOTGUARD_SOAK_LOG_DIR}/failures.log"
  exit 1
fi
