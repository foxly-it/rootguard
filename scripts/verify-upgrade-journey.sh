#!/usr/bin/env bash
# Upgrades a published RootGuard release to the latest published one the way
# an operator does it: install the old release exactly as it shipped, then
# take every update the WebGUI offers through its own API - Core and WebApp,
# the Updater, the Attestation Proxy, then every DNS service - with all
# Docker access going through the release's real rootguard-docker-proxy.
#
# Why this exists (found 2026-11-11, #809): every Unbound update through the
# docker-proxy had failed since September, and no test noticed. The
# integration stack runs without the proxy, and release-alpha.yml's upgrade
# test only swaps Core and WebApp. This also covers skipping versions: the
# WebGUI always offers the newest release, so ROOTGUARD_UPGRADE_FROM can lie
# several releases back.
#
# Tests published images only, not the checkout - a regression shows up here
# once it is released. Needs internet access (GitHub release discovery and
# cosign attestation checks, same as a real installation).
#
# Usage: ROOTGUARD_UPGRADE_FROM=1.0.10 ./scripts/verify-upgrade-journey.sh
# Optional: ROOTGUARD_UPGRADE_TO (defaults to the newest release tag).

set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
from_version="${ROOTGUARD_UPGRADE_FROM:?set ROOTGUARD_UPGRADE_FROM to the release to start from, e.g. 1.0.10}"
web_port="${ROOTGUARD_TEST_WEB_PORT:-18083}"
dns_port="${ROOTGUARD_TEST_DNS_PORT:-1056}"
expected_arch="${ROOTGUARD_TEST_ARCH:-}"
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/rootguard-upgrade-journey.XXXXXX")"
compose_file="${work_dir}/compose.release.yaml"
cookie_file="${work_dir}/cookies.txt"

# shellcheck source=verification-common.sh
. "${repository_dir}/scripts/verification-common.sh"
# shellcheck source=version-pattern.sh
. "${repository_dir}/scripts/version-pattern.sh"

require_commands curl dig docker git jq

to_version="${ROOTGUARD_UPGRADE_TO:-$(cd "${repository_dir}" && rootguard_current_version)}"
if [[ "${from_version}" == "${to_version}" ]]; then
  echo "Nothing to upgrade: ${from_version} is already the target release" >&2
  exit 1
fi

trap cleanup EXIT
guard_no_existing_resources

# The release tag points at its own pin commit (release-alpha.yml, "Point the
# release tag at the pin-update commit"), so the files there are exactly what
# that release shipped. Fail loudly if that ever stops being true rather than
# silently testing some other version.
git -C "${repository_dir}" show "v${from_version}:compose.release.yaml" >"${compose_file}"
if ! grep -q "rootguard-core:${from_version}@sha256:" "${compose_file}"; then
  echo "v${from_version}:compose.release.yaml does not pin rootguard-core:${from_version}" >&2
  exit 1
fi
target_compose="$(git -C "${repository_dir}" show "v${to_version}:compose.release.yaml")"

normalized_arch="$(detect_arch)"
export ROOTGUARD_API_TOKEN="upgrade-journey-api-token-not-a-real-secret-${normalized_arch}"
export ROOTGUARD_ADMIN_USER="admin"
export ROOTGUARD_ADMIN_PASSWORD="upgrade-journey-password-${normalized_arch}"
export ROOTGUARD_RECOVERY_TOKEN="upgrade-journey-recovery-token-${normalized_arch}"
export ROOTGUARD_WEB_BIND="127.0.0.1"
export ROOTGUARD_WEB_PORT="${web_port}"

base="http://127.0.0.1:${web_port}"

# compose.release.yaml mounts itself via ${PWD}/compose.release.yaml, so
# Compose has to run from the directory that holds it - here and in every
# update the Updater and Core run against it later.
cd "${work_dir}"

echo "== Installing ${from_version} as it shipped"
install_stack
wire_local_dnssec_test_zone
verify_dns >/dev/null

checked_marker='[.services[]?.checked_at // ""] | max // ""'
history_marker='[.history[]?.created_at // ""] | max // ""'

# fetch_status <path> prints the status object, signing in again on a 401;
# prints nothing while the WebApp or Core is being recreated underneath it
# (connection errors, 502-504).
fetch_status() {
  local raw code
  raw="$(curl --silent --cookie "${cookie_file}" --write-out $'\n%{http_code}' "${base}$1" || true)"
  code="${raw##*$'\n'}"
  if [[ "${code}" == "401" ]]; then
    wait_for_login
  elif [[ "${code}" == "200" ]]; then
    printf '%s' "${raw%$'\n'*}"
  fi
}

