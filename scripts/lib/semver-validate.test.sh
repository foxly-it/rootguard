#!/usr/bin/env bash
# Regression tests for semver-validate.sh - found in review: three places
# in release-alpha.yml filter `git for-each-ref` tag lists down to "real
# SemVer tags" for further processing (the latest-published-release
# ancestry check, the changelog's previous-tag lookup, and the upgrade
# test's previous-release lookup). Two of them hand-wrote this exact
# regex correctly; the third's own hand-written copy silently dropped the
# hyphen SemVer allows inside a prerelease identifier, so a real tag like
# v1.2.3-rc-hotfix.1 was invisible to it - it could pick an older release
# than the one actually directly before this one, or skip the upgrade
# test outright. All three now share $SEMVER_PATTERN instead. This tests
# both require_semver itself and the exact
# `grep -E "^v${SEMVER_PATTERN#^}"` construction those three sites use.

set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./semver-validate.sh
source "${script_dir}/semver-validate.sh"

failures=0

# assert_valid version - require_semver must accept it.
assert_valid() {
  local version="$1"
  if ! (require_semver "$version") >/dev/null 2>&1; then
    echo "FAIL: require_semver rejected valid version ${version}" >&2
    failures=$((failures + 1))
  fi
}

# assert_invalid version - require_semver must reject it.
assert_invalid() {
  local version="$1"
  if (require_semver "$version") >/dev/null 2>&1; then
    echo "FAIL: require_semver accepted invalid version ${version}" >&2
    failures=$((failures + 1))
  fi
}

# assert_tag_matches tag - the exact tag-regex construction every
# SemVer-tag-filtering site in release-alpha.yml uses must match it.
assert_tag_matches() {
  local tag="$1"
  if ! grep -qE "^v${SEMVER_PATTERN#^}" <<<"$tag"; then
    echo "FAIL: tag filter rejected valid tag ${tag}" >&2
    failures=$((failures + 1))
  fi
}

# assert_tag_rejects tag - the same construction must reject it.
assert_tag_rejects() {
  local tag="$1"
  if grep -qE "^v${SEMVER_PATTERN#^}" <<<"$tag"; then
    echo "FAIL: tag filter accepted invalid tag ${tag}" >&2
    failures=$((failures + 1))
  fi
}

# --- require_semver ---
assert_valid "1.0.0"
assert_valid "0.1.0-alpha.4"
assert_valid "1.0.0-rc.1"
# The live regression case: a hyphen inside a prerelease identifier
# itself (not just separating core from prerelease) is valid SemVer -
# this exact version was silently invisible to the third tag filter
# before this fix.
assert_valid "1.2.3-rc-hotfix.1"
assert_valid "1.2.3-x-y-z.0"

assert_invalid "1.0"
assert_invalid "01.0.0"
assert_invalid "1.0.0+build.1"
assert_invalid "v1.0.0"
assert_invalid "1.0.0-"
assert_invalid "not-a-version"

# --- the exact tag-filter construction release-alpha.yml uses ---
assert_tag_matches "v1.0.0"
assert_tag_matches "v0.1.0-alpha.4"
assert_tag_matches "v1.2.3-rc-hotfix.1"
assert_tag_rejects "v01.0.0"
assert_tag_rejects "v1.0.0+build.1"
assert_tag_rejects "not-a-tag"

if [[ "${failures}" -gt 0 ]]; then
  echo "${failures} semver-validate.sh test failure(s)" >&2
  exit 1
fi
echo "All semver-validate.sh tests passed."
