#!/usr/bin/env bash
# Regression tests for inject.sh - found in a v1.0.0 correctness review:
# with "docker inspect | grep | sort | head" as one pipeline, set -e
# aborted the script the instant docker inspect itself failed, before
# the script's own friendly, actionable "no gateway IP" error message
# ever had a chance to run - that message was only ever reachable for a
# container that inspects fine but genuinely has no gateway on any of
# its networks, not for the "container doesn't exist" case it also
# claims to cover.
set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
tmp_dir="$(mktemp -d)"
cleanup() { rm -rf -- "$tmp_dir"; }
trap cleanup EXIT

mkdir -p "$tmp_dir/bin" "$tmp_dir/zone"
echo "fake-trust-anchor-line" > "$tmp_dir/zone/trust-anchor"
export DNSSEC_TEST_ZONE_DIR="$tmp_dir/zone"

failures=0
fail() { echo "FAIL: $1" >&2; failures=$((failures + 1)); }

# DOCKER_TEST_MODE selects the fake docker's behavior for this run.
write_fake_docker() {
  cat >"$tmp_dir/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
if [[ "$1" == "inspect" ]]; then
  case "${DOCKER_TEST_MODE:-}" in
    inspect-fails) exit 1 ;;
    no-gateway) exit 0 ;;
    *) printf '172.29.53.1\n' ;;
  esac
  exit 0
fi
# cp/exec/restart - accepted unconditionally, nothing under test reads
# their output.
exit 0
EOF
  chmod +x "$tmp_dir/bin/docker"
}
write_fake_docker
export PATH="$tmp_dir/bin:$PATH"

run_inject() {
  DOCKER_TEST_MODE="$1" "$repository_dir/scripts/ci/dnssec-test-zone/inject.sh" fake-container
}

# --- docker inspect itself failing (container doesn't exist, daemon
# unreachable) must produce its own specific error, not a bare set -e
# abort - the core regression ---
output="$(run_inject inspect-fails 2>&1)" && status=0 || status=$?
if [[ "$status" -ne 1 ]]; then
  fail "inspect failure: expected exit 1, got $status"
fi
if [[ "$output" != *"docker inspect fake-container failed"* ]]; then
  fail "inspect failure: expected the docker-inspect-specific error, got: $output"
fi

# --- docker inspect succeeding but reporting no gateway at all (e.g. an
# internal-only network) must still produce the original, differently
# worded message - this path must remain reachable too ---
output="$(run_inject no-gateway 2>&1)" && status=0 || status=$?
if [[ "$status" -ne 1 ]]; then
  fail "no gateway: expected exit 1, got $status"
fi
if [[ "$output" != *"has no network gateway IP"* ]]; then
  fail "no gateway: expected the no-gateway-specific error, got: $output"
fi

# --- a real gateway IP must still let the script complete successfully ---
output="$(run_inject "" 2>&1)" && status=0 || status=$?
if [[ "$status" -ne 0 ]]; then
  fail "happy path: expected exit 0, got $status: $output"
fi

if [[ "$failures" -gt 0 ]]; then
  echo "$failures inject.sh test failure(s)" >&2
  exit 1
fi
echo "All inject.sh tests passed."
