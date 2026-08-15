#!/usr/bin/env bash

set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="${ROOTGUARD_TEST_COMPOSE_FILE:-${repository_dir}/compose.alpha.yaml}"
web_port="${ROOTGUARD_TEST_WEB_PORT:-18080}"
dns_port="${ROOTGUARD_TEST_DNS_PORT:-1053}"
expected_arch="${ROOTGUARD_TEST_ARCH:-}"
cookie_file="$(mktemp "${TMPDIR:-/tmp}/rootguard-clean-install.XXXXXX")"

# shellcheck source=verification-common.sh
. "$(dirname "${BASH_SOURCE[0]}")/verification-common.sh"

require_commands curl dig docker jq

# Must run before the cleanup trap is registered - see
# verification-common.sh's own comment on guard_no_existing_resources.
guard_no_existing_resources
trap cleanup EXIT

normalized_arch="$(detect_arch)"
export ROOTGUARD_API_TOKEN="clean-install-api-token-${normalized_arch}"
export ROOTGUARD_ADMIN_USER="admin"
export ROOTGUARD_ADMIN_PASSWORD="clean-install-password-${normalized_arch}"
export ROOTGUARD_RECOVERY_TOKEN="clean-install-recovery-token-${normalized_arch}"
export ROOTGUARD_WEB_BIND="127.0.0.1"
export ROOTGUARD_WEB_PORT="${web_port}"

echo "Pulling immutable RootGuard alpha images for ${normalized_arch}"
docker compose -f "${compose_file}" pull
install_stack
answer="$(verify_dns)"

echo "RootGuard clean install passed"
echo "platform=${normalized_arch}"
echo "docker=$(docker version --format '{{.Server.Version}}')"
echo "compose=$(docker compose version --short)"
echo "recursive_dns=${answer}"
echo "dnssec_invalid_chain=SERVFAIL"
