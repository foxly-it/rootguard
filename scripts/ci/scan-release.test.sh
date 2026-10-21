#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -f -- "$tmp"/bin/trivy "$tmp"/calls "$tmp"/out; rmdir -- "$tmp"/bin "$tmp"' EXIT
mkdir -p "$tmp/bin"

# Stub trivy: logs each image/platform; core on arm64 has one finding,
# everything else is clean; SCAN_FAIL=<image> makes that scan fail.
cat >"$tmp/bin/trivy" <<'EOF'
#!/usr/bin/env bash
image="${*: -1}" platform=""
while (( $# )); do [[ "$1" == "--platform" ]] && platform="$2"; shift; done
echo "${image} ${platform}" >>"$SCAN_CALLS"
[[ "${SCAN_FAIL:-}" == "${image}" ]] && exit 1
if [[ "${image}" == *rootguard-core:* && "${platform}" == linux/arm64 ]]; then
  echo '{"Results":[{"Target":"usr/local/bin/cosign","Vulnerabilities":[{"PkgName":"stdlib","InstalledVersion":"v1.27.1","VulnerabilityID":"CVE-1"}]}]}'
else
  echo '{"Results":[{"Target":"x"}]}'
fi
EOF
chmod +x "$tmp/bin/trivy"
export PATH="$tmp/bin:$PATH" SCAN_CALLS="$tmp/calls"

fail() { echo "FAIL: $*" >&2; exit 1; }

"$script_dir/scan-release.sh" 9.9.9 >"$tmp/out" || fail "clean scan run should exit 0"
[[ "$(cat "$tmp/out")" == $'core\tlinux/arm64\tusr/local/bin/cosign\tstdlib\tv1.27.1\tCVE-1' ]] \
  || fail "unexpected findings: $(cat "$tmp/out")"
[[ "$(wc -l <"$tmp/calls")" -eq 14 ]] || fail "expected 7 components x 2 platforms, got $(wc -l <"$tmp/calls")"
grep -qx "ghcr.io/foxly-it/rootguard-docker-proxy:9.9.9 linux/amd64" "$tmp/calls" || fail "docker-proxy amd64 not scanned"

: >"$tmp/calls"
if SCAN_FAIL="ghcr.io/foxly-it/rootguard-unbound:9.9.9" "$script_dir/scan-release.sh" 9.9.9 >/dev/null 2>&1; then
  fail "a failed image scan must fail the run"
fi

echo "All scan-release.sh tests passed."
