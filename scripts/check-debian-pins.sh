#!/usr/bin/env bash
# Checks every exact-pinned Debian package version in
# rootguard-unbound/Dockerfile against what the pinned base image's own
# repository actually serves right now. Debian only ever keeps the latest
# point release of a package available - any pin WILL eventually go stale
# the moment that package gets a security update, taking the old version
# down with it (confirmed live: libexpat1/libexpat1-dev 2.8.2 -> 2.8.3
# broke the scheduled Unbound CI build with a bare "Version ... was not
# found" apt error, easy to miss in a full build log). This script is the
# fast (~10s), targeted way to notice that before a scheduled 20-minute
# build fails and someone has to go digging.
#
# Usage:
#   ./scripts/check-debian-pins.sh          # report drift, exit 1 if any
#   ./scripts/check-debian-pins.sh --fix    # also rewrite the Dockerfile
# Exit codes: 0=current/fixed, 1=fixable drift, 2=unfixable repository data;
# operational failures such as docker pull errors retain their own code.

set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repository_dir}"

dockerfile="rootguard-unbound/Dockerfile"
fix=0
[[ "${1:-}" == "--fix" ]] && fix=1

base_image="$(grep -oE 'DEBIAN_IMAGE="[^"]+"' "${dockerfile}" | head -1 | sed -E 's/DEBIAN_IMAGE="([^"]+)"/\1/')"
if [[ -z "${base_image}" ]]; then
  echo "Could not find DEBIAN_IMAGE in ${dockerfile}" >&2
  exit 1
fi

# Every "package=version" pin across both apt-get install blocks (builder
# and runtime stage), deduplicated - the same package can be pinned
# identically in both, which is intentional, not something to flag.
mapfile -t pins < <(grep -oE '^[[:space:]]*[a-z0-9.+-]+=[A-Za-z0-9:.+~-]+[[:space:]]*\\?[[:space:]]*$' "${dockerfile}" \
  | sed -E 's/^[[:space:]]*([a-z0-9.+-]+)=([A-Za-z0-9:.+~-]+)[[:space:]]*\\?[[:space:]]*$/\1=\2/' | sort -u)
if [[ "${#pins[@]}" -eq 0 ]]; then
  echo "No pinned packages found in ${dockerfile} - check the parsing pattern" >&2
  exit 1
fi

echo "Base image: ${base_image}"
echo "Checking ${#pins[@]} pinned package(s) against the live Debian repository..."

package_names=()
for p in "${pins[@]}"; do package_names+=("${p%%=*}"); done

policy_output="$(docker run --rm "${base_image}" sh -c '
  apt-get update -qq >/dev/null 2>&1
  for pkg do
    candidate="$(apt-cache policy "$pkg" 2>/dev/null | awk "/Candidate:/ {print \$2; exit}")"
    printf "%s=%s\n" "$pkg" "$candidate"
  done
' -- "${package_names[@]}")"

drift_found=0
unfixable=0
updates=()
while IFS='=' read -r pinned_name pinned_version; do
  [[ -z "${pinned_name}" ]] && continue
  candidate="$(printf '%s\n' "${policy_output}" | awk -F= -v name="${pinned_name}" '$1 == name {print $2; exit}')"
  if [[ -z "${candidate}" || "${candidate}" == "(none)" ]]; then
    echo "ERROR  ${pinned_name}: could not determine the current candidate version" >&2
    drift_found=1
    unfixable=1
    continue
  fi
  if [[ "${candidate}" != "${pinned_version}" ]]; then
    echo "DRIFT  ${pinned_name}: pinned=${pinned_version} candidate=${candidate}"
    drift_found=1
    updates+=("${pinned_name}|${pinned_version}|${candidate}")
  fi
done < <(printf '%s\n' "${pins[@]}")

if [[ "${drift_found}" -eq 0 ]]; then
  echo "All pinned packages are current."
  exit 0
fi

if [[ "${unfixable}" -ne 0 ]]; then
  exit 2
fi
if [[ "${fix}" -eq 0 ]]; then
  exit 1
fi

# Build the complete replacement in a same-directory temporary file and
# move it into place only after every update validates. This keeps --fix
# atomic when several packages drift together or one replacement fails.
temporary_dockerfile="$(mktemp "${dockerfile}.tmp.XXXXXX")"
cleanup() {
  if [[ -n "${temporary_dockerfile:-}" \
    && "${temporary_dockerfile}" == "${dockerfile}.tmp."* ]]; then
    rm -f -- "${temporary_dockerfile}" "${temporary_dockerfile}.bak"
  fi
}
trap cleanup EXIT
cp -p "${dockerfile}" "${temporary_dockerfile}"

for update in "${updates[@]}"; do
  IFS='|' read -r pinned_name pinned_version candidate <<<"${update}"
  # Basic sed regexes treat '+' literally; only dots need escaping for
  # the package/version character set accepted by the parser above.
  escaped_name="${pinned_name//./\\.}"
  escaped_version="${pinned_version//./\\.}"
  sed -i.bak "s#${escaped_name}=${escaped_version}#${pinned_name}=${candidate}#g" "${temporary_dockerfile}"
  rm -f -- "${temporary_dockerfile}.bak"
  if grep -Fq "${pinned_name}=${pinned_version}" "${temporary_dockerfile}" \
    || ! grep -Fq "${pinned_name}=${candidate}" "${temporary_dockerfile}"; then
    echo "ERROR  ${pinned_name}: automatic replacement did not modify ${dockerfile}" >&2
    exit 2
  fi
  echo "FIXED  ${pinned_name}: ${pinned_version} -> ${candidate}"
done

mv -- "${temporary_dockerfile}" "${dockerfile}"
temporary_dockerfile=""
echo "Pins updated in ${dockerfile} - rebuild and test before committing."
