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

# Same pinned version/checksum as ci-security.yml's own trivy install -
# kept in sync by hand, not by reference, since a shared step needs
# GitHub Actions' reusable-workflow/composite-action machinery to avoid
# that duplication, and this repo's CI doesn't use either yet.
if ! command -v trivy >/dev/null 2>&1; then
  curl -sSfL -o trivy.tar.gz \
    https://github.com/aquasecurity/trivy/releases/download/v0.73.0/trivy_0.73.0_Linux-64bit.tar.gz
  echo "2edd39da482bb4e9831962487b68f68e3928ec3137794757f54d00383d79547b  trivy.tar.gz" | sha256sum -c -
  sudo tar -xz -C /usr/local/bin -f trivy.tar.gz trivy
  rm trivy.tar.gz
fi

trivy image \
  --severity HIGH,CRITICAL \
  --ignorefile .trivyignore.yaml \
  --exit-code 1 \
  "$image"
