#!/usr/bin/env bash
# 0.9 endurance test: continuous DNS/filtering/WebGUI health probe.
# Runs unattended on a systemd timer (see README.md in this directory).
# Appends one JSON line to probe.jsonl per run; never alerts anyone by
# design (see the RC plan) - a failing probe is only visible in the log
# and, on failure, a bounded log snapshot in failures.log.

set -Eeuo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
. ./common.sh

ts="$(soak_now)"
t0="$(date +%s%N)"

resolve_answer="$(soak_resolve_ok || true)"
resolve_ok=false
[ -n "$resolve_answer" ] && resolve_ok=true

dnssec_reject_ok=false
soak_dnssec_reject_ok && dnssec_reject_ok=true

# doubleclick.net is carried by AdGuard's default blocklists. Read the
# actual status line, not just "is the answer empty" - an empty answer can
# also mean a timeout or SERVFAIL, neither of which is a successful block.
# AdGuard's block response can be NXDOMAIN or an explicit all-zero address
# depending on version/blocking_mode; either counts as "blocked".
block_response="$(dig +time=5 +tries=2 @127.0.0.1 -p "$ROOTGUARD_SOAK_DNS_PORT" doubleclick.net A 2>/dev/null || true)"
block_status="$(sed -n 's/^;; ->>HEADER<<- opcode: QUERY, status: \([^,]*\).*/\1/p' <<<"$block_response")"
block_answer="$(awk '/^;; ANSWER SECTION:/{getline; print $NF; exit}' <<<"$block_response")"
block_ok=false
if [ "$block_status" = "NXDOMAIN" ] || [ "$block_answer" = "0.0.0.0" ] || [ "$block_answer" = "::" ]; then
  block_ok=true
fi

api_ok=false
if soak_login && soak_call GET /api/dashboard >/dev/null 2>&1; then
  api_ok=true
fi

t1="$(date +%s%N)"
latency_ms=$(( (t1 - t0) / 1000000 ))

# docker stats can legitimately fail for one container mid-recreate (e.g. a
# probe landing during a backup-restore-drill or update-exercise run) - with
# `set -e` and `pipefail` active, that fails the whole pipeline, and since
# this assignment isn't itself part of a conditional, `set -e` aborts the
# *entire script* right here, before anything below it (including the
# validation on the next line) ever runs. The `|| mem_json='[]'` on the
# assignment itself is what actually keeps the script alive across that
# failure - the follow-up jq -e check only guards against the pipeline
# succeeding but producing something unexpected, not against it failing.
mem_json="$(docker stats --no-stream --format '{{json .}}' \
  rootguard-core rootguard-webapp rootguard-updater rootguard-adguard rootguard-unbound rootguard-attestation-proxy 2>/dev/null \
  | jq -cs '[.[] | {name: .Name, mem: .MemUsage, cpu: .CPUPerc}]' 2>/dev/null)" || mem_json='[]'
jq -e . >/dev/null 2>&1 <<<"$mem_json" || mem_json='[]'

line="$(jq -nc \
  --arg ts "$ts" \
  --argjson resolve_ok "$resolve_ok" \
  --argjson dnssec_reject_ok "$dnssec_reject_ok" \
  --argjson block_ok "$block_ok" \
  --argjson api_ok "$api_ok" \
  --arg resolve_answer "$resolve_answer" \
  --argjson latency_ms "$latency_ms" \
  --argjson containers "$mem_json" \
  '{ts:$ts, resolve_ok:$resolve_ok, dnssec_reject_ok:$dnssec_reject_ok, block_ok:$block_ok,
    api_ok:$api_ok, resolve_answer:$resolve_answer, latency_ms:$latency_ms, containers:$containers}')"
soak_log probe.jsonl "$line"

if [ "$resolve_ok" != true ] || [ "$dnssec_reject_ok" != true ] || [ "$block_ok" != true ] || [ "$api_ok" != true ]; then
  {
    echo "=== probe failure snapshot ${ts} ==="
    echo "$line"
    for c in rootguard-core rootguard-unbound rootguard-adguard; do
      echo "--- ${c} ---"
      docker logs --tail 50 "$c" 2>&1 || true
    done
  } >> "${ROOTGUARD_SOAK_LOG_DIR}/failures.log"
fi
