#!/usr/bin/env bash
# Regression tests for semver-compare.sh - found in review, round 6: the
# comparator's own header comment referenced this file before it existed,
# so the release-version guard shipped with no automated coverage at all.
# Covers the canonical SemVer.org precedence chain (spec section 11),
# build-metadata stripping, the live-reproduced "0.9.9 vs 1.0.0-rc.1"
# case the guard exists for, and the 64-bit bash-arithmetic overflow
# case also found live (9223372036854775807 vs ...808).

set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./semver-compare.sh
#
# Found in review, round 7: running the linter from the repo root (as
# opposed to from scripts/lib/ itself) can't resolve the source=
# directive above through $script_dir and reports SC1091 - the directive
# is correct, the linter just needs its "-P SCRIPTDIR" flag (resolves
# source= paths relative to *this* file's own directory, regardless of
# the invoking shell's cwd) added to its own command line to follow it.
source "${script_dir}/semver-compare.sh"

failures=0

# assert_cmp a b expected — expected is -1, 0, or 1 for semver_compare(a, b).
assert_cmp() {
  local a="$1" b="$2" expected="$3" actual
  actual="$(semver_compare "$a" "$b")"
  if [[ "$actual" != "$expected" ]]; then
    echo "FAIL: semver_compare(${a}, ${b}) = ${actual}, expected ${expected}" >&2
    failures=$((failures + 1))
  fi
}

# assert_require_ok requested latest — require_new_version must accept.
assert_require_ok() {
  local requested="$1" latest="$2"
  if ! (require_new_version "$requested" "$latest") >/dev/null 2>&1; then
    echo "FAIL: require_new_version(${requested}, ${latest}) rejected, expected accept" >&2
    failures=$((failures + 1))
  fi
}

# assert_require_rejects requested latest — require_new_version must reject.
assert_require_rejects() {
  local requested="$1" latest="$2"
  if (require_new_version "$requested" "$latest") >/dev/null 2>&1; then
    echo "FAIL: require_new_version(${requested}, ${latest}) accepted, expected reject" >&2
    failures=$((failures + 1))
  fi
}

# --- SemVer.org's own canonical precedence chain (spec section 11), each
# adjacent pair strictly increasing ---
chain=(
  1.0.0-alpha
  1.0.0-alpha.1
  1.0.0-alpha.beta
  1.0.0-beta
  1.0.0-beta.2
  1.0.0-beta.11
  1.0.0-rc.1
  1.0.0
)
for ((i = 0; i < ${#chain[@]} - 1; i++)); do
  assert_cmp "${chain[$i]}" "${chain[$((i + 1))]}" -1
  assert_cmp "${chain[$((i + 1))]}" "${chain[$i]}" 1
done
assert_cmp "1.0.0" "1.0.0" 0

# --- major.minor.patch ordering, including numeric (not lexicographic)
# comparison of multi-digit components ---
assert_cmp "1.2.3" "1.2.4" -1
assert_cmp "1.10.0" "1.9.0" 1
assert_cmp "2.0.0" "1.99.99" 1

# --- build metadata is stripped and ignored ---
assert_cmp "1.0.0+build.1" "1.0.0+build.2" 0
assert_cmp "1.0.0-rc.1+exp.sha.5114f85" "1.0.0-rc.1" 0

# --- the live-reproduced case this guard exists for ---
assert_cmp "0.9.9" "1.0.0-rc.1" -1
assert_require_rejects "0.9.9" "1.0.0-rc.1"
assert_require_ok "1.0.0-rc.2" "1.0.0-rc.1"
assert_require_rejects "1.0.0-rc.1" "1.0.0-rc.1"

# --- the 64-bit bash-arithmetic overflow case found live: comparing via
# $(( )) wrapped 9223372036854775808 negative, past bash's signed 64-bit
# range, ranking it *below* 9223372036854775807 ---
assert_cmp "9223372036854775807.0.0" "9223372036854775808.0.0" -1
assert_cmp "9223372036854775808.0.0" "9223372036854775807.0.0" 1
assert_cmp "1.0.0-9223372036854775807" "1.0.0-9223372036854775808" -1

# --- numeric prerelease identifiers always outrank... no, always have
# *lower* precedence than alphanumeric ones at the same position (rule
# 11.4.3), and a longer identifier list outranks an equal-prefix shorter
# one (rule 11.4.4) ---
assert_cmp "1.0.0-1" "1.0.0-alpha" -1
assert_cmp "1.0.0-alpha" "1.0.0-alpha.1" -1

if [[ "${failures}" -gt 0 ]]; then
  echo "${failures} semver-compare.sh test failure(s)" >&2
  exit 1
fi
echo "All semver-compare.sh tests passed."
