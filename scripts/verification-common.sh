#!/usr/bin/env bash
# Shared helpers for scripts/verify-clean-install.sh and
# scripts/verify-backup-restore.sh - resource guard, login, install/restore
# polling, and DNS verification logic both scripts need identically.
# Sourced, not executed directly. Callers must set compose_file, web_port,
# dns_port, and cookie_file before sourcing this file.

managed_containers=(
  rootguard-webapp
  rootguard-core
  rootguard-updater
  rootguard-adguard
  rootguard-unbound
)
managed_volumes=(
  rootguard-data
  rootguard-sessions
  rootguard-unbound-config
  rootguard-unbound-state
  rootguard-adguard-work
  rootguard-adguard-config
)

# owns_managed_resources tracks whether *this* script instance actually
# started the managed containers, not merely whether it detected them.
# cleanup only ever tears down resources gated behind this flag - it must
# never delete something guard_no_existing_resources found pre-existing,
# which is exactly what happened before this file existed: both scripts
# registered `trap cleanup EXIT` before running their own "refuse to
# overwrite" checks, so the checks' own `exit 1` triggered the very
# deletion they existed to prevent.
owns_managed_resources=false

teardown_managed_resources() {
  docker rm -f "${managed_containers[@]}" >/dev/null 2>&1 || true
  docker compose -f "${compose_file}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  docker network rm rootguard-dns >/dev/null 2>&1 || true
  docker volume rm "${managed_volumes[@]}" >/dev/null 2>&1 || true
}

require_commands() {
  for command in "$@"; do
    if ! command -v "${command}" >/dev/null 2>&1; then
      echo "Required command not found: ${command}" >&2
      exit 1
    fi
  done
  docker compose version >/dev/null
  docker info >/dev/null
}

# guard_no_existing_resources refuses to run against pre-existing managed
# resources. Callers MUST call this before `trap cleanup EXIT` - it never
# touches owns_managed_resources or expects a cleanup trap to be active,
# so its own exit-on-refusal path is guaranteed inert rather than
# destructive regardless of call order elsewhere.
guard_no_existing_resources() {
  for container in "${managed_containers[@]}"; do
    if docker container inspect "${container}" >/dev/null 2>&1; then
      echo "Refusing to overwrite existing container: ${container}" >&2
      exit 1
    fi
  done
  for volume in "${managed_volumes[@]}"; do
    if docker volume inspect "${volume}" >/dev/null 2>&1; then
      echo "Refusing to overwrite existing volume: ${volume}" >&2
      exit 1
    fi
  done
  if docker network inspect rootguard-dns >/dev/null 2>&1; then
    echo "Refusing to overwrite existing network: rootguard-dns" >&2
    exit 1
  fi
}

# cleanup is safe to register via `trap cleanup EXIT` at any point, even
# before guard_no_existing_resources runs - it only ever removes temp
# files unconditionally and only tears down managed Docker resources when
# owns_managed_resources is true, so a guard refusal or any failure before
# this run started its own containers can never delete anything besides
# its own cookie/archive files.
cleanup() {
  result=$?
  trap - EXIT
  if (( result != 0 )); then
    docker compose -f "${compose_file}" logs --no-color 2>/dev/null || true
    docker logs rootguard-adguard 2>/dev/null || true
    docker logs rootguard-unbound 2>/dev/null || true
  fi
  if [[ "${owns_managed_resources}" == true ]]; then
    teardown_managed_resources
  fi
  rm -f "${cookie_file:-}" "${archive_file:-}"
  exit "${result}"
}

detect_arch() {
  local actual_arch normalized_arch
  actual_arch="$(docker info --format '{{.Architecture}}')"
  case "${actual_arch}" in
    x86_64) normalized_arch="amd64" ;;
    aarch64) normalized_arch="arm64" ;;
    *) normalized_arch="${actual_arch}" ;;
  esac
  if [[ -n "${expected_arch:-}" && "${normalized_arch}" != "${expected_arch}" ]]; then
    echo "Expected Docker architecture ${expected_arch}, got ${actual_arch}" >&2
    exit 1
  fi
  printf '%s' "${normalized_arch}"
}

