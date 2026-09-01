#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/ci/trivy-version.env
source "$script_dir/trivy-version.env"
export TRIVY_VERSION

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -f -- "$tmp_dir/bin/trivy" "$tmp_dir/args"
  rmdir -- "$tmp_dir/bin" "$tmp_dir"
}
trap cleanup EXIT

mkdir -p "$tmp_dir/bin"
cat >"$tmp_dir/bin/trivy" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then
  echo "Version: ${TRIVY_VERSION:?}"
  exit 0
fi
printf '%s\n' "$@" >"$TRIVY_ARGS_FILE"
EOF
chmod +x "$tmp_dir/bin/trivy"

export PATH="$tmp_dir/bin:$PATH"
export TRIVY_ARGS_FILE="$tmp_dir/args"

expect_usage_error() {
  local output status
  set +e
  output="$("$script_dir/trivy-image-scan.sh" "$@" 2>&1)"
  status=$?
  set -e
  [[ $status -eq 2 ]]
  [[ "$output" == usage:* ]]
}

expect_usage_error
expect_usage_error --platform
expect_usage_error --platform linux/amd64
expect_usage_error image:tag unexpected

"$script_dir/trivy-image-scan.sh" image:tag
grep -Fxq -- "image:tag" "$TRIVY_ARGS_FILE"

"$script_dir/trivy-image-scan.sh" --platform linux/arm64 image:tag
grep -Fxq -- "--platform" "$TRIVY_ARGS_FILE"
grep -Fxq -- "linux/arm64" "$TRIVY_ARGS_FILE"
grep -Fxq -- "image:tag" "$TRIVY_ARGS_FILE"

echo "trivy-image-scan argument tests passed"
