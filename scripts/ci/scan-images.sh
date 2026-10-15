#!/usr/bin/env bash
# Builds RootGuard's images and scans them exactly like CI does (#714): the
# trivy version pinned in trivy-version.env, run as its digest-pinned
# container image so macOS and Linux get the identical binary, the same
# HIGH/CRITICAL gate and ignore file as trivy-image-scan.sh, plus the
# repository-wide `trivy fs` scan from ci-security.yml.
#
# --preview also lists UNKNOWN findings (not gating): trivy's DB often
# rates a new CVE days after it appears, so these are what is most likely
# to turn CI red next.
#
# Usage: ./scripts/ci/scan-images.sh [--preview] [component...]
#   component = core updater webapp unbound blockpage docker-proxy
#               attestation-proxy (default: all)
# Exit code: 0 when every scan passes the gate, 1 otherwise.
set -Eeuo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."
# shellcheck source=scripts/ci/trivy-version.env
source scripts/ci/trivy-version.env
preview=0
[[ "${1:-}" == "--preview" ]] && { preview=1; shift; }
(( $# )) || set -- core updater webapp unbound blockpage docker-proxy attestation-proxy

cache="${TRIVY_CACHE_DIR:-${HOME}/.cache/rootguard-trivy}"
mkdir -p "${cache}"
trivy() {
  docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "${cache}:/root/.cache/trivy" \
    -v "${PWD}:/src:ro" \
    "aquasec/trivy:${TRIVY_VERSION}@${TRIVY_IMAGE_DIGEST}" "$@"
}

failed=()
for component in "$@"; do
  image="rootguard-${component}:scan"
  echo "== ${component}"
  docker build -q -t "${image}" "rootguard-${component}" >/dev/null
  trivy image --quiet --severity HIGH,CRITICAL --ignorefile /src/.trivyignore.yaml \
    --exit-code 1 "${image}" || failed+=("${component}")
  if (( preview )); then
    echo "-- ${component}: UNKNOWN (preview, not gating)"
    trivy image --quiet --skip-db-update --severity UNKNOWN \
      --ignorefile /src/.trivyignore.yaml "${image}"
  fi
done

echo "== repository (trivy fs)"
trivy fs /src --quiet --scanners vuln,secret,misconfig --ignorefile /src/.trivyignore.yaml \
  --skip-dirs '**/node_modules' --exit-code 1 || failed+=("fs")

if (( ${#failed[@]} )); then
  echo "FAILED: ${failed[*]}" >&2
  exit 1
fi
echo "All scans pass the CI gate."
