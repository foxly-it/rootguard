#!/usr/bin/env bash
# Installs the exact trivy version pinned in trivy-version.env, unless a
# matching version is already on PATH.
#
# Found in review: this used to be duplicated - trivy-image-scan.sh (used
# by every per-image scan) had its own, arch-aware, skip-if-already-
# pinned copy, while ci-security.yml's own `trivy fs .` job carried a
# second, simpler copy that hardcoded the amd64 asset/checksum and always
# reinstalled unconditionally. Both now call this one script instead.
#
# Usage: ./scripts/ci/install-trivy.sh
# Run from anywhere - resolves trivy-version.env relative to this
# script's own location, not the caller's working directory.
set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/ci/trivy-version.env
source "${script_dir}/trivy-version.env"

want_version="$TRIVY_VERSION"
have_version="$(trivy --version 2>/dev/null | awk '/^Version:/ {print $2; exit}' || true)"
if [[ "$have_version" == "$want_version" ]]; then
  exit 0
fi

# Found in review, round 14: this used to hardcode the amd64 asset and
# checksum - silently fine everywhere except ci-unbound.yml's own arm64
# matrix leg (ubuntu-24.04-arm), where it failed live with "cannot execute
# binary file: Exec format error". `uname -m` picks the matching release
# asset/checksum for both architectures this repo's CI actually runs on.
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
    echo "::error::install-trivy.sh: unsupported architecture $(uname -m)" >&2
    exit 1
    ;;
esac

curl -sSfL -o trivy.tar.gz \
  "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/${asset}"
echo "${checksum}  trivy.tar.gz" | sha256sum -c -
sudo tar -xz -C /usr/local/bin -f trivy.tar.gz trivy
rm trivy.tar.gz
