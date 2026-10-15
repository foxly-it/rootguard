#!/usr/bin/env bash
# Lists .trivyignore.yaml entries that have expired or expire within the
# next N days (default 14). Trivy silently stops honoring an entry once its
# expired_at passes, so the finding fails every scan again on that date -
# found 2026-10-15 (#710) with 61 of 62 entries sharing one date.
#
# Usage: ./scripts/ci/check-trivyignore-expiry.sh [days] [ignorefile]
# Exit codes: 0 = nothing due, 1 = entries due, 2 = usage/parse error.
# TODAY=YYYY-MM-DD overrides the current date (tests).
set -Eeuo pipefail

days="${1:-14}"
ignorefile="${2:-.trivyignore.yaml}"
[[ "${days}" =~ ^[0-9]+$ && -f "${ignorefile}" ]] || { echo "usage: $0 [days] [ignorefile]" >&2; exit 2; }

to_epoch() { date -u -d "$1" +%s 2>/dev/null || date -u -j -f "%Y-%m-%d %H:%M:%S" "$1 00:00:00" +%s; }
today="$(to_epoch "${TODAY:-$(date -u +%Y-%m-%d)}")"
limit=$((today + days * 86400))

entries="$(awk '/^  - id:/ {id=$3} /^    expired_at:/ {print id, $2}' "${ignorefile}")"
[[ -n "${entries}" ]] || { echo "No expired_at entries found in ${ignorefile}" >&2; exit 2; }

due=0
while read -r id date; do
  expiry="$(to_epoch "${date}")"
  if (( expiry <= today )); then
    echo "EXPIRED   ${date}  ${id}"; due=1
  elif (( expiry <= limit )); then
    echo "EXPIRING  ${date}  ${id} (in $(( (expiry - today) / 86400 )) days)"; due=1
  fi
done <<<"${entries}"

(( due )) || echo "No .trivyignore.yaml entry expires within ${days} days."
exit "${due}"
