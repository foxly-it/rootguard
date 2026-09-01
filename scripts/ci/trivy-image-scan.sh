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

# ci-security.yml sources the same file, keeping the version and asset
# checksums in one place.
#
# Found in review, round 14: this used to hardcode the amd64 asset and
# checksum - silently fine on every caller except ci-unbound.yml's own
# arm64 matrix leg (ubuntu-24.04-arm), where it failed live with "cannot
# execute binary file: Exec format error". `uname -m` picks the matching
# release asset/checksum for both architectures this repo's CI actually
# runs on.
#
# Found in review, round 15: this used to skip installing entirely
# whenever *any* `trivy` was already on PATH, trusting it to be this
# exact pinned version without ever checking - a runner image that ships
# its own trivy (GitHub's hosted images add security tools like this
# over time) would then silently scan with whatever version that happened
# to be, unpinned, with no `.trivyignore.yaml` entry safe to assume still
# applies the same way against a different DB/ruleset. Now installs
# whenever the version doesn't match exactly, not just when the command
# is missing.
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/ci/trivy-version.env
source "${script_dir}/trivy-version.env"

want_version="$TRIVY_VERSION"
have_version="$(trivy --version 2>/dev/null | awk '/^Version:/ {print $2; exit}' || true)"
if [[ "$have_version" != "$want_version" ]]; then
  case "$(uname -m)" in
    x86_64)
      asset="trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz"
      checksum="$TRIVY_LINUX_AMD64_SHA256"
      ;;
    aarch64)
      asset="trivy_${TRIVY_VERSION}_Linux-ARM64.tar.gz"
      checksum="$TRIVY_LINUX_ARM64_SHA256"
      ;;
    *)
      echo "::error::trivy-image-scan.sh: unsupported architecture $(uname -m)" >&2
      exit 1
      ;;
  esac
  curl -sSfL -o trivy.tar.gz \
    "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/${asset}"
  echo "${checksum}  trivy.tar.gz" | sha256sum -c -
  sudo tar -xz -C /usr/local/bin -f trivy.tar.gz trivy
  rm trivy.tar.gz
fi

platform_args=()
if [[ -n "$platform" ]]; then
  platform_args=(--platform "$platform")
fi

trivy image \
  --severity HIGH,CRITICAL \
  --ignorefile .trivyignore.yaml \
  --exit-code 1 \
  "${platform_args[@]}" \
  "$image"
