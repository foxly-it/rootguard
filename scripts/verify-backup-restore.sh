#!/usr/bin/env bash
# Verifies the documented backup/restore path end to end against real
# containers, not the mocked-Docker unit tests in
# rootguard-core/internal/backuprestore/*_test.go: install a primary
# instance, export an encrypted backup, tear the instance down completely,
# deploy a fresh uninstalled instance, restore the backup into it, and
# verify DNS resolution + DNSSEC rejection afterward. Shares its guard/
# install/cleanup logic with verify-clean-install.sh via
# verification-common.sh.

set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="${ROOTGUARD_TEST_COMPOSE_FILE:-${repository_dir}/compose.alpha.yaml}"
web_port="${ROOTGUARD_TEST_WEB_PORT:-18080}"
dns_port="${ROOTGUARD_TEST_DNS_PORT:-1053}"
expected_arch="${ROOTGUARD_TEST_ARCH:-}"
cookie_file="$(mktemp "${TMPDIR:-/tmp}/rootguard-backup-restore-cookies.XXXXXX")"
archive_file="$(mktemp "${TMPDIR:-/tmp}/rootguard-backup-restore-archive.XXXXXX.age")"
passphrase="verify-backup-restore-$(date +%s)-$$"

# shellcheck source=verification-common.sh
. "$(dirname "${BASH_SOURCE[0]}")/verification-common.sh"

require_commands curl dig docker jq

# Must run before the cleanup trap is registered - see
# verification-common.sh's own comment on guard_no_existing_resources.
guard_no_existing_resources
trap cleanup EXIT

normalized_arch="$(detect_arch)"
export ROOTGUARD_API_TOKEN="backup-restore-api-token-${normalized_arch}"
export ROOTGUARD_ADMIN_USER="admin"
export ROOTGUARD_ADMIN_PASSWORD="backup-restore-password-${normalized_arch}"
export ROOTGUARD_RECOVERY_TOKEN="backup-restore-recovery-token-${normalized_arch}"
export ROOTGUARD_WEB_BIND="127.0.0.1"
export ROOTGUARD_WEB_PORT="${web_port}"

deploy_config="$(jq -n --arg address "127.0.0.1" --argjson port "${dns_port}" \
  '{dns_bind_address:$address,dns_port:$port}')"

echo "Installing the primary instance to back up"
install_stack
verify_dns >/dev/null

echo "Exporting an encrypted backup"
curl --fail --silent --cookie "${cookie_file}" \
  --header 'Content-Type: application/json' \
  --data "{\"passphrase\":\"${passphrase}\"}" \
  -o "${archive_file}" \
  "http://127.0.0.1:${web_port}/api/backups/export"
if [[ ! -s "${archive_file}" ]]; then
  echo "Backup export produced an empty archive" >&2
  exit 1
fi

echo "Tearing the primary instance down"
teardown_managed_resources

echo "Deploying a fresh, uninstalled instance to restore into"
docker compose -f "${compose_file}" up -d
wait_for_login

echo "Restoring the backup"
curl --fail --silent --cookie "${cookie_file}" \
  --form "passphrase=${passphrase}" \
  --form "archive=@${archive_file};type=application/vnd.rootguard.backup+age" \
  --form "config=${deploy_config}" \
  --form "confirmation=RESTORE" \
  "http://127.0.0.1:${web_port}/api/backups/restore" >/dev/null
wait_for_installed

echo "Verifying DNS after restore"
answer="$(verify_dns)"

echo "RootGuard backup/restore cycle passed"
echo "platform=${normalized_arch}"
echo "docker=$(docker version --format '{{.Server.Version}}')"
echo "compose=$(docker compose version --short)"
echo "recursive_dns=${answer}"
echo "dnssec_invalid_chain=SERVFAIL"
