#!/usr/bin/env bash
# Shared SemVer 2.0 precedence comparison for release-alpha.yml and
# release-version-bump.yml - found in review: the ancestry guard added
# alongside this (git merge-base --is-ancestor against the latest
# published release's own commit) only ever verified that the source
# commit descends from the latest release, never that the *version
# number itself* has higher SemVer precedence. Confirmed live: a manual
# override requesting "0.9.9" against a HEAD descending from the real
# published "1.0.0-rc.1" passed the ancestry check cleanly - nothing
# stopped an unused-but-semantically-older version from being published
# and then resetting README/site/compose pins back to it.
#
# Deliberately not `sort -V` (GNU coreutils' version sort) - confirmed
# live that it gets the one comparison this project will need very soon
# wrong: `printf '%s\n' 1.0.0-rc.1 1.0.0 | sort -V` ranks 1.0.0 *below*
# 1.0.0-rc.1, the opposite of real SemVer precedence (a version with no
# prerelease suffix always outranks any prerelease of the same core
# version - rule 11.3 of the spec). semver_compare below implements the
# real precedence rules (11.2-11.4) directly instead.
#
# This is the bash sibling of rootguard-updater's own isOlderReleaseVersion
# (image.go), which already does this correctly for Go callers via
# golang.org/x/mod/semver - not reused directly since neither
# release-alpha.yml nor release-version-bump.yml's jobs have a Go
# toolchain set up, and standing one up purely for this comparison was
# judged heavier than a careful, well-tested bash implementation of a
# well-defined, stable algorithm. Both copies are exercised against the
# same known-tricky cases (see this file's own semver-compare.test.sh,
# run by ci.yml, and rootguard-core's main_test.go's
# TestIsOlderReleaseVersion) - keep them in sync by hand if either ever
# changes.
#
# Found in review, round 6: this comment used to point at a
# "semver-compare.test-cases" that was never actually created - the new
# guard shipped with no automated regression coverage at all beyond
# manual review. semver-compare.test.sh now holds it.
#
# Usage: source this file, then call require_new_version "$requested"
# "$latest_published" - exits non-zero (message on stderr) unless
# $requested has strictly greater SemVer 2.0 precedence than
# $latest_published. Assumes both are already grammar-valid (see
# semver-validate.sh's require_semver) - garbage in, undefined out.

_semver_is_numeric_identifier() {
  [[ "$1" =~ ^(0|[1-9][0-9]*)$ ]]
}

