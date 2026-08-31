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
# Usage: ./scripts/ci/trivy-image-scan.sh <image-ref>
# Run from the repo root (matches every caller - ci-core.yml,
# ci-updater.yml, ci-webapp.yml - none of which set a working-directory
# on the step that calls this), so the relative .trivyignore.yaml path
# below resolves the same way ci-security.yml's own trivy step already
# relies on.
set -Eeuo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <image-ref>" >&2
  exit 2
fi
image="$1"

# Same pinned version as ci-security.yml's own trivy install - kept in
# sync by hand, not by reference, since a shared step needs GitHub
# Actions' reusable-workflow/composite-action machinery to avoid that
# duplication, and this repo's CI doesn't use either yet.
#
# Found in review, round 14: this used to hardcode the amd64 asset and
# checksum - silently fine on every caller except ci-unbound.yml's own
# arm64 matrix leg (ubuntu-24.04-arm), where it failed live with "cannot
# execute binary file: Exec format error". `uname -m` picks the matching
# release asset/checksum for both architectures this repo's CI actually
# runs on.
if ! command -v trivy >/dev/null 2>&1; then
  case "$(uname -m)" in
    x86_64)
      asset="trivy_0.73.0_Linux-64bit.tar.gz"
      checksum="2edd39da482bb4e9831962487b68f68e3928ec3137794757f54d00383d79547b"
      ;;
    aarch64)
      asset="trivy_0.73.0_Linux-ARM64.tar.gz"
      checksum="13833d97e8a1a5367471c372a173180157f593bece570e20d5d925fef552f5dd"
      ;;
    *)
      echo "::error::trivy-image-scan.sh: unsupported architecture $(uname -m)" >&2
      exit 1
      ;;
  esac
  curl -sSfL -o trivy.tar.gz \
    "https://github.com/aquasecurity/trivy/releases/download/v0.73.0/${asset}"
  echo "${checksum}  trivy.tar.gz" | sha256sum -c -
  sudo tar -xz -C /usr/local/bin -f trivy.tar.gz trivy
  rm trivy.tar.gz
fi

trivy image \
  --severity HIGH,CRITICAL \
  --ignorefile .trivyignore.yaml \
  --exit-code 1 \
  "$image"
