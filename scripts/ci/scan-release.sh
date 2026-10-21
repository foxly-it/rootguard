#!/usr/bin/env bash
# Lists the HIGH/CRITICAL findings in a published release's images that
# the current .trivyignore.yaml does not cover (#772) - what users of that
# release are exposed to, and the input for security-release.yml's
# "would a new release help?" decision. Scans the registry images
# directly, both platforms, with the CI-pinned trivy on PATH.
#
# Usage: ./scripts/ci/scan-release.sh <version>    (e.g. 1.0.6)
# Output: one TSV line per finding:
#   component  platform  target  package  installed-version  vulnerability-id
# Exit code: 0 when every image was scanned (findings or not), non-zero
# when a scan itself failed.
set -Eeuo pipefail

version="${1:?usage: $0 <version>}"
owner="${RELEASE_IMAGE_OWNER:-foxly-it}"
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

for component in core webapp updater unbound blockpage attestation-proxy docker-proxy; do
  for platform in linux/amd64 linux/arm64; do
    trivy image --quiet --platform "${platform}" --severity HIGH,CRITICAL \
      --ignorefile .trivyignore.yaml --format json \
      "ghcr.io/${owner}/rootguard-${component}:${version}" \
      | jq -r --arg c "${component}" --arg p "${platform}" \
        '.Results[]? | .Target as $t | .Vulnerabilities[]?
         | [$c, $p, $t, .PkgName, .InstalledVersion, .VulnerabilityID] | @tsv'
  done
done
