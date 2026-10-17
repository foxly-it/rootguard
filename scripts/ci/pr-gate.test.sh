#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -f -- "$tmp"/bin/gh "$tmp"/bin/trivy "$tmp"/out; rmdir -- "$tmp"/bin "$tmp"' EXIT
mkdir -p "$tmp/bin"

# Stub gh: prints $CHANGED as the PR's file list, or fails on demand.
cat >"$tmp/bin/gh" <<'EOF'
#!/usr/bin/env bash
[[ "${GH_FAIL:-}" == 1 ]] && exit 1
[[ "$1 $2 $4" == "pr diff --name-only" ]] || exit 3
printf '%s\n' "$CHANGED"
EOF
# Stub trivy: version probe for install-trivy.sh, otherwise "findings".
cat >"$tmp/bin/trivy" <<'EOF'
#!/usr/bin/env bash
[[ "${1:-}" == "--version" ]] && { echo "Version: ${TRIVY_VERSION:?}"; exit 0; }
exit 1
EOF
chmod +x "$tmp/bin/gh" "$tmp/bin/trivy"
# shellcheck source=scripts/ci/trivy-version.env
source "$script_dir/trivy-version.env"
export TRIVY_VERSION PATH="$tmp/bin:$PATH" GITHUB_REPOSITORY=o/r \
  GITHUB_REF=refs/pull/42/merge \
  GITHUB_WORKFLOW_REF=o/r/.github/workflows/ci-core.yml@refs/pull/42/merge

fail() { echo "FAIL: $*" >&2; exit 1; }
gate() { "$script_dir/pr-gate.sh" "rootguard-core/*" 2>/dev/null; }

GITHUB_EVENT_NAME=push CHANGED="docs/x.md" gate || fail "push must gate"
export GITHUB_EVENT_NAME=pull_request
CHANGED=$'docs/x.md\nrootguard-core/go.mod' gate || fail "component change must gate"
CHANGED=".trivyignore.yaml" gate || fail "ignore-file change must gate"
CHANGED="scripts/ci/install-trivy.sh" gate || fail "scan tooling change must gate"
CHANGED=".github/workflows/ci-core.yml" gate || fail "own workflow change must gate"
CHANGED=$'docs/x.md\n.github/workflows/ci-webapp.yml' gate && fail "unrelated PR must only report"
GH_FAIL=1 CHANGED="" gate || fail "unknown file list must gate"

# trivy-image-scan.sh: findings fail when gating, warn when not.
cd "$tmp"
set +e
CHANGED="docs/x.md" SCAN_GATE_PATHS="rootguard-core/*" "$script_dir/trivy-image-scan.sh" img:t >"$tmp/out" 2>&1; rc=$?
set -e
(( rc == 0 )) || fail "unrelated PR scan should not fail, got $rc"
grep -q "::warning title=Not gating::" "$tmp/out" || fail "missing not-gating warning"
set +e
CHANGED="rootguard-core/Dockerfile" SCAN_GATE_PATHS="rootguard-core/*" "$script_dir/trivy-image-scan.sh" img:t >/dev/null 2>&1; rc=$?
set -e
(( rc == 1 )) || fail "component PR scan with findings should fail, got $rc"

echo "All pr-gate.sh tests passed."