# _semver_compare_numeric_string a b -> echoes -1, 0, or 1 for a<b, a==b,
# a>b, treating both as arbitrary-precision non-negative decimal integers
# (no leading zeros - semver-validate.sh's own SEMVER_PATTERN enforces
# that grammar for major/minor/patch and for numeric prerelease
# identifiers alike). SemVer doesn't cap numeric identifiers at 64 bits;
# bash integer arithmetic does - found in review: comparing via $(( ))
# silently overflowed and wrapped negative past 9223372036854775807, so
# "9223372036854775808.0.0" compared as *older* than
# "9223372036854775807.0.0", the opposite of the true order. Without
# leading zeros, a longer digit string is always numerically larger, and
# two equal-length ones compare correctly byte-for-byte (ASCII digit
# order matches numeric order) - no arithmetic, no width limit.
_semver_compare_numeric_string() {
  local a="$1" b="$2"
  if [[ ${#a} -lt ${#b} ]]; then
    echo -1
    return
  fi
  if [[ ${#a} -gt ${#b} ]]; then
    echo 1
    return
  fi
  if [[ "$a" < "$b" ]]; then
    echo -1
    return
  fi
  if [[ "$a" > "$b" ]]; then
    echo 1
    return
  fi
  echo 0
}

# _semver_compare_prerelease a b -> echoes -1, 0, or 1 for a<b, a==b,
# a>b per SemVer 2.0 rule 11.4: compare dot-separated identifiers left
# to right; numeric identifiers compare numerically and always have
# lower precedence than alphanumeric identifiers at the same position;
# a longer identifier list outranks an otherwise-equal shorter one.
_semver_compare_prerelease() {
  local a="$1" b="$2"
  local -a a_parts b_parts
  IFS='.' read -r -a a_parts <<<"$a"
  IFS='.' read -r -a b_parts <<<"$b"
  local i=0
  while [[ $i -lt ${#a_parts[@]} && $i -lt ${#b_parts[@]} ]]; do
    local ai="${a_parts[$i]}" bi="${b_parts[$i]}"
    if [[ "$ai" != "$bi" ]]; then
      local a_num=0 b_num=0
      _semver_is_numeric_identifier "$ai" && a_num=1
      _semver_is_numeric_identifier "$bi" && b_num=1
      if [[ $a_num -eq 1 && $b_num -eq 1 ]]; then
        local numcmp
        numcmp="$(_semver_compare_numeric_string "$ai" "$bi")"
        if [[ "$numcmp" -ne 0 ]]; then
          echo "$numcmp"
          return
        fi
      elif [[ $a_num -eq 1 && $b_num -eq 0 ]]; then
        echo -1
        return
      elif [[ $a_num -eq 0 && $b_num -eq 1 ]]; then
        echo 1
        return
      else
        if [[ "$ai" < "$bi" ]]; then
          echo -1
          return
        fi
        if [[ "$ai" > "$bi" ]]; then
          echo 1
          return
        fi
      fi
    fi
    i=$((i + 1))
  done
  if [[ ${#a_parts[@]} -lt ${#b_parts[@]} ]]; then
    echo -1
    return
  fi
  if [[ ${#a_parts[@]} -gt ${#b_parts[@]} ]]; then
    echo 1
    return
  fi
  echo 0
}

# semver_compare a b -> echoes -1, 0, or 1 for a<b, a==b, a>b in full
# SemVer 2.0 precedence. Build metadata (a "+..." suffix), if present,
# is stripped and ignored per the spec - moot in practice, since
# require_semver already rejects it as ungrammatical for this project's
# own tags (see semver-validate.sh's own comment on why).
semver_compare() {
  local a="${1%%+*}" b="${2%%+*}"
  local a_core="${a%%-*}" b_core="${b%%-*}"
  local a_pre="" b_pre=""
  [[ "$a" == *-* ]] && a_pre="${a#*-}"
  [[ "$b" == *-* ]] && b_pre="${b#*-}"
  local a_major a_minor a_patch b_major b_minor b_patch
  IFS='.' read -r a_major a_minor a_patch <<<"$a_core"
  IFS='.' read -r b_major b_minor b_patch <<<"$b_core"
  local x y cmp
  for pair in "$a_major:$b_major" "$a_minor:$b_minor" "$a_patch:$b_patch"; do
    x="${pair%%:*}"
    y="${pair#*:}"
    cmp="$(_semver_compare_numeric_string "$x" "$y")"
    if [[ "$cmp" -ne 0 ]]; then
      echo "$cmp"
      return
    fi
  done
  if [[ -z "$a_pre" && -z "$b_pre" ]]; then
    echo 0
    return
  fi
  # Rule 11.3: a version with no prerelease always outranks one with a
  # prerelease of the same major.minor.patch.
  if [[ -z "$a_pre" && -n "$b_pre" ]]; then
    echo 1
    return
  fi
  if [[ -n "$a_pre" && -z "$b_pre" ]]; then
    echo -1
    return
  fi
  _semver_compare_prerelease "$a_pre" "$b_pre"
}

require_new_version() {
  local requested="$1" latest="$2"
  local cmp
  cmp="$(semver_compare "$requested" "$latest")"
  if [[ "$cmp" -le 0 ]]; then
    echo "Refusing to publish ${requested}: not strictly newer than the currently published ${latest} (SemVer 2.0 precedence) - cut a genuinely higher version instead." >&2
    exit 1
  fi
}
