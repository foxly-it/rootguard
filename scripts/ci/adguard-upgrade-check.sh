#!/usr/bin/env bash
# Asserts that a freshly recreated AdGuard Home container came back on the
# expected version with its state intact. Used by ci-adguard-compat.yml's
# upgrade job after each image swap: the volumes stay, only the binary
# changes, which is exactly what an update on a real installation does.
#
# A fresh install can't show this. Everything else in CI starts from an empty
# volume, so it would never notice a release that can't read an existing
# configuration, or one that loses it.
#
# Usage: ./scripts/ci/adguard-upgrade-check.sh <expected-version> <cookie-jar> <user-rule>
set -Eeuo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: $0 <expected-version> <cookie-jar> <user-rule>" >&2
  exit 2
fi
expected_version="$1"
cookie_jar="$2"
user_rule="$3"
base_url="${ROOTGUARD_URL:-http://127.0.0.1:18080}"

status=""
for _ in $(seq 1 60); do
  status="$(curl --fail --silent --cookie "$cookie_jar" "${base_url}/api/adguard/status" || true)"
  [[ "$(jq -r .version <<<"${status:-null}")" == "$expected_version" ]] && break
  sleep 2
done
actual_version="$(jq -r .version <<<"${status:-null}")"
if [[ "$actual_version" != "$expected_version" ]]; then
  echo "::error::Expected AdGuard ${expected_version}, got ${actual_version}" >&2
  exit 1
fi

for field in configured healthy best_practices_ready upstream_ready filtering_enabled; do
  if [[ "$(jq -r ".${field}" <<<"$status")" != "true" ]]; then
    echo "::error::Expected ${field} to stay true on ${expected_version}" >&2
    jq . <<<"$status" >&2
    exit 1
  fi
done

rules="$(curl --fail --silent --cookie "$cookie_jar" "${base_url}/adguard-ui/control/filtering/status" | jq -r '.user_rules // [] | .[]')"
if ! grep -qxF "$user_rule" <<<"$rules"; then
  echo "::error::User rule ${user_rule} did not survive the switch to ${expected_version}" >&2
  exit 1
fi

answer="$(dig +short +time=5 +tries=3 @127.0.0.1 -p "${ROOTGUARD_DNS_PORT:-1053}" good.rgtest-ci.internal A)"
if [[ -z "$answer" ]]; then
  echo "::error::No DNS answer through AdGuard on ${expected_version}" >&2
  exit 1
fi

echo "AdGuard ${expected_version}: state intact, DNS answering"
