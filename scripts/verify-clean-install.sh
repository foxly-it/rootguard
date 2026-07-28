#!/usr/bin/env bash

set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="${ROOTGUARD_TEST_COMPOSE_FILE:-${repository_dir}/compose.alpha.yaml}"
web_port="${ROOTGUARD_TEST_WEB_PORT:-18080}"
dns_port="${ROOTGUARD_TEST_DNS_PORT:-1053}"
expected_arch="${ROOTGUARD_TEST_ARCH:-}"
cookie_file="$(mktemp "${TMPDIR:-/tmp}/rootguard-clean-install.XXXXXX")"

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

cleanup() {
  result=$?
  trap - EXIT
  if (( result != 0 )); then
    docker compose -f "${compose_file}" logs --no-color 2>/dev/null || true
    docker logs rootguard-adguard 2>/dev/null || true
    docker logs rootguard-unbound 2>/dev/null || true
  fi
  docker rm -f "${managed_containers[@]}" >/dev/null 2>&1 || true
  docker compose -f "${compose_file}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  docker network rm rootguard-dns >/dev/null 2>&1 || true
  docker volume rm "${managed_volumes[@]}" >/dev/null 2>&1 || true
  rm -f "${cookie_file}"
  exit "${result}"
}
trap cleanup EXIT

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Required command not found: $1" >&2
    exit 1
  fi
}

for command in curl dig docker jq; do
  require_command "${command}"
done
docker compose version >/dev/null
docker info >/dev/null

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

actual_arch="$(docker info --format '{{.Architecture}}')"
case "${actual_arch}" in
  x86_64) normalized_arch="amd64" ;;
  aarch64) normalized_arch="arm64" ;;
  *) normalized_arch="${actual_arch}" ;;
esac
if [[ -n "${expected_arch}" && "${normalized_arch}" != "${expected_arch}" ]]; then
  echo "Expected Docker architecture ${expected_arch}, got ${actual_arch}" >&2
  exit 1
fi

export ROOTGUARD_API_TOKEN="clean-install-api-token-${normalized_arch}"
export ROOTGUARD_ADMIN_USER="admin"
export ROOTGUARD_ADMIN_PASSWORD="clean-install-password-${normalized_arch}"
export ROOTGUARD_RECOVERY_TOKEN="clean-install-recovery-token-${normalized_arch}"
export ROOTGUARD_WEB_BIND="127.0.0.1"
export ROOTGUARD_WEB_PORT="${web_port}"

echo "Pulling immutable RootGuard alpha images for ${normalized_arch}"
docker compose -f "${compose_file}" pull
docker compose -f "${compose_file}" up -d

login_code=""
for _ in {1..60}; do
  login_code="$(curl --silent --output /dev/null --write-out '%{http_code}' \
    --cookie-jar "${cookie_file}" \
    --header 'Content-Type: application/json' \
    --data "{\"username\":\"admin\",\"password\":\"${ROOTGUARD_ADMIN_PASSWORD}\"}" \
    "http://127.0.0.1:${web_port}/api/auth/login" || true)"
  [[ "${login_code}" == "200" ]] && break
  sleep 2
done
if [[ "${login_code}" != "200" ]]; then
  echo "WebApp login did not become ready (HTTP ${login_code})" >&2
  exit 1
fi

config="$(jq -n --arg address "127.0.0.1" --argjson port "${dns_port}" \
  '{dns_bind_address:$address,dns_port:$port}')"
preflight="$(curl --fail --silent \
  --cookie "${cookie_file}" \
  --header 'Content-Type: application/json' \
  --data "${config}" \
  "http://127.0.0.1:${web_port}/api/installation/preflight")"
if [[ "$(jq -r .ready <<<"${preflight}")" != "true" ]]; then
  echo "Installation preflight failed: ${preflight}" >&2
  exit 1
fi

curl --fail --silent \
  --cookie "${cookie_file}" \
  --header 'Content-Type: application/json' \
  --data "${config}" \
  "http://127.0.0.1:${web_port}/api/installation/deploy" >/dev/null

state=""
installation=""
for _ in {1..90}; do
  installation="$(curl --fail --silent \
    --cookie "${cookie_file}" \
    "http://127.0.0.1:${web_port}/api/installation")"
  state="$(jq -r .state <<<"${installation}")"
  [[ "${state}" == "installed" ]] && break
  if [[ "${state}" == "failed" ]]; then
    echo "Installation failed: ${installation}" >&2
    exit 1
  fi
  sleep 2
done
if [[ "${state}" != "installed" ]]; then
  echo "Installation timed out in state ${state}: ${installation}" >&2
  exit 1
fi

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

echo "RootGuard clean install passed"
echo "platform=${normalized_arch}"
echo "docker=$(docker version --format '{{.Server.Version}}')"
echo "compose=$(docker compose version --short)"
echo "recursive_dns=${answer%%$'\n'*}"
echo "dnssec_invalid_chain=${dnssec_status}"