wait_for_login() {
  local code
  for _ in {1..60}; do
    code="$(curl --silent --output /dev/null --write-out '%{http_code}' \
      --cookie-jar "${cookie_file}" \
      --header 'Content-Type: application/json' \
      --data "{\"username\":\"admin\",\"password\":\"${ROOTGUARD_ADMIN_PASSWORD}\"}" \
      "http://127.0.0.1:${web_port}/api/auth/login" || true)"
    [[ "${code}" == "200" ]] && return 0
    sleep 2
  done
  echo "WebApp login did not become ready (HTTP ${code})" >&2
  exit 1
}

# wait_for_installed polls /api/installation until it reaches "installed",
# used both by install_stack (fresh deploy) and a restore's own polling -
# identical logic either way.
wait_for_installed() {
  local state=""
  for _ in {1..90}; do
    state="$(curl --fail --silent --cookie "${cookie_file}" \
      "http://127.0.0.1:${web_port}/api/installation" | jq -r .state)"
    [[ "${state}" == "installed" ]] && return 0
    if [[ "${state}" == "failed" ]]; then
      echo "Installation failed" >&2
      exit 1
    fi
    sleep 2
  done
  echo "Installation timed out in state ${state}" >&2
  exit 1
}

# install_stack marks this run as owning the managed resources *before*
# starting the compose stack, then signs in and runs preflight + deploy
# through to "installed". The flag must flip before `compose up`, not
# after: callers only ever reach this function once
# guard_no_existing_resources has already confirmed nothing managed
# exists yet, so anything that appears under the managed names from here
# on unconditionally belongs to this run - including a partially created
# network/volume/container left behind by a `compose up` that itself
# fails (image pull error, port conflict, failed healthcheck, ...). With
# the flag set only after a *successful* `compose up`, that exact failure
# left such partial resources on disk with owns_managed_resources still
# false, so cleanup skipped them entirely and the next run's own guard
# check would then refuse to start against its own run's leftovers.
install_stack() {
  owns_managed_resources=true
  docker compose -f "${compose_file}" up -d
  wait_for_login
  local config
  config="$(jq -n --arg address "127.0.0.1" --argjson port "${dns_port}" \
    '{dns_bind_address:$address,dns_port:$port}')"
  local preflight
  preflight="$(curl --fail --silent --cookie "${cookie_file}" \
    --header 'Content-Type: application/json' --data "${config}" \
    "http://127.0.0.1:${web_port}/api/installation/preflight")"
  if [[ "$(jq -r .ready <<<"${preflight}")" != "true" ]]; then
    echo "Installation preflight failed: ${preflight}" >&2
    exit 1
  fi
  curl --fail --silent --cookie "${cookie_file}" \
    --header 'Content-Type: application/json' --data "${config}" \
    "http://127.0.0.1:${web_port}/api/installation/deploy" >/dev/null
  wait_for_installed
}

# verify_dns asserts recursive resolution works and an invalid DNSSEC chain
# is rejected, printing the resolved address to stdout on success.
verify_dns() {
  local answer dnssec_status
  answer="$(dig +short +time=5 +tries=2 @127.0.0.1 -p "${dns_port}" example.com A)"
  if [[ -z "${answer}" ]]; then
    echo "Recursive DNS returned no address" >&2
    exit 1
  fi
  dnssec_status="$(dig +dnssec +time=5 +tries=2 @127.0.0.1 -p "${dns_port}" \
    dnssec-failed.org A | sed -n \
    's/^;; ->>HEADER<<- opcode: QUERY, status: \([^,]*\).*/\1/p')"
  if [[ "${dnssec_status}" != "SERVFAIL" ]]; then
    echo "Invalid DNSSEC chain was not rejected (status ${dnssec_status})" >&2
    exit 1
  fi
  printf '%s' "${answer%%$'\n'*}"
}
