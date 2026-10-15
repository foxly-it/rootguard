#!/usr/bin/env bash
# Alpine counterpart of check-debian-pins.sh (#712): checks every exact
# `apk add pkg=ver-rN` pin in the given Dockerfiles against the newest build
# the final-stage base image's own repository serves. Alpine's mirrors drop
# superseded builds, so a stale pin breaks the build outright ("unable to
# select packages") - found live four times before this existed.
#
# Usage: ./scripts/check-alpine-pins.sh [--fix] [Dockerfile...]
#   (defaults to Core, Updater and Blockpage)
# Exit codes: 0=current/fixed, 1=fixable drift, 2=unfixable repository data;
# operational failures such as docker pull errors retain their own code.
set -Eeuo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
fix=0
[[ "${1:-}" == "--fix" ]] && { fix=1; shift; }
(( $# )) || set -- rootguard-core/Dockerfile rootguard-updater/Dockerfile rootguard-blockpage/Dockerfile

status=0
for dockerfile in "$@"; do
  # Final stage's base: a literal FROM, or ${VAR} resolved from its ARG default.
  base="$(awk '/^FROM / {b=$2} END {print b}' "${dockerfile}")"
  if [[ "${base}" =~ ^\$\{([A-Z_]+)\}$ ]]; then
    base="$(sed -nE "s/^ARG ${BASH_REMATCH[1]}=\"?([^\"]+)\"?$/\1/p" "${dockerfile}")"
  fi
  mapfile -t pins < <(grep -vE '^[[:space:]]*#' "${dockerfile}" \
    | grep -oE '[a-z0-9][a-z0-9._+-]*=[0-9][A-Za-z0-9._+~]*-r[0-9]+' | sort -u)
  if [[ -z "${base}" || "${#pins[@]}" -eq 0 ]]; then
    echo "ERROR  ${dockerfile}: no base image or apk pins found - check the parsing" >&2
    exit 2
  fi
  echo "${dockerfile}: ${#pins[@]} pin(s) against ${base}"

  newest="$(docker run --rm --entrypoint sh "${base}" -c '
    apk update -q >/dev/null 2>&1
    for pkg do
      printf "%s=%s\n" "$pkg" "$(apk policy "$pkg" 2>/dev/null \
        | awk "/^  [^ ]/ {v=\$1; sub(/:\$/, \"\", v)} /:\\/\\// {c=v} END {print c}")"
    done
  ' -- "${pins[@]%%=*}")"

  expressions=()
  for pin in "${pins[@]}"; do
    name="${pin%%=*}" pinned="${pin#*=}"
    candidate="$(awk -F= -v n="${name}" '$1 == n {print $2; exit}' <<<"${newest}")"
    if [[ -z "${candidate}" ]]; then
      echo "ERROR  ${name}: no repository version found" >&2; status=2; continue
    fi
    [[ "${candidate}" == "${pinned}" ]] && continue
    echo "DRIFT  ${name}: pinned=${pinned} newest=${candidate}"
    (( status )) || status=1
    expressions+=(-e "s#${name//./\\.}=${pinned//./\\.}#${name}=${candidate}#g")
  done

  # Rewrite via a same-directory temp file so a failed run never leaves a
  # half-edited Dockerfile behind.
  if (( fix && ${#expressions[@]} )) && [[ "${status}" -ne 2 ]]; then
    tmp="$(mktemp "${dockerfile}.tmp.XXXXXX")"
    sed "${expressions[@]}" "${dockerfile}" >"${tmp}" && mv -- "${tmp}" "${dockerfile}"
    echo "FIXED  ${dockerfile}"
  fi
done

if (( status == 0 )); then echo "All apk pins are current."
elif (( fix && status == 1 )); then status=0
fi
exit "${status}"
