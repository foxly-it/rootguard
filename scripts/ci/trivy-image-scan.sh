#!/usr/bin/env bash
# Scans a single already-built container image for HIGH/CRITICAL
# vulnerabilities.
#
# Found in review: ci-security.yml's own trivy job only ever ran
# `trivy fs .` - repo files, dependency manifests, and Dockerfile
# misconfigurations. It never looked at what a *built* image's base
# layers and bundled binaries actually contain, so a CVE baked into the
# pinned runtime base (docker:29-cli, e.g. - see rootguard-core and
# rootguard-updater's own Dockerfiles) never failed CI, even though it
# ships in every published image. Called once per image, right after
# that workflow's own `docker build`, so it scans the exact content a
# real PR/release would ship - not a separately-tagged or hypothetical
# one.
#
# Usage: ./scripts/ci/trivy-image-scan.sh [--platform <os/arch>] <image-ref>
# Run from the repo root (matches every caller - ci-core.yml,
# ci-updater.yml, ci-webapp.yml - none of which set a working-directory
# on the step that calls this), so the relative .trivyignore.yaml path
# below resolves the same way ci-security.yml's own trivy step already
# relies on.
set -Eeuo pipefail

platform=""
if [[ "${1:-}" == "--platform" ]]; then
  if [[ $# -lt 3 || -z "${2:-}" ]]; then
    echo "usage: $0 [--platform <os/arch>] <image-ref>" >&2
    exit 2
  fi
  platform="$2"
  shift 2
fi
if [[ $# -ne 1 ]]; then
  echo "usage: $0 [--platform <os/arch>] <image-ref>" >&2
  exit 2
fi
image="$1"

# Found in review: the arch-aware, skip-if-already-pinned install logic
# that used to live here (rounds 14/15) is now install-trivy.sh, shared
# with ci-security.yml's own `trivy fs .` job - see that script's own
# header for the full history.
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
"${script_dir}/install-trivy.sh"

platform_args=()
if [[ -n "$platform" ]]; then
  platform_args=(--platform "$platform")
fi

# SCAN_GATE_PATHS (space-separated globs) opts into pr-gate.sh (#719): on a
# PR that touches none of them, findings are reported but don't fail.
gate=1
if [[ -n "${SCAN_GATE_PATHS:-}" ]]; then
  read -r -a gate_paths <<<"${SCAN_GATE_PATHS}"
  "${script_dir}/pr-gate.sh" "${gate_paths[@]}" || gate=0
fi

if trivy image \
  --severity HIGH,CRITICAL \
  --ignorefile .trivyignore.yaml \
  --exit-code 1 \
  "${platform_args[@]}" \
  "$image"; then
  exit 0
fi
(( gate )) && exit 1
echo "::warning title=Not gating::HIGH/CRITICAL findings in ${image} (listed above), but this PR doesn't touch it - main's push and nightly scans still gate on them."