# run_and_settle <path> <action-path> <marker> starts an action and prints the
# status once it failed, or is idle *and* the marker (a jq expression over
# the status) has moved - right after the POST, the status can still show the
# previous run's idle state. A failure that stops an update before it changes
# anything records no history entry, only state "failed", so that one can't
# wait for the marker; every earlier step succeeded, so it can't be stale.
run_and_settle() {
  local path="$1" action="$2" marker="$3" before body state="" current
  before="$(jq -r "${marker}" <<<"$(fetch_status "${path}")" 2>/dev/null || true)"
  curl --fail --silent --show-error --cookie "${cookie_file}" --request POST "${base}${action}" >/dev/null
  for _ in {1..180}; do
    body="$(fetch_status "${path}")"
    if [[ -n "${body}" ]]; then
      state="$(jq -r '.state // empty' <<<"${body}" 2>/dev/null || true)"
      current="$(jq -r "${marker}" <<<"${body}" 2>/dev/null || true)"
      if [[ "${state}" == "failed" || ("${state}" == "idle" && "${current}" != "${before}") ]]; then
        printf '%s' "${body}"
        return 0
      fi
    fi
    sleep 2
  done
  echo "${action} did not finish within 6 minutes (last state: ${state:-unreachable})" >&2
  exit 1
}

# expect_success <status-json> <service|""> <step> checks the newest history
# entry (for one service, if given) ended in success.
expect_success() {
  local status="$1" service="$2" step="$3" outcome
  if [[ -n "${service}" ]]; then
    outcome="$(jq -r --arg s "${service}" '[.history[]? | select(.service == $s)][0].outcome // "none"' <<<"${status}")"
  else
    outcome="$(jq -r '.history[0].outcome // "none"' <<<"${status}")"
  fi
  if [[ "${outcome}" != "success" ]]; then
    echo "${step} ended with outcome ${outcome}:" >&2
    jq . <<<"${status}" >&2
    docker logs --tail 40 rootguard-docker-proxy 2>&1 | grep -i reject >&2 || true
    exit 1
  fi
  echo "   ${step}: success"
}

echo "== Core and WebApp through the control-plane updater"
status="$(run_and_settle /api/control-plane-updates /api/control-plane-updates/check "${checked_marker}")"
if [[ "$(jq '[.services[] | select(.update_available)] | length' <<<"${status}")" == "0" ]]; then
  echo "${from_version} was not offered a control-plane update to ${to_version}:" >&2
  jq . <<<"${status}" >&2
  exit 1
fi
expect_success "$(run_and_settle /api/control-plane-updates /api/control-plane-updates/install "${history_marker}")" "" "control plane"

echo "== Updater and Attestation Proxy"
status="$(run_and_settle /api/updater-updates /api/updater-updates/check "${checked_marker}")"
for service in $(jq -r '.services[] | select(.update_available) | .name' <<<"${status}"); do
  expect_success "$(run_and_settle /api/updater-updates "/api/updater-updates/install/${service}" "${history_marker}")" "${service}" "${service}"
done

echo "== DNS services through Core's updater"
status="$(run_and_settle /api/updates /api/updates/check "${checked_marker}")"
for service in $(jq -r '.services[] | select(.update_available) | .name' <<<"${status}"); do
  expect_success "$(run_and_settle /api/updates "/api/updates/${service}" "${history_marker}")" "${service}" "${service}"
done

echo "== Every updatable component now runs ${to_version}'s pinned image"
failures=0
check_pin() {
  local container="$1" variable="$2" pinned expected actual
  pinned="$(sed -nE "s/.*\\\$\\{${variable}:-([^}\"]+)\\}.*/\\1/p" <<<"${target_compose}" | head -1)"
  if [[ -z "${pinned}" ]]; then
    echo "   ${container}: no ${variable} pin in v${to_version}'s compose.release.yaml" >&2
    failures=$((failures + 1))
    return
  fi
  docker pull --quiet "${pinned}" >/dev/null
  expected="$(docker image inspect --format '{{.Id}}' "${pinned}")"
  actual="$(docker inspect --format '{{.Image}}' "${container}")"
  if [[ "${actual}" == "${expected}" ]]; then
    echo "   ${container}: ${pinned%@*}"
  else
    echo "   ${container}: runs ${actual}, ${to_version} pins ${pinned} (${expected})" >&2
    failures=$((failures + 1))
  fi
}
check_pin rootguard-core ROOTGUARD_CORE_UPDATE_IMAGE
check_pin rootguard-webapp ROOTGUARD_WEBAPP_UPDATE_IMAGE
check_pin rootguard-updater ROOTGUARD_UPDATER_UPDATE_IMAGE
check_pin rootguard-attestation-proxy ROOTGUARD_ATTESTATION_PROXY_UPDATE_IMAGE
check_pin rootguard-unbound ROOTGUARD_UNBOUND_UPDATE_IMAGE
check_pin rootguard-blockpage ROOTGUARD_BLOCKPAGE_UPDATE_IMAGE
check_pin rootguard-adguard ROOTGUARD_ADGUARD_UPDATE_IMAGE
if (( failures > 0 )); then
  exit 1
fi
# No update channel (#670): the proxy stays on the version installed first.
echo "   rootguard-docker-proxy: $(docker inspect --format '{{ index .Config.Labels "org.opencontainers.image.version" }}' rootguard-docker-proxy) (no update channel)"

echo "== DNS after the upgrade"
wire_local_dnssec_test_zone
answer="$(verify_dns)"

echo "RootGuard upgrade journey ${from_version} -> ${to_version} passed"
echo "platform=${normalized_arch}"
echo "recursive_dns=${answer}"
echo "dnssec_invalid_chain=SERVFAIL"
