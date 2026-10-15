#!/usr/bin/env bash
set -Eeuo pipefail

script="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/check-trivyignore-expiry.sh"
fixture="$(mktemp)"
trap 'rm -f -- "${fixture}"' EXIT

cat >"${fixture}" <<'EOF'
vulnerabilities:
  - id: CVE-PAST
    expired_at: 2026-10-01
    statement: >-
      already expired
  # comment between entries
  - id: CVE-SOON
    purls: ["pkg:golang/stdlib@v1.26.4"]
    expired_at: 2026-10-20
  - id: CVE-LATER
    expired_at: 2026-12-31
EOF

fail() { echo "FAIL: $*" >&2; exit 1; }

set +e
output="$(TODAY=2026-10-15 "${script}" 14 "${fixture}")"; rc=$?
set -e
[[ "${rc}" -eq 1 ]] || fail "due entries should exit 1, got ${rc}"
grep -q '^EXPIRED   2026-10-01  CVE-PAST$' <<<"${output}" || fail "past entry not reported as EXPIRED"
grep -q '^EXPIRING  2026-10-20  CVE-SOON (in 5 days)$' <<<"${output}" || fail "entry inside window not reported"
grep -q 'CVE-LATER' <<<"${output}" && fail "entry outside window reported"

output="$(TODAY=2026-09-01 "${script}" 14 "${fixture}")" || fail "nothing due should exit 0"
grep -q 'No .trivyignore.yaml entry expires within 14 days.' <<<"${output}" || fail "missing all-clear message"

set +e
"${script}" abc "${fixture}" 2>/dev/null; rc=$?
set -e
[[ "${rc}" -eq 2 ]] || fail "invalid days should exit 2, got ${rc}"

echo "All check-trivyignore-expiry.sh tests passed."
