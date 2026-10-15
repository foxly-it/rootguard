#!/usr/bin/env bash
set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -f -- "$tmp"/bin/docker "$tmp"/calls; rm -rf -- "$tmp"/cache; rmdir -- "$tmp"/bin "$tmp"' EXIT
mkdir -p "$tmp/bin"
# shellcheck source=scripts/ci/trivy-version.env
source "$repository_dir/scripts/ci/trivy-version.env"

# Stub docker: logs every call; a trivy image scan of $SCAN_FAIL fails.
cat >"$tmp/bin/docker" <<'EOF'
#!/usr/bin/env bash
echo "$*" >>"$SCAN_CALLS"
[[ "$1" == "run" && -n "${SCAN_FAIL:-}" && "$*" == *"--exit-code 1 rootguard-${SCAN_FAIL}:scan"* ]] && exit 1
exit 0
EOF
chmod +x "$tmp/bin/docker"
export PATH="$tmp/bin:$PATH" SCAN_CALLS="$tmp/calls" TRIVY_CACHE_DIR="$tmp/cache"
fail() { echo "FAIL: $*" >&2; exit 1; }
run() { : >"$SCAN_CALLS"; "$repository_dir/scripts/ci/scan-images.sh" "$@" >/dev/null 2>&1; }

run core blockpage || fail "passing scans should exit 0"
grep -q "^build -q -t rootguard-core:scan rootguard-core$" "$SCAN_CALLS" || fail "core not built"
grep -q "aquasec/trivy:${TRIVY_VERSION}@${TRIVY_IMAGE_DIGEST} image --quiet --severity HIGH,CRITICAL --ignorefile /src/.trivyignore.yaml --exit-code 1 rootguard-blockpage:scan" "$SCAN_CALLS" \
  || fail "blockpage not scanned with the pinned image and CI gate"
grep -q " fs /src .*--exit-code 1" "$SCAN_CALLS" || fail "trivy fs not run"
grep -q -- "--severity UNKNOWN" "$SCAN_CALLS" && fail "preview ran without --preview"

set +e; SCAN_FAIL=core run core blockpage; rc=$?; set -e
(( rc == 1 )) || fail "a failing image scan should exit 1, got $rc"
grep -q "rootguard-blockpage:scan" "$SCAN_CALLS" || fail "a failing image stopped the remaining scans"

run --preview core || fail "--preview should not gate"
grep -q -- "--severity UNKNOWN --ignorefile /src/.trivyignore.yaml rootguard-core:scan" "$SCAN_CALLS" \
  || fail "--preview did not list UNKNOWN findings"

echo "All scan-images.sh tests passed."
